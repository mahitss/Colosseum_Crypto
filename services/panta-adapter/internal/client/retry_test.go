package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	adaptererrors "prophet/panta-adapter/internal/errors"
)

const retryTestMarketID = "11111111111111111111111111111111"

func TestNetworkFailureRetriesAndDeadlineTimesOut(t *testing.T) {
	attempts := 0
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		attempts++
		return nil, context.DeadlineExceeded
	})
	client, err := New("https://panta.example/api/v1/", testAPIKey, time.Second, Options{
		HTTPClient: &http.Client{Transport: transport},
		Sleep:      func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.GetMarket(context.Background(), retryTestMarketID)
	var apiError *adaptererrors.Error
	if !errorsAs(err, &apiError) || apiError.Kind != adaptererrors.KindTimeout {
		t.Fatalf("error = %#v", err)
	}
	if attempts != defaultMaxAttempts {
		t.Fatalf("attempts = %d, want %d", attempts, defaultMaxAttempts)
	}
}

func TestMalformedJSONIsNotRetried(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		_, _ = io.WriteString(w, `{"marketId":`)
	}))
	defer server.Close()
	_, err := testClient(t, server, Options{Sleep: func(context.Context, time.Duration) error { return nil }}).GetMarket(context.Background(), retryTestMarketID)
	var apiError *adaptererrors.Error
	if !errorsAs(err, &apiError) || apiError.Kind != adaptererrors.KindMalformed {
		t.Fatalf("error = %#v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func errorsAs(err error, target any) bool {
	return errors.As(err, target)
}
