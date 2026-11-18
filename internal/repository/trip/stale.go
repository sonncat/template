package repository

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/sonncat/template/internal/domain/trip"
	"github.com/sonncat/template/internal/infra/postgres/transaction"
	"github.com/sonncat/template/internal/model"
)

const statusColumn = "status"

// ListStaleTrips возвращает активные поездки без свежих координат
// Время последней координаты сравнивается с текущим временем в PostgreSQL
func (r *TripsRepository) ListStaleTrips(
	ctx context.Context,
	staleAfter time.Duration,
) ([]model.StaleTrip, error) {
	exec := txmanager.ExecutorFromContext(ctx, r.pool)

	query := sq.Select("id", "driver_id").
		From("trips").
		Where(sq.Eq{statusColumn: trip.StatusActive}).
		Where(sq.Expr(
			"COALESCE(last_position_at, started_at) < now() - (? * interval '1 second')",
			staleAfter.Seconds(),
		)).
		OrderBy("COALESCE(last_position_at, started_at) ASC", "id ASC").
		PlaceholderFormat(sq.Dollar)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build stale trips query: %w", err)
	}

	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("select stale trips: %w", err)
	}
	defer rows.Close()

	trips := make([]model.StaleTrip, 0)

	for rows.Next() {
		var staleTrip model.StaleTrip

		if err := rows.Scan(&staleTrip.TripID, &staleTrip.DriverID); err != nil {
			return nil, fmt.Errorf("scan stale trip: %w", err)
		}

		trips = append(trips, staleTrip)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read stale trips: %w", err)
	}

	return trips, nil
}
