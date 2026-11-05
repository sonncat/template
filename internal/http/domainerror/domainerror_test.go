package domainerror

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/sonncat/template/internal/domain"
)

func TestProblem(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "trip not found",
			err:        domain.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
		},
		{
			name:       "completed trip",
			err:        domain.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
		},
		{
			name:       "driver busy",
			err:        fmt.Errorf("create trip: %w", domain.ErrDriverBusy),
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
		},
		{
			name:       "invalid request",
			err:        domain.ErrInvalidRequest,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "internal error",
			err:        errors.New("secret database error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := Problem(tt.err)

			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if body.Status != int32(tt.wantStatus) {
				t.Errorf("body status = %d, want %d", body.Status, tt.wantStatus)
			}
			if body.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", body.Code, tt.wantCode)
			}
			if body.Detail == nil {
				t.Fatal("detail is nil")
			}
			if strings.Contains(*body.Detail, "secret database error") {
				t.Error("internal error details leaked into response")
			}
		})
	}
}
