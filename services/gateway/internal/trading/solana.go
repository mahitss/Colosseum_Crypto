package trading

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ConfirmationLevel controls how confirmed a transaction must be before we consider it final.
type ConfirmationLevel string

const (
	ConfirmationNone      ConfirmationLevel = "none"
	ConfirmationProcessed ConfirmationLevel = "processed"
	ConfirmationConfirmed ConfirmationLevel = "confirmed"
	ConfirmationFinalized ConfirmationLevel = "finalized"
)

// BroadcastResult reports the outcome of broadcasting a signed transaction.
type BroadcastResult struct {
	Signature     string
	SubmitStatus  string // "submitted" | "failed" | "expired"
	ConfirmStatus string // "confirmed" | "finalized" | "failed" | "unknown"
	Confirmed     bool
}

// Broadcaster broadcasts signed Solana transactions through a configured RPC.
// The RPC URL always comes from server configuration — never from the browser.
type Broadcaster struct {
	rpcURL  string
	client  *http.Client
	timeout time.Duration
	retries int
	sleep   func(context.Context, time.Duration) error
	now     func() time.Time
}

type BroadcasterOptions struct {
	HTTPClient   *http.Client
	Timeout      time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
	Sleep        func(context.Context, time.Duration) error
}

func NewBroadcaster(rpcURL string, options BroadcasterOptions) (*Broadcaster, error) {
	if strings.TrimSpace(rpcURL) == "" {
		return nil, errors.New("SOLANA_RPC_URL is required")
	}
	if options.Timeout <= 0 {
		options.Timeout = 20 * time.Second
	}
	if options.MaxRetries < 0 {
		options.MaxRetries = 0
	}
	if options.MaxRetries == 0 {
		options.MaxRetries = 2
	}
	if options.RetryBackoff <= 0 {
		options.RetryBackoff = 500 * time.Millisecond
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: options.Timeout}
	}
	sleep := options.Sleep
	if sleep == nil {
		sleep = func(ctx context.Context, d time.Duration) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d):
				return nil
			}
		}
	}
	return &Broadcaster{
		rpcURL:  rpcURL,
		client:  client,
		timeout: options.Timeout,
		retries: options.MaxRetries,
		sleep:   sleep,
		now:     time.Now,
	}, nil
}

// SendRawTransaction submits a signed, base64-encoded transaction to the RPC.
// It uses the transaction signature for idempotency: retries reuse the same
// signature so we never create duplicate submissions.
func (b *Broadcaster) SendRawTransaction(ctx context.Context, signedTxB64 string) (BroadcastResult, error) {
	// The signed transaction must be valid base64.
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signedTxB64))
	if err != nil || len(raw) == 0 {
		return BroadcastResult{}, errors.New("invalid signed transaction encoding")
	}

	var lastErr error
	for attempt := 0; attempt <= b.retries; attempt++ {
		sig, err := b.sendOnce(ctx, signedTxB64)
		if err == nil {
			// Idempotent: the same signature is returned whether the tx is new or a duplicate replay.
			return BroadcastResult{
				Signature:    sig,
				SubmitStatus: "submitted",
			}, nil
		}
		lastErr = err
		if attempt < b.retries {
			if sleepErr := b.sleep(ctx, b.timeout/4); sleepErr != nil {
				return BroadcastResult{}, sleepErr
			}
		}
	}
	return BroadcastResult{}, fmt.Errorf("broadcast failed after %d attempts: %w", b.retries+1, lastErr)
}

func (b *Broadcaster) sendOnce(ctx context.Context, signedTxB64 string) (string, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendRawTransaction",
		"params":  []any{signedTxB64, map[string]any{"encoding": "base64", "skipPreflight": false}},
	}
	body, _ := json.Marshal(payload)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, b.rpcURL, strings.NewReader(string(body)))
	if err != nil {
		return "", errors.New("could not create broadcast request")
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := b.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("RPC unavailable: %w", err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return "", errors.New("could not read RPC response")
	}

	var rpc struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &rpc); err != nil {
		return "", errors.New("malformed RPC response")
	}
	if rpc.Error != nil {
		return "", fmt.Errorf("RPC error: %s", rpc.Error.Message)
	}
	if rpc.Result == "" {
		return "", errors.New("RPC returned no signature")
	}
	return rpc.Result, nil
}

// GetSignatureStatus polls confirmations for a signature.
func (b *Broadcaster) GetSignatureStatus(ctx context.Context, signature string, level ConfirmationLevel) (BroadcastResult, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getSignatureStatuses",
		"params":  []any{[]string{signature}, map[string]any{"searchTransactionHistory": true}},
	}
	body, _ := json.Marshal(payload)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, b.rpcURL, strings.NewReader(string(body)))
	if err != nil {
		return BroadcastResult{}, errors.New("could not create status request")
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := b.client.Do(request)
	if err != nil {
		return BroadcastResult{}, fmt.Errorf("RPC unavailable: %w", err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return BroadcastResult{}, errors.New("could not read RPC response")
	}

	var rpc struct {
		Result struct {
			Value []*struct {
				ConfirmationStatus *string `json:"confirmationStatus"`
				Err                any     `json:"err"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &rpc); err != nil {
		return BroadcastResult{}, errors.New("malformed RPC status response")
	}

	if len(rpc.Result.Value) == 0 || rpc.Result.Value[0] == nil {
		return BroadcastResult{Signature: signature, ConfirmStatus: "unknown", Confirmed: false}, nil
	}

	status := rpc.Result.Value[0]
	if status.Err != nil {
		return BroadcastResult{Signature: signature, ConfirmStatus: "failed", Confirmed: false}, nil
	}

	// The RPC reports confirmation status; if nil but no error, treat as processed.
	current := "processed"
	if status.ConfirmationStatus != nil {
		current = *status.ConfirmationStatus
	}

	confirmed := false
	switch level {
	case ConfirmationNone:
		confirmed = true
		current = "submitted"
	case ConfirmationProcessed:
		confirmed = true
	case ConfirmationConfirmed:
		confirmed = current == "confirmed" || current == "finalized"
	case ConfirmationFinalized:
		confirmed = current == "finalized"
	}

	return BroadcastResult{
		Signature:     signature,
		SubmitStatus:  "submitted",
		ConfirmStatus: current,
		Confirmed:     confirmed,
	}, nil
}

// WaitForConfirmation polls until the desired level is reached or the timeout elapses.
func (b *Broadcaster) WaitForConfirmation(ctx context.Context, signature string, level ConfirmationLevel, timeout time.Duration) (BroadcastResult, error) {
	deadline := time.Now().Add(timeout)
	for {
		result, err := b.GetSignatureStatus(ctx, signature, level)
		if err != nil {
			return result, err
		}
		if result.ConfirmStatus == "failed" {
			return result, nil
		}
		if result.Confirmed {
			return result, nil
		}
		if time.Now().After(deadline) {
			return result, fmt.Errorf("confirmation timed out: status %q", result.ConfirmStatus)
		}
		if sleepErr := b.sleep(ctx, 1*time.Second); sleepErr != nil {
			return result, sleepErr
		}
	}
}
