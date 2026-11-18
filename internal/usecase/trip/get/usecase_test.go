package get

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonncat/template/internal/domain"
	"github.com/sonncat/template/internal/model"
)

func TestExecute(t *testing.T) {
	t.Run("returns a trip", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)

		id := uuid.New()
		expected := &model.Trip{ID: id}

		repo.EXPECT().
			GetByID(gomock.Any(), id).
			Return(expected, nil)

		usecase := New(repo)
		got, err := usecase.Execute(context.Background(), id)
		if err != nil {
			t.Fatalf("Execute() error = %v, want nil", err)
		}
		if got != expected {
			t.Errorf("Execute() = %v, want %v", got, expected)
		}
	})

	t.Run("preserves trip not found error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)

		id := uuid.New()

		repo.EXPECT().
			GetByID(gomock.Any(), id).
			Return(nil, domain.ErrTripNotFound)

		usecase := New(repo)
		got, err := usecase.Execute(context.Background(), id)

		if !errors.Is(err, domain.ErrTripNotFound) {
			t.Fatalf("Execute() error = %v, want ErrTripNotFound", err)
		}
		if got != nil {
			t.Errorf("Execute() = %v, want nil", got)
		}
	})
}
