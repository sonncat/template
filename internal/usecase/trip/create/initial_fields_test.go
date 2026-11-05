package create

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonncat/template/internal/domain/trip"
	"github.com/sonncat/template/internal/model"
)

func TestExecuteInitializesTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	newTrip := validTrip()
	newTrip.ID = uuid.Nil

	tx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

	repo.EXPECT().
		Create(gomock.Any(), newTrip).
		DoAndReturn(func(_ context.Context, saved *model.Trip) error {
			if saved.ID == uuid.Nil {
				t.Error("created trip ID is empty")
			}
			if saved.Status != trip.StatusActive {
				t.Errorf("status = %q, want active", saved.Status)
			}
			if saved.StartedAt.IsZero() {
				t.Error("started_at is empty")
			}
			return nil
		})

	repo.EXPECT().
		GetByID(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID) (*model.Trip, error) {
			if id != newTrip.ID {
				t.Errorf("requested ID = %v, want %v", id, newTrip.ID)
			}
			return newTrip, nil
		})

	before := time.Now().UTC()
	created, err := New(repo, tx).Execute(context.Background(), newTrip)
	after := time.Now().UTC()

	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if created.StartedAt.Before(before) || created.StartedAt.After(after) {
		t.Errorf("started_at = %v, want time between %v and %v", created.StartedAt, before, after)
	}
}
