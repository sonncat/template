package create

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonncat/template/internal/domain"
	"github.com/sonncat/template/internal/model"
)

func validTrip() *model.Trip {
	return &model.Trip{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  59.9398,
		StartLongitude: 30.3146,
		EndLatitude:    59.9290,
		EndLongitude:   30.3626,
		Price:          1450,
	}
}

func TestExecute(t *testing.T) {
	t.Run("creates a trip", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		ctx := context.Background()
		trip := validTrip()

		tx.EXPECT().
			Do(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			})

		repo.EXPECT().
			Create(gomock.Any(), trip).
			Return(nil)

		repo.EXPECT().
			GetByID(gomock.Any(), trip.ID).
			Return(trip, nil)

		usecase := New(repo, tx)
		got, err := usecase.Execute(ctx, trip)
		if err != nil {
			t.Fatalf("Execute() error = %v, want nil", err)
		}
		if got != trip {
			t.Errorf("Execute() = %v, want %v", got, trip)
		}
	})

	t.Run("preserves driver busy error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		ctx := context.Background()
		trip := validTrip()

		tx.EXPECT().
			Do(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			})

		repo.EXPECT().
			Create(gomock.Any(), trip).
			Return(domain.ErrDriverBusy)

		usecase := New(repo, tx)
		got, err := usecase.Execute(ctx, trip)

		if !errors.Is(err, domain.ErrDriverBusy) {
			t.Fatalf("Execute() error = %v, want ErrDriverBusy", err)
		}
		if got != nil {
			t.Errorf("Execute() = %v, want nil", got)
		}
	})

	t.Run("rejects invalid trip before transaction", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		trip := validTrip()
		trip.Price = -1

		usecase := New(repo, tx)
		got, err := usecase.Execute(context.Background(), trip)

		var validationErr *domain.ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("Execute() error = %v, want ValidationError", err)
		}
		if validationErr.Field != "price" {
			t.Errorf("field = %q, want price", validationErr.Field)
		}
		if got != nil {
			t.Errorf("Execute() = %v, want nil", got)
		}
	})
}
