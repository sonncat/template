package get

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/model"
)

// UseCase получает поездку по идентификатору
type UseCase struct {
	trips tripRepository
}

// New создаёт usecase с переданным репозиторием
func New(trips tripRepository) *UseCase {
	return &UseCase{trips: trips}
}

// Execute возвращает поездку по её идентификатору
func (u *UseCase) Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	trip, err := u.trips.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get trip: %w", err)
	}

	return trip, nil
}
