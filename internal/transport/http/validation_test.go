package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/sonncat/template/api"
	"github.com/sonncat/template/internal/domain"
)

func TestCreateTripValidationError(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		message string
	}{
		{name: "user", field: "user_id", message: "is required"},
		{name: "driver", field: "driver_id", message: "is required"},
		{name: "price", field: "price", message: "must be greater than zero"},
		{name: "start point", field: "start_point", message: "coordinates are out of range"},
		{name: "end point", field: "end_point", message: "coordinates are out of range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			create := NewMockcreateTripUseCase(ctrl)

			create.EXPECT().
				Execute(gomock.Any(), gomock.Any()).
				Return(nil, &domain.ValidationError{
					Field:   tt.field,
					Message: tt.message,
				})

			handler := NewHandler(nil, create, nil, nil, time.Second)
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/trips",
				strings.NewReader(validTripBody),
			)
			response := httptest.NewRecorder()

			handler.CreateTrip(response, request, api.CreateTripParams{})

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", response.Code)
			}

			var problem api.Problem
			if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
				t.Fatalf("decode problem: %v", err)
			}
			if problem.Code != "invalid_request" {
				t.Errorf("code = %q, want invalid_request", problem.Code)
			}
			if problem.Detail == nil || !strings.Contains(*problem.Detail, tt.field) {
				t.Errorf("detail = %v, want field %q", problem.Detail, tt.field)
			}
		})
	}
}
