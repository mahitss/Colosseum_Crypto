package marketstudio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Interpreter turns a natural-language description into a draft.
//
// It is deliberately a narrow interface. The gateway treats the interpreter's
// output as a *suggestion*: it is re-validated here before anything reaches
// Panta, and an unconfirmed resolution source blocks creation regardless of
// what the model proposed.
type Interpreter interface {
	Interpret(ctx context.Context, description string) (InterpretationResult, error)
}

// InterpretationResult is the assistant's answer, which may be a draft, a
// request for clarification, or both.
type InterpretationResult struct {
	NeedsClarification bool              `json:"needs_clarification"`
	MissingFields      []string          `json:"missing_fields"`
	Clarification      string            `json:"clarification,omitempty"`
	Draft              *Draft            `json:"draft"`
	Validation         *ValidationReport `json:"validation,omitempty"`
	UsedFallback       bool              `json:"used_fallback"`
}

// IntelligenceClient talks to the Python Market Studio service.
type IntelligenceClient struct {
	baseURL    string
	httpClient *http.Client
}

// MaxDescriptionLength bounds what we forward. The Python service enforces the
// same limit; this copy exists so an oversized request is rejected before it
// costs a network round trip.
const MaxDescriptionLength = 4000

func NewIntelligenceClient(baseURL string, timeout time.Duration) (*IntelligenceClient, error) {
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Host == "" {
		return nil, errors.New("INTELLIGENCE_SERVICE_URL must be an absolute HTTP(S) URL")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &IntelligenceClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

func (c *IntelligenceClient) Interpret(ctx context.Context, description string) (InterpretationResult, error) {
	trimmed := strings.TrimSpace(description)
	if trimmed == "" {
		return InterpretationResult{}, ErrDraftRequired
	}
	if len(trimmed) > MaxDescriptionLength {
		return InterpretationResult{}, errors.New("description exceeds the maximum supported length")
	}

	body, err := json.Marshal(map[string]string{"prompt": trimmed})
	if err != nil {
		return InterpretationResult{}, errors.New("interpret request encode failed")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/market-studio/interpret", bytes.NewReader(body))
	if err != nil {
		return InterpretationResult{}, errors.New("interpret request build failed")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return InterpretationResult{}, &Failure{Code: ErrorIntelligenceServiceOffline, Err: err}
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusBadGateway || response.StatusCode >= 500 {
		return InterpretationResult{}, &Failure{Code: ErrorIntelligenceServiceOffline,
			Message: "market architect service unavailable"}
	}
	if response.StatusCode == http.StatusBadRequest {
		return InterpretationResult{}, ErrDraftRequired
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return InterpretationResult{}, errors.New("interpretation failed")
	}

	var result InterpretationResult
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return InterpretationResult{}, errors.New("interpretation returned malformed data")
	}
	if result.Draft == nil {
		result.NeedsClarification = true
	}
	return result, nil
}
