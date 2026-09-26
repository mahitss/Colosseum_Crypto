package config

import (
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	t.Setenv("PANTA_API_BASE_URL", "")
	t.Setenv("PANTA_API_KEY", "")
	t.Setenv("PANTA_API_TIMEOUT_SECONDS", "")
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.BaseURL != DefaultBaseURL || config.Timeout != 10*time.Second {
		t.Fatalf("unexpected defaults: %#v", config)
	}
	if err := config.Validate(); err == nil {
		t.Fatal("expected missing key to make configuration invalid")
	}
}

func TestBaseURLTrailingSlashAndTimeout(t *testing.T) {
	t.Setenv("PANTA_API_BASE_URL", "https://example.test/api/v1")
	t.Setenv("PANTA_API_KEY", "pk_test_x")
	t.Setenv("PANTA_API_TIMEOUT_SECONDS", "4")
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.BaseURL != "https://example.test/api/v1/" || config.Timeout != 4*time.Second {
		t.Fatalf("config = %#v", config)
	}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
}
