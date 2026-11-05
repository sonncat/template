package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecover(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	handler := Recover(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("secret panic detail")
	}))

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}
	if !strings.Contains(response.Body.String(), `"code":"internal_error"`) {
		t.Errorf("response = %s, want internal_error", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "secret panic detail") {
		t.Error("panic details leaked into response")
	}
	if !strings.Contains(logs.String(), "secret panic detail") {
		t.Error("panic details are missing from the log")
	}
	if !strings.Contains(logs.String(), `"stack"`) {
		t.Error("stack trace is missing from the log")
	}
}
