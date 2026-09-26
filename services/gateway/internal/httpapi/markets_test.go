package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adapter "prophet/gateway/internal/markets"
	"prophet/types"
)

type fakeMarkets struct {
	list types.MarketPage
	one  types.Market
	err  error
}

func (f fakeMarkets) ListMarkets(context.Context, types.MarketQuery) (types.MarketPage, error) {
	return f.list, f.err
}

func (f fakeMarkets) GetMarket(context.Context, string) (types.Market, error) {
	return f.one, f.err
}

func TestMarketRoutesReturnProphetTypesAndRequestID(t *testing.T) {
	volume := types.HumanUSDC("20.00")
	service := fakeMarkets{
		list: types.MarketPage{Items: []types.Market{{ID: "id-1", Title: "domain market", VolumeUSDC: &volume}}},
		one:  types.Market{ID: "id-1", Title: "domain market"},
	}
	handler := NewHandlerWithMarkets(service)

	listResponse := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/markets/?limit=3&status=primary", nil)
	request.Header.Set("X-Request-Id", "gateway-correlation")
	handler.ServeHTTP(listResponse, request)
	if listResponse.Code != http.StatusOK || !contains(listResponse.Body.String(), `"volumeUsdc":"20.00"`) {
		t.Fatalf("unexpected list response: status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}
	if listResponse.Header().Get("X-Request-Id") != "gateway-correlation" {
		t.Fatal("request id was not echoed")
	}

	detailResponse := httptest.NewRecorder()
	handler.ServeHTTP(detailResponse, httptest.NewRequest(http.MethodGet, "/api/v1/markets/id-1", nil))
	if detailResponse.Code != http.StatusOK || !contains(detailResponse.Body.String(), `"id":"id-1"`) {
		t.Fatalf("unexpected detail response: status=%d body=%s", detailResponse.Code, detailResponse.Body.String())
	}
}

func TestMarketAdapterFailureIsSanitized(t *testing.T) {
	handler := NewHandlerWithMarkets(fakeMarkets{err: adapter.ErrInvalidQuery})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/markets/", nil))
	if response.Code != http.StatusBadRequest || contains(response.Body.String(), "upstream") {
		t.Fatalf("unexpected safe validation response: status=%d body=%s", response.Code, response.Body.String())
	}

	handler = NewHandlerWithMarkets(fakeMarkets{err: context.DeadlineExceeded})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/markets/", nil))
	if response.Code != http.StatusBadGateway || contains(response.Body.String(), "deadline") {
		t.Fatalf("unexpected sanitized upstream response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMarketServiceStatusMappings(t *testing.T) {
	cases := []struct {
		err        error
		statusCode int
	}{
		{adapter.ErrRateLimited, http.StatusTooManyRequests},
		{adapter.ErrTimeout, http.StatusGatewayTimeout},
		{adapter.ErrNotFound, http.StatusNotFound},
	}
	for _, test := range cases {
		response := httptest.NewRecorder()
		handler := NewHandlerWithMarkets(fakeMarkets{err: test.err})
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/markets/", nil))
		if response.Code != test.statusCode {
			t.Errorf("error %v: status=%d, want %d", test.err, response.Code, test.statusCode)
		}
	}
}

func contains(value, substring string) bool {
	return strings.Contains(value, substring)
}
