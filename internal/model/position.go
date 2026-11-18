package model

import (
	"time"

	"github.com/google/uuid"
)

// Position — сохранённая координата поездки
type Position struct {
	ID         int64
	TripID     uuid.UUID
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
}
