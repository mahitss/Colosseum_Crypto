package marketstudio

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

// BroadcastResult reports the outcome of submitting a signed transaction.
//
// The three fields are tracked separately on purpose. A signature alone means
// the transaction was accepted for processing, nothing more. Only Confirmed
// means the chain has finalised it, and even then the market does not exist
// until Panta registration succeeds.
type BroadcastResult struct {
	Signature     string
	SubmitStatus  string // submitted | failed
	ConfirmStatus string // confirmed | finalized | failed | unknown
	Confirmed     bool
	// Confirmations is the depth reported by the RPC.
	Confirmations int
}

// Broadcaster submits signed Solana transactions through a configured RPC.
//
// It holds no key. The signed bytes arrive from the user's wallet, are relayed
// straight to the RPC, and are never written to disk or to the database.
type Broadcaster struct {
	rpcURL  string
	client  *http.Client
	timeout time.Duration
	retries int
	sleep   func(context.Context, time.Duration) error
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
	}, nil
}

// SendRawTransaction relays a signed, base64 transaction to the RPC.
//
// Retries reuse the same signed bytes, so a retry can never create a second
// transaction: Solana deduplicates by signature.
func (b *Broadcaster) SendRawTransaction(ctx context.Context, signedTxB64 string) (BroadcastResult, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signedTxB64))
	if err != nil || len(raw) == 0 {
		return BroadcastResult{}, errors.New("invalid signed transaction encoding")
	}

	var lastErr error
	for attempt := 0; attempt <= b.retries; attempt++ {
		signature, err := b.sendOnce(ctx, signedTxB64)
		if err == nil {
			return BroadcastResult{Signature: signature, SubmitStatus: "submitted"}, nil
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
	var result string
	err := b.rpc(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []any{
			strings.TrimSpace(signedTxB64),
			map[string]any{"encoding": "base64", "skipPreflight": false},
		},
	}, &result)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result) == "" {
		return "", errors.New("rpc returned no signature")
	}
	return result, nil
}

// GetSignatureStatuses polls confirmation depth for a signature.
//
// A dropped transaction is reported as ConfirmStatus "failed" so the caller can
// distinguish "never landed" from "still pending" — the two need different
// messages to the user.
func (b *Broadcaster) GetSignatureStatuses(ctx context.Context, signature string) (BroadcastResult, error) {
	if strings.TrimSpace(signature) == "" {
		return BroadcastResult{}, ErrSignatureRequired
	}

	// envelope.Result already holds the *contents* of the JSON-RPC "result" key,
	// so this struct describes that payload directly — not another envelope.
	var response struct {
		Value []struct {
			Err           any    `json:"err"`
			Confirmations *int   `json:"confirmations"`
			Status        string `json:"status"`
		} `json:"value"`
	}
	err := b.rpc(ctx, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getSignatureStatuses",
		"params": []any{
			[]string{signature},
			map[string]any{"searchTransactionHistory": true},
		},
	}, &response)
	if err != nil {
		return BroadcastResult{}, err
	}

	if len(response.Value) == 0 {
		return BroadcastResult{
			Signature:     signature,
			SubmitStatus:  "submitted",
			ConfirmStatus: "unknown",
		}, nil
	}

	value := response.Value[0]
	if value.Err != nil {
		return BroadcastResult{
			Signature:     signature,
			SubmitStatus:  "submitted",
			ConfirmStatus: "failed",
		}, nil
	}
	if value.Status == "" {
		// The signature is known but not yet in a landed slot.
		return BroadcastResult{
			Signature:     signature,
			SubmitStatus:  "submitted",
			ConfirmStatus: "unknown",
		}, nil
	}

	confirmations := 0
	if value.Confirmations != nil {
		confirmations = *value.Confirmations
	}
	// finalized is the strongest guarantee; confirmed is acceptable. Anything
	// else is not a confirmation we will act on.
	confirmed := value.Status == "confirmed" || value.Status == "finalized"
	confirmStatus := value.Status
	if !confirmed {
		confirmStatus = "unknown"
	}
	return BroadcastResult{
		Signature:     signature,
		SubmitStatus:  "submitted",
		ConfirmStatus: confirmStatus,
		Confirmed:     confirmed,
		Confirmations: confirmations,
	}, nil
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (b *Broadcaster) rpc(ctx context.Context, request any, destination any) error {
	raw, err := json.Marshal(request)
	if err != nil {
		return errors.New("rpc request encode failed")
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, b.rpcURL, strings.NewReader(string(raw)))
	if err != nil {
		return errors.New("rpc request build failed")
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")

	response, err := b.client.Do(httpRequest)
	if err != nil {
		return errors.New("solana rpc unavailable")
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return errors.New("solana rpc returned an unreadable response")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("solana rpc request failed")
	}

	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return errors.New("solana rpc returned malformed data")
	}
	if envelope.Error != nil {
		// A "blockhash not found"/"expired" style error is a real, reportable
		// failure rather than a transient one.
		return fmt.Errorf("solana rpc error: %s", envelope.Error.Message)
	}
	if destination == nil || len(envelope.Result) == 0 {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, destination); err != nil {
		return errors.New("solana rpc returned a malformed result")
	}
	return nil
}
