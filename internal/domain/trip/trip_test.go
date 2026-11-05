package trip

import (
	"errors"
	"testing"
	"time"

	"github.com/sonncat/template/internal/domain"
)

func TestFinish(t *testing.T) {
	finishedAt := time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC)

	t.Run("active trip is completed", func(t *testing.T) {
		trip := &Trip{Status: StatusActive}

		err := trip.Finish(finishedAt)
		if err != nil {
			t.Fatalf("Finish() error = %v, want nil", err)
		}
		if trip.Status != StatusCompleted {
			t.Errorf("status = %q, want %q", trip.Status, StatusCompleted)
		}
		if trip.FinishedAt == nil {
			t.Fatal("FinishedAt = nil, want completion time")
		}
		if !trip.FinishedAt.Equal(finishedAt) {
			t.Errorf("FinishedAt = %v, want %v", *trip.FinishedAt, finishedAt)
		}
	})

	t.Run("completed trip cannot be completed again", func(t *testing.T) {
		originalTime := finishedAt.Add(-time.Hour)
		trip := &Trip{
			Status:     StatusCompleted,
			FinishedAt: &originalTime,
		}

		err := trip.Finish(finishedAt)

		if !errors.Is(err, domain.ErrTripCompleted) {
			t.Fatalf("Finish() error = %v, want ErrTripCompleted", err)
		}
		if trip.Status != StatusCompleted {
			t.Errorf("status = %q, want %q", trip.Status, StatusCompleted)
		}
		if trip.FinishedAt == nil || !trip.FinishedAt.Equal(originalTime) {
			t.Errorf("FinishedAt = %v, want original time %v", trip.FinishedAt, originalTime)
		}
	})
}
