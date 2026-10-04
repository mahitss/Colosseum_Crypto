package client

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	adaptererrors "qevryn/panta-adapter/internal/errors"
	"qevryn/types"
)

const (
	defaultMaxAttempts = 3
	defaultBaseBackoff = 150 * time.Millisecond
	defaultMaxBackoff  = 2 * time.Second
	maxResponseBytes   = 2 << 20
	maxRetryAfter      = 30 * time.Second
)

type Options struct {
	HTTPClient  *http.Client
	Logger      *slog.Logger
	MaxAttempts int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	Sleep       func(context.Context, time.Duration) error
}

type Client struct {
	baseURL     *url.URL
	apiKey      string
	timeout     time.Duration
	httpClient  *http.Client
	logger      *slog.Logger
	maxAttempts int
	baseBackoff time.Duration
	maxBackoff  time.Duration
	sleep       func(context.Context, time.Duration) error
	circuitBreaker *CircuitBreaker
}

type requestIDKey struct{}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

func New(baseURL, apiKey string, timeout time.Duration, options Options) (*Client, error) {
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return nil, adaptererrors.New(adaptererrors.KindConfiguration)
	}
	if timeout <= 0 {
		return nil, adaptererrors.New(adaptererrors.KindConfiguration)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/"

	httpClient := options.HTTPClient
	if httpClient == nil {
		transport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, adaptererrors.New(adaptererrors.KindConfiguration)
		}
		cloned := transport.Clone()
		cloned.MaxIdleConns = 100
		cloned.MaxIdleConnsPerHost = 10
		cloned.IdleConnTimeout = 90 * time.Second
		httpClient = &http.Client{Transport: cloned}
	}
	clientCopy := *httpClient
	clientCopy.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	httpClient = &clientCopy
	maxAttempts := options.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = defaultMaxAttempts
	}
	if maxAttempts < 1 || maxAttempts > 5 {
		return nil, adaptererrors.New(adaptererrors.KindConfiguration)
	}
	baseBackoff := options.BaseBackoff
	if baseBackoff == 0 {
		baseBackoff = defaultBaseBackoff
	}
	maxBackoff := options.MaxBackoff
	if maxBackoff == 0 {
		maxBackoff = defaultMaxBackoff
	}
	if baseBackoff < 0 || maxBackoff < baseBackoff {
		return nil, adaptererrors.New(adaptererrors.KindConfiguration)
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	sleep := options.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	return &Client{
		baseURL: parsed, apiKey: apiKey, timeout: timeout, httpClient: httpClient,
		logger: logger, maxAttempts: maxAttempts, baseBackoff: baseBackoff,
		maxBackoff: maxBackoff, sleep: sleep,
		circuitBreaker: NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 2,
			Timeout:          30 * time.Second,
			MaxConcurrent:    50,
		}),
	}, nil
}

func (c *Client) ListMarkets(ctx context.Context, query PantaMarketQuery) (PantaMarketPage, error) {
	if query.Limit == 0 {
		query.Limit = 20
	}
	if query.Limit < 1 || query.Limit > 50 || (query.Status != "" && !validStatus(query.Status)) || (query.CreatedBy != "" && query.CreatedBy != "me") {
		return PantaMarketPage{}, adaptererrors.New(adaptererrors.KindValidation)
	}
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
	values.Set("limit", strconv.Itoa(query.Limit))

	var page PantaMarketPage
	if err := c.getJSON(ctx, "markets/", values, &page); err != nil {
		return PantaMarketPage{}, err
	}
	for _, market := range page.Items {
		if err := validateMarket(market); err != nil {
			return PantaMarketPage{}, err
		}
	}
	if page.Items == nil {
		return PantaMarketPage{}, adaptererrors.New(adaptererrors.KindMalformed)
	}
	return page, nil
}

func (c *Client) GetMarket(ctx context.Context, marketID string) (PantaMarket, error) {
	if !types.IsBase58Address(marketID) {
		return PantaMarket{}, adaptererrors.New(adaptererrors.KindValidation)
	}
	var market PantaMarket
	if err := c.getJSON(ctx, "markets/"+marketID+"/", nil, &market); err != nil {
		return PantaMarket{}, err
	}
	if err := validateMarket(market); err != nil {
		return PantaMarket{}, err
	}
	return market, nil
}

