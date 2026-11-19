package list

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonncat/template/internal/domain"
	"github.com/sonncat/template/internal/model"
)

func TestExecute(t *testing.T) {
	t.Run("returns positions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMockpositionRepository(ctrl)

		tripID := uuid.New()
		positions := []model.Position{
			{
				ID:         1,
				TripID:     tripID,
				Latitude:   59.93,
				Longitude:  30.33,
				RecordedAt: time.Date(2026, time.November, 1, 10, 0, 0, 0, time.UTC),
			},
		}

		repo.EXPECT().
			ListPositions(gomock.Any(), tripID).
			Return(positions, nil)

		got, err := New(repo).Execute(context.Background(), tripID)
		if err != nil {
			t.Fatalf("Execute() error = %v, want nil", err)
		}
		if len(got) != 1 {
			t.Fatalf("positions count = %d, want 1", len(got))
		}
		if got[0].ID != positions[0].ID || got[0].TripID != tripID {
			t.Errorf("position = %+v, want %+v", got[0], positions[0])
		}
	})

	t.Run("preserves trip not found error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMockpositionRepository(ctrl)

		tripID := uuid.New()

		repo.EXPECT().
			ListPositions(gomock.Any(), tripID).
			Return(nil, domain.ErrTripNotFound)

		_, err := New(repo).Execute(context.Background(), tripID)
		if !errors.Is(err, domain.ErrTripNotFound) {
			t.Errorf("error = %v, want ErrTripNotFound", err)
		}
	})
}
