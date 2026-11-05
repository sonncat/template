package finish

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonncat/template/internal/domain"
	"github.com/sonncat/template/internal/domain/trip"
	"github.com/sonncat/template/internal/model"
)

func TestExecute(t *testing.T) {
	t.Run("finishes active trip", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		id := uuid.New()
		current := &model.Trip{ID: id, Status: trip.StatusActive}
		finished := &model.Trip{ID: id, Status: trip.StatusCompleted}

		tx.EXPECT().
			Do(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			})

		repo.EXPECT().
			GetByID(gomock.Any(), id).
			Return(current, nil)

		repo.EXPECT().
			Finish(gomock.Any(), id).
			Return(finished, nil)

		usecase := New(repo, tx)
		got, err := usecase.Execute(context.Background(), id)
		if err != nil {
			t.Fatalf("Execute() error = %v, want nil", err)
		}
		if got != finished {
			t.Errorf("Execute() = %v, want %v", got, finished)
		}
	})

	t.Run("does not finish completed trip", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		id := uuid.New()
		current := &model.Trip{ID: id, Status: trip.StatusCompleted}

		tx.EXPECT().
			Do(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			})

		repo.EXPECT().
			GetByID(gomock.Any(), id).
			Return(current, nil)

		usecase := New(repo, tx)
		got, err := usecase.Execute(context.Background(), id)

		if !errors.Is(err, domain.ErrTripCompleted) {
			t.Fatalf("Execute() error = %v, want ErrTripCompleted", err)
		}
		if got != nil {
			t.Errorf("Execute() = %v, want nil", got)
		}
	})
}
