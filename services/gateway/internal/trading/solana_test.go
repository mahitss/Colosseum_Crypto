package trading

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Test fixtures — deterministic, never mainnet.
const (
	fixtureSignature = "5dGt2jGv4d5ykJ8kY8vqQ1oKXzXmCjMhqWwLz7tK1aK2m3n4p5q6r7s8t9u0v1w2x3y4z5A6B7C8D9E"
	fixtureBase64    = "RElWSU5JUkVMQU5ET01FMEQ=" // deterministic placeholder
)

func fixtureServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
}

func newFixturesBroadcaster(t *testing.T, url string) *Broadcaster {
	t.Helper()
	b, err := NewBroadcaster(url, BroadcasterOptions{
		Timeout:      2 * time.Second,
		MaxRetries:   1,
		RetryBackoff: 1 * time.Millisecond,
		Sleep: func(ctx context.Context, d time.Duration) error {
			return nil // no real waiting in tests
		},
	})
	if err != nil {
		t.Fatalf("failed to create broadcaster: %v", err)
	}
	return b
}

func TestSendRawTransactionSuccess(t *testing.T) {
	// The RPC returns a signature for the deterministic tx we send.
	server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "sendRawTransaction") {
			t.Errorf("expected sendRawTransaction method, got %s", string(body))
		}
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","result":"`+fixtureSignature+`","id":1}`)
	})
	defer server.Close()

	b := newFixturesBroadcaster(t, server.URL)
	result, err := b.SendRawTransaction(context.Background(), fixtureBase64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Signature != fixtureSignature {
		t.Errorf("expected signature %s, got %s", fixtureSignature, result.Signature)
	}
	if result.SubmitStatus != "submitted" {
		t.Errorf("expected submitted, got %s", result.SubmitStatus)
	}
}

func TestSendRawTransactionInvalidEncoding(t *testing.T) {
	server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	b := newFixturesBroadcaster(t, server.URL)
	_, err := b.SendRawTransaction(context.Background(), "not-valid-base64!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestSendRawTransactionRPCReturnsError(t *testing.T) {
	server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","error":{"message":"Blockhash not found"},"id":1}`)
	})
	defer server.Close()

	b := newFixturesBroadcaster(t, server.URL)
	_, err := b.SendRawTransaction(context.Background(), fixtureBase64)
	if err == nil {
		t.Fatal("expected error when RPC returns error")
	}
}

func TestSendRawTransactionRetriesAndSucceeds(t *testing.T) {
	attempts := 0
	server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","error":{"message":"Node unavailable"},"id":1}`)
			return
		}
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","result":"`+fixtureSignature+`","id":1}`)
	})
	defer server.Close()

	b := newFixturesBroadcaster(t, server.URL)
	result, err := b.SendRawTransaction(context.Background(), fixtureBase64)
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if result.Signature != fixtureSignature {
		t.Errorf("expected signature, got %s", result.Signature)
	}
}

func TestWaitForConfirmationConfirmed(t *testing.T) {
	server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","result":{"value":[{
			"confirmationStatus":"confirmed","err":null
		}]},"id":1}`)
	})
	defer server.Close()

	b := newFixturesBroadcaster(t, server.URL)
	result, err := b.GetSignatureStatus(context.Background(), fixtureSignature, ConfirmationConfirmed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Confirmed {
		t.Error("expected confirmed=true")
	}
	if result.ConfirmStatus != "confirmed" {
		t.Errorf("expected confirmed, got %s", result.ConfirmStatus)
	}
}

func TestWaitForConfirmationFailed(t *testing.T) {
	server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","result":{"value":[{
			"confirmationStatus":"confirmed","err":{"InstructionError":0}
		}]},"id":1}`)
	})
	defer server.Close()

	b := newFixturesBroadcaster(t, server.URL)
	result, err := b.GetSignatureStatus(context.Background(), fixtureSignature, ConfirmationConfirmed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ConfirmStatus != "failed" {
		t.Errorf("expected failed status, got %s", result.ConfirmStatus)
	}
	if result.Confirmed {
		t.Error("failed tx must not be confirmed")
	}
}

func TestWaitForConfirmationTimeout(t *testing.T) {
	server := fixtureServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","result":{"value":[{
			"confirmationStatus":"processed","err":null}]},"id":1}`)
	})
	defer server.Close()

	b := newFixturesBroadcaster(t, server.URL)
	_, err := b.WaitForConfirmation(context.Background(), fixtureSignature, ConfirmationFinalized, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestBroadcasterRequiresRPCURL(t *testing.T) {
	if _, err := NewBroadcaster("", BroadcasterOptions{}); err == nil {
		t.Fatal("expected error for empty RPC URL, got nil")
	}
}

var _ = base64.StdEncoding // keep import
