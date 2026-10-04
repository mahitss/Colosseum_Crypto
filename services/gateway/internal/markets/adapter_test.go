package markets

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"qevryn/gateway/internal/requestid"
	"qevryn/types"
)

func TestAdapterClientUsesDomainRoutesAndRequestID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/markets/" || r.URL.Query().Get("status") != "primary" || r.URL.Query().Get("limit") != "7" {
			t.Errorf("unexpected request URL: %s", r.URL.String())
		}
		if r.Header.Get("X-Request-Id") != "correlation-1" {
			t.Errorf("request id = %q", r.Header.Get("X-Request-Id"))
		}
		_, _ = io.WriteString(w, `{"items":[],"nextCursor":null}`)
	}))
	defer server.Close()
	client, err := NewAdapterClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := requestid.With(context.Background(), "correlation-1")
	page, err := client.ListMarkets(ctx, types.MarketQuery{Status: "primary", Limit: 7})
	if err != nil {
		t.Fatal(err)
	}
	if page.Items == nil {
		t.Fatal("expected a typed market list")
	}
}

func TestAdapterClientMapsNotFoundWithoutUpstreamBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"private adapter detail"}`)
	}))
	defer server.Close()
	client, err := NewAdapterClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.GetMarket(context.Background(), "11111111111111111111111111111111")
	if err != ErrNotFound {
		t.Fatalf("error = %v", err)
	}
}

