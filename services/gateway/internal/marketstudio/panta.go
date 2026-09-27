package marketstudio

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PantaClient is the subset of Panta's creation API that Market Studio uses.
//
// Only the three documented creation endpoints appear here. Nothing else in
// this file invents an endpoint or a field.
type PantaClient interface {
	CreateQuote(ctx context.Context, params PantaCreateParams) (CreateQuote, error)
	CreateBuild(ctx context.Context, createID, wallet string) (CreateBuild, error)
	Register(ctx context.Context, createID, signature string) (RegisterResponse, error)
}

// PantaHTTPClient is the production PantaClient.
//
// The RPC URL always comes from server configuration. The base URL keeps a
// trailing slash because Panta's documented paths end in a slash.
type PantaHTTPClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewPantaHTTPClient(baseURL, apiKey string, timeout time.Duration) (*PantaHTTPClient, error) {
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, errors.New("PANTA_API_URL must be an absolute HTTP(S) URL")
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("PANTA_API_KEY is required for market creation")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &PantaHTTPClient{
		baseURL:    strings.TrimRight(baseURL, "/") + "/",
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

func (c *PantaHTTPClient) doJSON(ctx context.Context, method, path string, body any, destination any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return errors.New("panta request encode failed")
		}
		reader = strings.NewReader(string(raw))
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return errors.New("panta request build failed")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Api-Key", c.apiKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return errors.New("panta unavailable")
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return pantaStatusError(response.StatusCode)
	}
	if destination == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(destination); err != nil {
		return errors.New("panta returned malformed data")
	}
	return nil
}

func pantaStatusError(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.New("panta authentication failed")
	case http.StatusTooManyRequests:
		return errors.New("panta rate limit exceeded")
	case http.StatusBadRequest:
		return errors.New("panta rejected request")
	case http.StatusNotFound:
		return errors.New("panta resource not found")
	case http.StatusConflict, http.StatusUnprocessableEntity:
		// Panta's registration verification is fail-closed, so a conflict here
		// means the signature did not verify. That is a real failure, not a
		// duplicate: the caller must not treat it as success.
		return errors.New("panta rejected request")
	default:
		return errors.New("panta request failed")
	}
}

// CreateQuote calls POST /api/v1/markets/create/quote/.
//
// The returned amounts are Panta's own values, kept as integer base-unit
// strings. We never estimate, never default and never compute a fee locally.
func (c *PantaHTTPClient) CreateQuote(ctx context.Context, params PantaCreateParams) (CreateQuote, error) {
	var quote CreateQuote
	if err := c.doJSON(ctx, http.MethodPost, "markets/create/quote/", params, &quote); err != nil {
		return CreateQuote{}, err
	}
	if quote.CreateID == "" {
		return CreateQuote{}, errors.New("panta quote missing create id")
	}
	if err := validateBaseUnitString("paymentUsdc", quote.PaymentUSDC); err != nil {
		return CreateQuote{}, err
	}
	if err := validateBaseUnitString("liquidityInjectionUsdc", quote.LiquidityInjectionUSDC); err != nil {
		return CreateQuote{}, err
	}
	if err := validateBaseUnitString("platformRevenueUsdc", quote.PlatformRevenueUSDC); err != nil {
		return CreateQuote{}, err
	}
	return quote, nil
}

// CreateBuild calls POST /api/v1/markets/create/build/.
//
// The returned transaction is unsigned and must be signed by the user's wallet.
// This server never signs and never has a key.
func (c *PantaHTTPClient) CreateBuild(ctx context.Context, createID, wallet string) (CreateBuild, error) {
	body := map[string]any{"createId": createID}
	if strings.TrimSpace(wallet) != "" {
		// Panta allows omitting the wallet; when present it must match the quote.
		body["wallet"] = wallet
	}

	var build CreateBuild
	if err := c.doJSON(ctx, http.MethodPost, "markets/create/build/", body, &build); err != nil {
		return CreateBuild{}, err
	}
	if strings.TrimSpace(build.Transaction) == "" {
		return CreateBuild{}, errors.New("panta build missing transaction")
	}
	if _, err := base64.StdEncoding.DecodeString(build.Transaction); err != nil {
		// A transaction we cannot decode is a transaction the wallet cannot sign.
		return CreateBuild{}, errors.New("panta build returned an undecodable transaction")
	}
	return build, nil
}

// Register calls POST /api/v1/markets/register/ with the confirmed signature.
//
// Note the path: Panta's registration endpoint is not under markets/create/.
// Only a response carrying a market id counts as success.
func (c *PantaHTTPClient) Register(ctx context.Context, createID, signature string) (RegisterResponse, error) {
	var result RegisterResponse
	err := c.doJSON(ctx, http.MethodPost, "markets/register/", map[string]any{
		"createId":  createID,
		"signature": signature,
	}, &result)
	if err != nil {
		return RegisterResponse{}, err
	}
	if strings.TrimSpace(result.MarketID) == "" {
		return RegisterResponse{}, errors.New("panta registration returned no market id")
	}
	return result, nil
}

// validateBaseUnitString rejects any amount that is not an integer base-unit
// string. This is the enforcement point for "never use float64 for money":
// a decimal or negative amount from upstream is treated as malformed data
// rather than silently accepted.
func validateBaseUnitString(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if _, err := ParseBaseUnits(value); err != nil {
		return errors.New("panta returned a malformed " + field + " amount")
	}
	return nil
}
