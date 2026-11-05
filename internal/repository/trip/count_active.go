package repository

import (
	"context"
	"fmt"

	"github.com/sonncat/template/internal/domain/trip"
	"github.com/sonncat/template/internal/infra/postgres/transaction"
)

// CountActive возвращает число поездок в активном состоянии
func (r *TripsRepository) CountActive(ctx context.Context) (int64, error) {
	exec := txmanager.ExecutorFromContext(ctx, r.pool)

	var count int64
	if err := exec.QueryRow(
		ctx,
		"SELECT count(*) FROM trips WHERE status = $1",
		trip.StatusActive,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active trips: %w", err)
	}

	return count, nil
}
