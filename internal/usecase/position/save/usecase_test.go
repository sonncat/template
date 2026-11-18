package save

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
	t.Run("saves a valid position in a transaction", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMockpositionRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		position := &model.Position{
			TripID:     uuid.New(),
			Latitude:   59.93,
			Longitude:  30.33,
			RecordedAt: time.Now().UTC().Add(-time.Minute),
		}

		tx.EXPECT().
			Do(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			})

		repo.EXPECT().
			SavePosition(gomock.Any(), position).
			Return(nil)

		got, err := New(repo, tx).Execute(context.Background(), position)
		if err != nil {
			t.Fatalf("Execute() error = %v, want nil", err)
		}
		if got != position {
			t.Errorf("Execute() = %v, want %v", got, position)
		}
	})

	t.Run("rejects invalid coordinates before transaction", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMockpositionRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		position := &model.Position{
			TripID:     uuid.New(),
			Latitude:   91,
			Longitude:  30.33,
			RecordedAt: time.Now().UTC(),
		}

		_, err := New(repo, tx).Execute(context.Background(), position)

		var validationErr *domain.ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("error = %v, want validation error", err)
		}
		if validationErr.Field != "coordinates" {
			t.Errorf("field = %q, want coordinates", validationErr.Field)
		}
	})

	t.Run("preserves repository error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMockpositionRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		position := &model.Position{
			TripID:     uuid.New(),
			Latitude:   59.93,
			Longitude:  30.33,
			RecordedAt: time.Now().UTC(),
		}

		tx.EXPECT().
			Do(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			})

		repo.EXPECT().
			SavePosition(gomock.Any(), position).
			Return(domain.ErrTripCompleted)

		_, err := New(repo, tx).Execute(context.Background(), position)
		if !errors.Is(err, domain.ErrTripCompleted) {
			t.Errorf("error = %v, want ErrTripCompleted", err)
		}
	})
}

func TestExecuteRejectsPositionTooFarInFuture(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockpositionRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	position := &model.Position{
		TripID:     uuid.New(),
		Latitude:   59.93,
		Longitude:  30.33,
		RecordedAt: time.Now().UTC().Add(2 * time.Minute),
	}

	_, err := New(repo, tx).Execute(context.Background(), position)

	var validationErr *domain.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if validationErr.Field != "recorded_at" {
		t.Errorf("field = %q, want recorded_at", validationErr.Field)
	}
}
