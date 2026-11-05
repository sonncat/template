package domainerror

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sonncat/template/internal/domain"
)

func TestProblemForRequest(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantLog    bool
	}{
		{
			name:       "internal error is logged",
			err:        errors.New("database connection failed"),
			wantStatus: http.StatusInternalServerError,
			wantLog:    true,
		},
		{
			name:       "not found is not logged as internal error",
			err:        domain.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantLog:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))

			request := httptest.NewRequest(http.MethodGet, "/api/v1/trips/test", nil)
			request = request.WithContext(WithLogger(request.Context(), logger))

			status, problem := ProblemForRequest(request, tt.err)

			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if tt.wantLog && !strings.Contains(logs.String(), "database connection failed") {
				t.Errorf("internal error missing from log: %s", logs.String())
			}
			if !tt.wantLog && logs.Len() != 0 {
				t.Errorf("unexpected error log: %s", logs.String())
			}
			if problem.Detail != nil && strings.Contains(*problem.Detail, "database connection failed") {
				t.Error("internal error details leaked into response")
			}
		})
	}
}
