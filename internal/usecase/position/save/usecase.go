package save

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/domain"
	"github.com/sonncat/template/internal/model"
)

type UseCase struct {
	positions positionRepository
	tx        txManager
}

func New(positions positionRepository, tx txManager) *UseCase {
	return &UseCase{
		positions: positions,
		tx:        tx,
	}
}

func (u *UseCase) Execute(
	ctx context.Context,
	position *model.Position,
) (*model.Position, error) {
	if position.TripID == uuid.Nil {
		return nil, &domain.ValidationError{
			Field:   "trip_id",
			Message: "is required",
		}
	}

	if position.Latitude < -90 || position.Latitude > 90 ||
		position.Longitude < -180 || position.Longitude > 180 {
		return nil, &domain.ValidationError{
			Field:   "coordinates",
			Message: "coordinates are out of range",
		}
	}

	if position.RecordedAt.IsZero() {
		return nil, &domain.ValidationError{
			Field:   "recorded_at",
			Message: "is required",
		}
	}

	if position.RecordedAt.After(time.Now().UTC().Add(time.Minute)) {
		return nil, &domain.ValidationError{
			Field:   "recorded_at",
			Message: "must not be more than one minute in the future",
		}
	}

	if err := u.tx.Do(ctx, func(txCtx context.Context) error {
		return u.positions.SavePosition(txCtx, position)
	}); err != nil {
		return nil, fmt.Errorf("save position: %w", err)
	}

	return position, nil
}
