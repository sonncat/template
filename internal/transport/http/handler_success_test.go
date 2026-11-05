package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonncat/template/api"
	"github.com/sonncat/template/internal/model"
)

func TestCreateTripSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	create := NewMockcreateTripUseCase(ctrl)

	create.EXPECT().
		Execute(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, trip *model.Trip) (*model.Trip, error) {
			return trip, nil
		})

	handler := NewHandler(nil, create, nil, nil, time.Second)

	body := validTripBody

	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(body))
	response := httptest.NewRecorder()

	handler.CreateTrip(response, request, api.CreateTripParams{})

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", response.Code)
	}

	var created api.Trip
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.Id == uuid.Nil {
		t.Error("created trip has an empty ID")
	}
	if created.Status != api.TripStatus("active") {
		t.Errorf("status = %q, want active", created.Status)
	}

	wantLocation := "/api/v1/trips/" + created.Id.String()
	if got := response.Header().Get("Location"); got != wantLocation {
		t.Errorf("Location = %q, want %q", got, wantLocation)
	}
}

func TestFinishTripSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	finish := NewMockfinishTripUseCase(ctrl)

	id := uuid.New()
	finishedAt := time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC)
	finished := &model.Trip{
		ID:         id,
		Status:     "completed",
		StartedAt:  finishedAt.Add(-time.Hour),
		FinishedAt: &finishedAt,
	}

	finish.EXPECT().
		Execute(gomock.Any(), id).
		Return(finished, nil)

	handler := NewHandler(nil, nil, nil, finish, time.Second)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id.String()+"/finish", nil)
	response := httptest.NewRecorder()

	handler.FinishTrip(response, request, id)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}

	var got api.Trip
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Id != id {
		t.Errorf("ID = %v, want %v", got.Id, id)
	}
	if got.Status != api.TripStatus("completed") {
		t.Errorf("status = %q, want completed", got.Status)
	}
	if got.FinishedAt == nil || !got.FinishedAt.Equal(finishedAt) {
		t.Errorf("finished_at = %v, want %v", got.FinishedAt, finishedAt)
	}
}
