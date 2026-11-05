package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sonncat/template/api"
)

func TestCreateTripValidation(t *testing.T) {
	validBody := validTripBody

	tests := []struct {
		name      string
		oldValue  string
		newValue  string
		wantField string
	}{
		{
			name:      "missing user",
			oldValue:  `"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c"`,
			newValue:  `"user_id":"00000000-0000-0000-0000-000000000000"`,
			wantField: "user_id",
		},
		{
			name:      "missing driver",
			oldValue:  `"driver_id":"7aa2363d-b025-4b6d-82af-f1c81c6e6bd0"`,
			newValue:  `"driver_id":"00000000-0000-0000-0000-000000000000"`,
			wantField: "driver_id",
		},
		{
			name:      "zero price",
			oldValue:  `"price":1450`,
			newValue:  `"price":0`,
			wantField: "price",
		},
		{
			name:      "invalid start coordinates",
			oldValue:  `"start_point":{"latitude":59.9398,"longitude":30.3146}`,
			newValue:  `"start_point":{"latitude":91,"longitude":30.3146}`,
			wantField: "start_point",
		},
		{
			name:      "invalid end coordinates",
			oldValue:  `"end_point":{"latitude":59.9290,"longitude":30.3626}`,
			newValue:  `"end_point":{"latitude":59.9290,"longitude":181}`,
			wantField: "end_point",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.Replace(validBody, tt.oldValue, tt.newValue, 1)
			if body == validBody {
				t.Fatal("test body was not changed")
			}

			handler := NewHandler(nil, nil, nil, nil, time.Second)
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/trips",
				strings.NewReader(body),
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
			if problem.Detail == nil || !strings.Contains(*problem.Detail, tt.wantField) {
				t.Errorf("detail = %v, want field %q", problem.Detail, tt.wantField)
			}
		})
	}
}
