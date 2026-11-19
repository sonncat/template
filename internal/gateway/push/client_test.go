package push

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRequestPosition(t *testing.T) {
	tripID := uuid.New()
	driverID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/push" {
			t.Errorf("path = %s, want /api/v1/push", r.URL.Path)
		}

		var body pushRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if body.RequestID == "" {
			t.Error("request_id is empty")
		}
		if body.RecipientID != driverID {
			t.Errorf("recipient_id = %v, want %v", body.RecipientID, driverID)
		}
		if body.Kind != "REQUEST_POSITION" {
			t.Errorf("kind = %q, want REQUEST_POSITION", body.Kind)
		}
		if body.Data["trip_id"] != tripID.String() {
			t.Errorf("trip_id = %q, want %s", body.Data["trip_id"], tripID)
		}

		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := New(server.URL, time.Second)

	if err := client.RequestPosition(context.Background(), tripID, driverID); err != nil {
		t.Fatalf("RequestPosition() error = %v, want nil", err)
	}
}

func TestRequestPositionReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := New(server.URL, time.Second)

	err := client.RequestPosition(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("RequestPosition() error = nil, want HTTP error")
	}
}
