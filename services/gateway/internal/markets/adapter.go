package markets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"qevryn/gateway/internal/requestid"
	"qevryn/types"
)

type Service interface {
	ListMarkets(context.Context, types.MarketQuery) (types.MarketPage, error)
	GetMarket(context.Context, string) (types.Market, error)
}

type AdapterClient struct {
	baseURL *url.URL
	client  *http.Client
}

func NewAdapterClient(baseURL string, timeout time.Duration) (*AdapterClient, error) {
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("PANTA_ADAPTER_URL must be an absolute HTTP(S) URL")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("adapter timeout must be positive")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return &AdapterClient{baseURL: parsed, client: &http.Client{Timeout: timeout}}, nil
}

func (c *AdapterClient) ListMarkets(ctx context.Context, query types.MarketQuery) (types.MarketPage, error) {
	values := url.Values{}
	if query.Category != "" {
		values.Set("category", query.Category)
	}
	if query.Status != "" {
		values.Set("status", query.Status)
	}
	if query.CreatedBy != "" {
		values.Set("createdBy", query.CreatedBy)
	}
	if query.Cursor != "" {
		values.Set("cursor", query.Cursor)
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	var result types.MarketPage
	if err := c.get(ctx, "markets/", values, &result); err != nil {
		return types.MarketPage{}, err
	}
	if result.Items == nil {
		return types.MarketPage{}, errors.New("market adapter returned malformed data")
	}
	return result, nil
}

func (c *AdapterClient) GetMarket(ctx context.Context, id string) (types.Market, error) {
	if !types.IsBase58Address(id) {
		return types.Market{}, errors.New("market id is invalid")
	}
	var result types.Market
	if err := c.get(ctx, "markets/"+id+"/", nil, &result); err != nil {
		return types.Market{}, err
	}
	if result.ID == "" || result.Title == "" {
		return types.Market{}, errors.New("market adapter returned malformed data")
	}
	return result, nil
}

func (c *AdapterClient) get(ctx context.Context, path string, query url.Values, destination any) error {
	endpoint := *c.baseURL
	endpoint.Path += "/" + strings.TrimLeft(path, "/")
	if query != nil {
		endpoint.RawQuery = query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return errors.New("market adapter request failed")
	}
	request.Header.Set("Accept", "application/json")
	if requestID := requestid.From(ctx); requestID != "" {
		request.Header.Set("X-Request-Id", requestID)
	}
	response, err := c.client.Do(request)
	if err != nil {
		var networkError net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
			return ErrTimeout
		}
		return errors.New("market adapter is unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode == http.StatusNotFound {
			return ErrNotFound
		}
		if response.StatusCode == http.StatusBadRequest {
			return ErrInvalidQuery
		}
		if response.StatusCode == http.StatusTooManyRequests {
			return ErrRateLimited
		}
		if response.StatusCode == http.StatusGatewayTimeout {
			return ErrTimeout
		}
		return errors.New("market adapter request failed")
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	if err := decoder.Decode(destination); err != nil {
		return errors.New("market adapter returned malformed data")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("market adapter returned malformed data")
	}
	return nil
}

var (
	ErrNotFound     = errors.New("market not found")
	ErrInvalidQuery = errors.New("market query is invalid")
	ErrRateLimited  = errors.New("market service is rate limited")
	ErrTimeout      = errors.New("market service timed out")
)

