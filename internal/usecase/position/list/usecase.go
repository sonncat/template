package list

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/model"
)

type UseCase struct {
	positions positionRepository
}

func New(positions positionRepository) *UseCase {
	return &UseCase{positions: positions}
}

func (u *UseCase) Execute(
	ctx context.Context,
	tripID uuid.UUID,
) ([]model.Position, error) {
	positions, err := u.positions.ListPositions(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("list positions: %w", err)
	}

	return positions, nil
}
