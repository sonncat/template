package create

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonncat/template/internal/model"
)

func TestHandleRejectsMissingCoordinateFields(t *testing.T) {
	const body = `{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c","driver_id":"7aa2363d-b025-4b6d-82af-f1c81c6e6bd0","start_point":{"latitude":1,"longitude":2},"end_point":{"latitude":3,"longitude":4},"price":1}`

	tests := []struct {
		name  string
		old   string
		new   string
		field string
	}{
		{
			name:  "missing latitude",
			old:   `"start_point":{"latitude":1,"longitude":2}`,
			new:   `"start_point":{"longitude":2}`,
			field: "start_point.latitude",
		},
		{
			name:  "missing longitude",
			old:   `"end_point":{"latitude":3,"longitude":4}`,
			new:   `"end_point":{"latitude":3}`,
			field: "end_point.longitude",
		},
		{
			name:  "null start point",
			old:   `"start_point":{"latitude":1,"longitude":2}`,
			new:   `"start_point":null`,
			field: "start_point",
		},
		{
			name:  "missing price",
			old:   `,"price":1`,
			new:   "",
			field: "price",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestBody := strings.Replace(body, tt.old, tt.new, 1)
			if requestBody == body {
				t.Fatal("test body was not changed")
			}

			handler := New(nil, time.Second)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(requestBody))
			response := httptest.NewRecorder()

			handler.Handle(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", response.Code)
			}
			if !strings.Contains(response.Body.String(), tt.field) {
				t.Errorf("response = %s, want field %q", response.Body.String(), tt.field)
			}
		})
	}
}

func TestHandleAcceptsZeroCoordinates(t *testing.T) {
	ctrl := gomock.NewController(t)
	usecase := NewMocktripUseCase(ctrl)

	usecase.EXPECT().
		Execute(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, trip *model.Trip) (*model.Trip, error) {
			if trip.StartLatitude != 0 || trip.StartLongitude != 0 {
				t.Errorf("start point = (%v, %v), want (0, 0)", trip.StartLatitude, trip.StartLongitude)
			}
			trip.ID = uuid.New()
			trip.Status = "active"
			trip.StartedAt = time.Now().UTC()
			return trip, nil
		})

	body := `{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c","driver_id":"7aa2363d-b025-4b6d-82af-f1c81c6e6bd0","start_point":{"latitude":0,"longitude":0},"end_point":{"latitude":3,"longitude":4},"price":1}`

	handler := New(usecase, time.Second)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(body))
	response := httptest.NewRecorder()

	handler.Handle(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", response.Code, response.Body.String())
	}
}
