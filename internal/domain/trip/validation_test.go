package trip

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/domain/validation"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		change    func(*Trip)
		wantField string
	}{
		{
			name:   "valid trip",
			change: func(*Trip) {},
		},
		{
			name:      "missing user",
			change:    func(trip *Trip) { trip.UserID = uuid.Nil },
			wantField: "user_id",
		},
		{
			name:      "missing driver",
			change:    func(trip *Trip) { trip.DriverID = uuid.Nil },
			wantField: "driver_id",
		},
		{
			name:      "invalid price",
			change:    func(trip *Trip) { trip.Price = 0 },
			wantField: "price",
		},
		{
			name: "missing start point",
			change: func(trip *Trip) {
				trip.StartLatitude = 0
				trip.StartLongitude = 0
			},
			wantField: "start_point",
		},
		{
			name: "missing end point",
			change: func(trip *Trip) {
				trip.EndLatitude = 0
				trip.EndLongitude = 0
			},
			wantField: "end_point",
		},
		{
			name:      "invalid start latitude",
			change:    func(trip *Trip) { trip.StartLatitude = 91 },
			wantField: "start_point",
		},
		{
			name:      "invalid end longitude",
			change:    func(trip *Trip) { trip.EndLongitude = 181 },
			wantField: "end_point",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trip := &Trip{
				UserID:         uuid.New(),
				DriverID:       uuid.New(),
				StartLatitude:  59.9398,
				StartLongitude: 30.3146,
				EndLatitude:    59.9290,
				EndLongitude:   30.3626,
				Price:          1450,
			}

			tt.change(trip)
			err := trip.Validate()

			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}

			var validationErr *validation.Error
			if !errors.As(err, &validationErr) {
				t.Fatalf("Validate() error = %v, want validation error", err)
			}
			if validationErr.Field != tt.wantField {
				t.Errorf("field = %q, want %q", validationErr.Field, tt.wantField)
			}
		})
	}
}
