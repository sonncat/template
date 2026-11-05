package create

import (
	"context"
	"fmt"

	"github.com/sonncat/template/internal/model"
)

// UseCase создаёт поездку
type UseCase struct {
	trips tripRepository
	tx    txManager
}

// New создаёт usecase с репозиторием и менеджером транзакций
func New(trips tripRepository, tx txManager) *UseCase {
	return &UseCase{
		trips: trips,
		tx:    tx,
	}
}

// Execute проверяет и сохраняет поездку, затем возвращает её данные из базы
func (u *UseCase) Execute(ctx context.Context, trip *model.Trip) (*model.Trip, error) {
	if err := trip.Validate(); err != nil {
		return nil, fmt.Errorf("validate trip: %w", err)
	}

	if err := u.tx.Do(ctx, func(txCtx context.Context) error {
		return u.trips.Create(txCtx, trip)
	}); err != nil {
		return nil, fmt.Errorf("create trip: %w", err)
	}

	created, err := u.trips.GetByID(ctx, trip.ID)
	if err != nil {
		return nil, fmt.Errorf("get created trip: %w", err)
	}

	return created, nil
}
