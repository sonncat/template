package position

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

func TestCreatePosition(t *testing.T) {
	t.Run("saves position including zero coordinates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		save := NewMocksaveUseCase(ctrl)

		tripID := uuid.New()

		save.EXPECT().
			Execute(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, position *model.Position) (*model.Position, error) {
				if position.TripID != tripID {
					t.Errorf("trip ID = %v, want %v", position.TripID, tripID)
				}
				if position.Latitude != 0 || position.Longitude != 0 {
					t.Errorf(
						"coordinates = (%v, %v), want (0, 0)",
						position.Latitude,
						position.Longitude,
					)
				}

				position.ID = 42
				return position, nil
			})

		handler := New(save, nil, time.Second)
		body := `{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"2026-01-01T12:00:00Z"}`
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		response := httptest.NewRecorder()

		handler.Create(response, request, tripID)

		if response.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", response.Code, response.Body.String())
		}

		var got api.TripPosition
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if got.Id != 42 || got.TripId != tripID {
			t.Errorf("position = %+v, want ID 42 and trip ID %v", got, tripID)
		}
	})

	t.Run("rejects missing latitude", func(t *testing.T) {
		handler := New(nil, nil, time.Second)

		body := `{"coordinates":{"longitude":30.33},"recorded_at":"2026-01-01T12:00:00Z"}`
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		response := httptest.NewRecorder()

		handler.Create(response, request, uuid.New())

		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), `"code":"invalid_request"`) {
			t.Errorf("body = %s, want invalid_request", response.Body.String())
		}
	})

	t.Run("rejects unknown fields", func(t *testing.T) {
		handler := New(nil, nil, time.Second)

		body := `{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"2026-01-01T12:00:00Z","extra":true}`
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		response := httptest.NewRecorder()

		handler.Create(response, request, uuid.New())

		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", response.Code, response.Body.String())
		}
	})
}

func TestListPositionsReturnsEmptyArray(t *testing.T) {
	ctrl := gomock.NewController(t)
	list := NewMocklistUseCase(ctrl)

	tripID := uuid.New()

	list.EXPECT().
		Execute(gomock.Any(), tripID).
		Return([]model.Position{}, nil)

	handler := New(nil, list, time.Second)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	handler.List(response, request, tripID)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}

	var got api.TripPositionsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Positions == nil {
		t.Fatal("positions is null, want an empty array")
	}
	if len(got.Positions) != 0 {
		t.Errorf("positions count = %d, want 0", len(got.Positions))
	}
}
