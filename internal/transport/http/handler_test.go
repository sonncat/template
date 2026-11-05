package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonncat/template/api"
	"github.com/sonncat/template/internal/domain"
)

func TestCreateTripInvalidBody(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil, time.Second)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader("{"))
	response := httptest.NewRecorder()

	handler.CreateTrip(response, request, api.CreateTripParams{})

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":"invalid_request"`) {
		t.Errorf("response = %s, want invalid_request", response.Body.String())
	}
}

func TestCreateTripDriverBusy(t *testing.T) {
	ctrl := gomock.NewController(t)
	create := NewMockcreateTripUseCase(ctrl)

	create.EXPECT().
		Execute(gomock.Any(), gomock.Any()).
		Return(nil, domain.ErrDriverBusy)

	handler := NewHandler(nil, create, nil, nil, time.Second)

	body := validTripBody

	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(body))
	response := httptest.NewRecorder()

	handler.CreateTrip(response, request, api.CreateTripParams{})

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":"driver_busy"`) {
		t.Errorf("response = %s, want driver_busy", response.Body.String())
	}
}

func TestGetTripNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	get := NewMockgetTripUseCase(ctrl)

	id := uuid.New()
	get.EXPECT().
		Execute(gomock.Any(), id).
		Return(nil, domain.ErrTripNotFound)

	handler := NewHandler(nil, nil, get, nil, time.Second)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+id.String(), nil)
	response := httptest.NewRecorder()

	handler.GetTrip(response, request, id)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":"trip_not_found"`) {
		t.Errorf("response = %s, want trip_not_found", response.Body.String())
	}
}

func TestFinishTripCompleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	finish := NewMockfinishTripUseCase(ctrl)

	id := uuid.New()
	finish.EXPECT().
		Execute(gomock.Any(), id).
		Return(nil, domain.ErrTripCompleted)

	handler := NewHandler(nil, nil, nil, finish, time.Second)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id.String()+"/finish", nil)
	response := httptest.NewRecorder()

	handler.FinishTrip(response, request, id)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":"trip_completed"`) {
		t.Errorf("response = %s, want trip_completed", response.Body.String())
	}
}

func TestGetTripInternalErrorDoesNotLeakDetails(t *testing.T) {
	ctrl := gomock.NewController(t)
	get := NewMockgetTripUseCase(ctrl)

	id := uuid.New()
	get.EXPECT().
		Execute(gomock.Any(), id).
		Return(nil, errors.New("secret database error"))

	handler := NewHandler(nil, nil, get, nil, time.Second)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+id.String(), nil)
	response := httptest.NewRecorder()

	handler.GetTrip(response, request, id)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if strings.Contains(response.Body.String(), "secret database error") {
		t.Errorf("internal details leaked in response: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"code":"internal_error"`) {
		t.Errorf("response = %s, want internal_error", response.Body.String())
	}
}