func (c *Client) GetAccount(ctx context.Context) (Account, error) {
	var account Account
	if err := c.getJSON(ctx, "account/", nil, &account); err != nil {
		return Account{}, err
	}
	if account.UserID == "" || account.Status == "" {
		return Account{}, adaptererrors.New(adaptererrors.KindMalformed)
	}
	return account, nil
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, destination any) error {
	if c.apiKey == "" {
		return adaptererrors.New(adaptererrors.KindConfiguration)
	}
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	if requestID == "" {
		var err error
		requestID, err = newRequestID()
		if err != nil {
			return adaptererrors.New(adaptererrors.KindNetwork)
		}
	}
	operationContext, operationCancel := context.WithTimeout(ctx, c.timeout*time.Duration(c.maxAttempts)+maxRetryAfter*time.Duration(c.maxAttempts-1))
	defer operationCancel()
	endpoint := *c.baseURL
	endpoint.Path += path
	if query != nil {
		endpoint.RawQuery = query.Encode()
	}

	var lastError error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		requestContext, cancel := context.WithTimeout(operationContext, c.timeout)
		req, requestErr := http.NewRequestWithContext(requestContext, http.MethodGet, endpoint.String(), nil)
		if requestErr != nil {
			cancel()
			return adaptererrors.New(adaptererrors.KindConfiguration)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Api-Key", c.apiKey)
		req.Header.Set("X-Request-Id", requestID)

		started := time.Now()
		response, requestErr := c.httpClient.Do(req)
		if requestErr != nil {
			cancel()
			lastError = networkError(requestErr, requestID)
			c.logAttempt(req, 0, requestID, attempt, started, lastError)
			if attempt == c.maxAttempts || !retryable(lastError) {
				return lastError
			}
			if err := c.sleep(operationContext, c.backoff(attempt, 0)); err != nil {
				return timeoutError(requestID)
			}
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
		_ = response.Body.Close()
		cancel()
		if readErr != nil {
			lastError = networkError(readErr, requestID)
			c.logAttempt(req, response.StatusCode, requestID, attempt, started, lastError)
			if attempt == c.maxAttempts || !retryable(lastError) {
				return lastError
			}
			if err := c.sleep(ctx, c.backoff(attempt, 0)); err != nil {
				return timeoutError(requestID)
			}
			continue
		}
		if len(body) > maxResponseBytes {
			lastError = &adaptererrors.Error{Kind: adaptererrors.KindMalformed, RequestID: requestID}
			c.logAttempt(req, response.StatusCode, requestID, attempt, started, lastError)
			return lastError
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			if err := decodeJSON(body, destination); err != nil {
				lastError = &adaptererrors.Error{Kind: adaptererrors.KindMalformed, StatusCode: response.StatusCode, RequestID: requestID}
				c.logAttempt(req, response.StatusCode, requestID, attempt, started, lastError)
				return lastError
			}
			c.logAttempt(req, response.StatusCode, requestID, attempt, started, nil)
			return nil
		}

		var apiError PantaErrorResponse
		_ = json.Unmarshal(body, &apiError)
		retryAfter := parseRetryAfter(response.Header.Get("Retry-After"), time.Now())
		lastError = adaptererrors.WithStatus(response.StatusCode, requestID, apiError.Code, retryAfter)
		c.logAttempt(req, response.StatusCode, requestID, attempt, started, lastError)
		if attempt == c.maxAttempts || !retryable(lastError) {
			return lastError
		}
		if err := c.sleep(operationContext, c.backoff(attempt, retryAfter)); err != nil {
			return timeoutError(requestID)
		}
	}
	return lastError
}

func (c *Client) logAttempt(req *http.Request, status int, requestID string, attempt int, started time.Time, err error) {
	attributes := []any{
		"method", req.Method,
		"path", req.URL.EscapedPath(),
		"status", status,
		"request_id", requestID,
		"attempt", attempt,
		"duration_ms", time.Since(started).Milliseconds(),
	}
	if err != nil {
		var safeError *adaptererrors.Error
		if errors.As(err, &safeError) {
			attributes = append(attributes, "error_kind", string(safeError.Kind))
		}
		c.logger.Warn("Panta request failed", attributes...)
		return
	}
	c.logger.Info("Panta request completed", attributes...)
}

func (c *Client) backoff(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > maxRetryAfter {
			return maxRetryAfter
		}
		return retryAfter
	}
	delay := c.baseBackoff
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= c.maxBackoff {
			delay = c.maxBackoff
			break
		}
	}
	jitter := time.Duration(rand.Float64() * float64(delay/4))
	delay = delay - delay/8 + jitter
	if delay > c.maxBackoff {
		return c.maxBackoff
	}
	return delay
}

func validStatus(status string) bool {
	switch status {
	case "primary", "secondary", "resolved", "cancelled":
		return true
	default:
		return false
	}
}

func validateMarket(market PantaMarket) error {
	if market.MarketID == "" || market.Title == "" {
		return adaptererrors.New(adaptererrors.KindMalformed)
	}
	return nil
}

func decodeJSON(body []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

func retryable(err error) bool {
	var safeError *adaptererrors.Error
	return errors.As(err, &safeError) && safeError.Retryable()
}

func networkError(err error, requestID string) *adaptererrors.Error {
	var netError net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netError) && netError.Timeout()) {
		return timeoutError(requestID)
	}
	return &adaptererrors.Error{Kind: adaptererrors.KindNetwork, RequestID: requestID}
}

func timeoutError(requestID string) *adaptererrors.Error {
	return &adaptererrors.Error{Kind: adaptererrors.KindTimeout, RequestID: requestID}
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil && when.After(now) {
		return when.Sub(now)
	}
	return 0
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func newRequestID() (string, error) {
	var bytes [16]byte
	if _, err := cryptorand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

