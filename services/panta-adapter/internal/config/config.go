package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://live-api.panta.market/api/v1/"

// Config contains process configuration for the Panta adapter.
type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

func Load() (Config, error) {
	baseURL := strings.TrimSpace(os.Getenv("PANTA_API_BASE_URL"))
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return Config{}, fmt.Errorf("PANTA_API_BASE_URL must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/"
	baseURL = parsed.String()

	timeoutSeconds := 10
	if raw := strings.TrimSpace(os.Getenv("PANTA_API_TIMEOUT_SECONDS")); raw != "" {
		timeoutSeconds, err = strconv.Atoi(raw)
		if err != nil || timeoutSeconds < 1 || timeoutSeconds > 120 {
			return Config{}, fmt.Errorf("PANTA_API_TIMEOUT_SECONDS must be an integer from 1 to 120")
		}
	}

	return Config{
		BaseURL: baseURL,
		APIKey:  strings.TrimSpace(os.Getenv("PANTA_API_KEY")),
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}, nil
}

func (c Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("PANTA_API_KEY is required")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("PANTA_API_TIMEOUT_SECONDS must be positive")
	}
	parsed, err := url.ParseRequestURI(c.BaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("PANTA_API_BASE_URL must be an absolute HTTP(S) URL")
	}
	return nil
}
