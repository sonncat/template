package trip

import (
	"time"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/domain"
)

const (
	StatusActive    = "active"
	StatusCompleted = "completed"
)

// Trip содержит данные и доменное состояние поездки
type Trip struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
	Status         string
	StartedAt      time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Finish завершает активную поездку в указанное время
func (t *Trip) Finish(at time.Time) error {
	if t.Status != StatusActive {
		return domain.ErrTripCompleted
	}

	t.Status = StatusCompleted
	t.FinishedAt = &at

	return nil
}
