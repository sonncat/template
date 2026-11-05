package config

import (
	"strings"
	"testing"
	"time"
)

func setValidEnvironment(t *testing.T) {
	t.Helper()

	values := map[string]string{
		"HTTP_ADDR":                  ":8080",
		"LOG_LEVEL":                  "info",
		"SHUTDOWN_TIMEOUT":           "10s",
		"HTTP_READ_TIMEOUT":          "5s",
		"HTTP_READ_HEADER_TIMEOUT":   "2s",
		"HTTP_WRITE_TIMEOUT":         "10s",
		"HTTP_IDLE_TIMEOUT":          "60s",
		"DATABASE_URL":               "postgres://example",
		"DATABASE_MAX_CONNS":         "10",
		"DATABASE_MIN_CONNS":         "2",
		"DATABASE_MAX_CONN_LIFETIME": "30m",
		"DATABASE_CONNECT_TIMEOUT":   "5s",
		"DATABASE_QUERY_TIMEOUT":     "3s",
	}

	for key, value := range values {
		t.Setenv(key, value)
	}
}

func TestLoadHTTPTimeouts(t *testing.T) {
	setValidEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if cfg.HTTPReadTimeout != 5*time.Second {
		t.Errorf("read timeout = %v, want 5s", cfg.HTTPReadTimeout)
	}
	if cfg.HTTPHeaderTimeout != 2*time.Second {
		t.Errorf("header timeout = %v, want 2s", cfg.HTTPHeaderTimeout)
	}
	if cfg.HTTPWriteTimeout != 10*time.Second {
		t.Errorf("write timeout = %v, want 10s", cfg.HTTPWriteTimeout)
	}
	if cfg.HTTPIdleTimeout != 60*time.Second {
		t.Errorf("idle timeout = %v, want 60s", cfg.HTTPIdleTimeout)
	}
}

func TestLoadRejectsMissingHTTPTimeout(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("HTTP_READ_TIMEOUT", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "HTTP_READ_TIMEOUT") {
		t.Fatalf("Load() error = %v, want missing HTTP_READ_TIMEOUT", err)
	}
}

func TestLoadRejectsInvalidHTTPTimeout(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("HTTP_READ_TIMEOUT", "-1s")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "HTTP_READ_TIMEOUT") {
		t.Fatalf("Load() error = %v, want invalid HTTP_READ_TIMEOUT", err)
	}
}
