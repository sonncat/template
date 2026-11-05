package trip

import (
	"time"

	"github.com/sonncat/template/internal/domain"
	"github.com/sonncat/template/internal/model"
)

const (
	StatusActive    = "active"
	StatusCompleted = "completed"
)

func Finish(t *model.Trip, at time.Time) error {
	if t.Status != StatusActive {
		return domain.ErrTripCompleted
	}

	t.Status = StatusCompleted
	t.FinishedAt = &at

	return nil
}
