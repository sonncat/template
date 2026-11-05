package trip

import (
	"github.com/google/uuid"

	"github.com/sonncat/template/internal/domain/validation"
)

const requiredMessage = "is required"

// Validate проверяет значения, необходимые для создания поездки
func (t *Trip) Validate() error {
	if t.UserID == uuid.Nil {
		return &validation.Error{Field: "user_id", Message: requiredMessage}
	}

	if t.DriverID == uuid.Nil {
		return &validation.Error{Field: "driver_id", Message: requiredMessage}
	}

	if t.Price < 0 {
		return &validation.Error{
			Field:   "price",
			Message: "must be greater than or equal to zero",
		}
	}

	if t.StartLatitude < -90 || t.StartLatitude > 90 ||
		t.StartLongitude < -180 || t.StartLongitude > 180 {
		return &validation.Error{
			Field:   "start_point",
			Message: "coordinates are out of range",
		}
	}

	if t.EndLatitude < -90 || t.EndLatitude > 90 ||
		t.EndLongitude < -180 || t.EndLongitude > 180 {
		return &validation.Error{
			Field:   "end_point",
			Message: "coordinates are out of range",
		}
	}

	return nil
}
