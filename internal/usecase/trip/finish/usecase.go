package finish

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/model"
)

// UseCase завершает поездку
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

// Execute завершает поездку и возвращает её обновлённые данные
func (u *UseCase) Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	var finished *model.Trip

	err := u.tx.Do(ctx, func(txCtx context.Context) error {
		current, err := u.trips.GetByID(txCtx, id)
		if err != nil {
			return fmt.Errorf("get trip: %w", err)
		}

		if err := current.Finish(time.Now().UTC()); err != nil {
			return fmt.Errorf("check trip status: %w", err)
		}

		finished, err = u.trips.Finish(txCtx, id)
		if err != nil {
			return fmt.Errorf("save finished trip: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("finish trip: %w", err)
	}

	return finished, nil
}
