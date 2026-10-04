package client

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adaptererrors "qevryn/panta-adapter/internal/errors"
)

const testAPIKey = "pk_test_do_not_log_this"
const testMarketID = "11111111111111111111111111111111"

func testClient(t *testing.T, server *httptest.Server, options Options) *Client {
	t.Helper()
	client, err := New(server.URL+"/api/v1", testAPIKey, time.Second, options)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestMarketAuthenticationBaseURLAndTrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/markets/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("limit") != "20" {
			t.Errorf("limit = %q", r.URL.Query().Get("limit"))
		}
		if got := r.Header.Get("X-Api-Key"); got != testAPIKey {
			t.Errorf("X-Api-Key was not set")
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("unexpected Authorization header")
		}
		if r.Header.Get("X-Request-Id") == "" {
			t.Error("X-Request-Id missing")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"items":[],"nextCursor":null}`)
	}))
	defer server.Close()

	page, err := testClient(t, server, Options{}).ListMarkets(context.Background(), PantaMarketQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Items == nil || len(page.Items) != 0 {
		t.Fatalf("items = %#v", page.Items)
	}
}

func TestGetMarketDecodesPricesAndUsesTrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/api/v1/markets/"+testMarketID+"/" {
			t.Errorf("escaped path = %q", r.URL.EscapedPath())
		}
		_, _ = io.WriteString(w, `{"marketId":"11111111111111111111111111111111","title":"Example","yesPrice":"0.52","noPrice":"0.48","volumeUsdc":"100.00"}`)
	}))
	defer server.Close()

	market, err := testClient(t, server, Options{}).GetMarket(context.Background(), testMarketID)
	if err != nil {
		t.Fatal(err)
	}
	if market.YesPrice == nil || *market.YesPrice != "0.52" {
		t.Fatalf("yes price = %#v", market.YesPrice)
	}
	if market.VolumeUSDC != "100.00" {
		t.Fatalf("volume = %q", market.VolumeUSDC)
	}
}

func TestMalformedMarketResponseIsTyped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"items":[{"marketId":""}]}`)
	}))
	defer server.Close()

	_, err := testClient(t, server, Options{}).ListMarkets(context.Background(), PantaMarketQuery{})
	var apiError *adaptererrors.Error
	if !errors.As(err, &apiError) || apiError.Kind != adaptererrors.KindMalformed {
		t.Fatalf("error = %#v", err)
	}
}

func TestRequestTimeoutRetriesTransientFailures(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		<-r.Context().Done()
	}))
	defer server.Close()
	client, err := New(server.URL, testAPIKey, 10*time.Millisecond, Options{
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.GetMarket(context.Background(), testMarketID)
	var apiError *adaptererrors.Error
	if !errors.As(err, &apiError) || apiError.Kind != adaptererrors.KindTimeout {
		t.Fatalf("error = %#v", err)
	}
	if attempts != defaultMaxAttempts {
		t.Fatalf("attempts = %d, want %d", attempts, defaultMaxAttempts)
	}
}

func TestHTTPErrorClassification(t *testing.T) {
	cases := []struct {
		status    int
		kind      adaptererrors.Kind
		wantTries int
	}{
		{http.StatusUnauthorized, adaptererrors.KindAuthentication, 1},
		{http.StatusForbidden, adaptererrors.KindAuthorization, 1},
		{http.StatusNotFound, adaptererrors.KindNotFound, 1},
		{http.StatusTooManyRequests, adaptererrors.KindRateLimit, 3},
		{http.StatusInternalServerError, adaptererrors.KindUpstream, 3},
	}
	for _, test := range cases {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			tries := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				tries++
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, `{"code":"UPSTREAM_CODE","message":"sensitive detail"}`)
			}))
			defer server.Close()

			client := testClient(t, server, Options{Sleep: func(context.Context, time.Duration) error { return nil }})
			_, err := client.GetMarket(context.Background(), testMarketID)
			var apiError *adaptererrors.Error
			if !errors.As(err, &apiError) || apiError.Kind != test.kind {
				t.Fatalf("error = %#v, want kind %q", err, test.kind)
			}
			if tries != test.wantTries {
				t.Fatalf("tries = %d, want %d", tries, test.wantTries)
			}
			if strings.Contains(err.Error(), "sensitive detail") || strings.Contains(err.Error(), testAPIKey) {
				t.Fatalf("unsafe error text: %q", err)
			}
		})
	}
}

func TestRetryAfterIsRespectedAndBounded(t *testing.T) {
	tries := 0
	var delays []time.Duration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		tries++
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	options := Options{Sleep: func(_ context.Context, duration time.Duration) error { delays = append(delays, duration); return nil }}
	_, err := testClient(t, server, options).GetMarket(context.Background(), testMarketID)
	if err == nil || tries != defaultMaxAttempts || len(delays) != defaultMaxAttempts-1 {
		t.Fatalf("err=%v tries=%d delays=%d", err, tries, len(delays))
	}
	for _, delay := range delays {
		if delay != 2*time.Second {
			t.Fatalf("retry delay = %s, want Retry-After 2s", delay)
		}
	}
}

func TestValidationFailureDoesNotRetryOrLogKey(t *testing.T) {
	var logOutput strings.Builder
	logger := slog.New(slog.NewJSONHandler(&logOutput, nil))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"code":"INVALID_MARKET_PARAMS","message":"pk_test_do_not_log_this"}`)
	}))
	defer server.Close()
	tries := 0
	options := Options{Logger: logger, Sleep: func(context.Context, time.Duration) error { return nil }}
	client := testClient(t, server, options)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		tries++
		return http.DefaultTransport.RoundTrip(req)
	})}
	_, err := client.GetMarket(context.Background(), testMarketID)
	if err == nil || tries != 1 {
		t.Fatalf("err=%v tries=%d", err, tries)
	}
	if strings.Contains(logOutput.String(), testAPIKey) || strings.Contains(logOutput.String(), "X-Api-Key") {
		t.Fatalf("secret found in logs: %s", logOutput.String())
	}
}

func TestRedirectNeverForwardsAPIKey(t *testing.T) {
	redirectTargetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirectTargetCalls++
		_, _ = io.WriteString(w, `{"marketId":"11111111111111111111111111111111","title":"unexpected"}`)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, nil, target.URL, http.StatusFound)
	}))
	defer source.Close()

	_, err := testClient(t, source, Options{}).GetMarket(context.Background(), testMarketID)
	if err == nil || redirectTargetCalls != 0 {
		t.Fatalf("redirect followed or returned success: err=%v targetCalls=%d", err, redirectTargetCalls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

