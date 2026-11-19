package repository

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/sonncat/template/internal/domain"
	"github.com/sonncat/template/internal/infra/postgres/transaction"
	"github.com/sonncat/template/internal/model"
)

// SavePosition сохраняет координату активной поездки
// Вызывать внутри транзакции через TxManager
func (r *TripsRepository) SavePosition(
	ctx context.Context,
	position *model.Position,
) error {
	exec := txmanager.ExecutorFromContext(ctx, r.pool)

	// Условный UPDATE блокирует строку поездки до конца транзакции
	// Поэтому параллельное завершение не сможет пройти между проверкой
	// статуса и сохранением координаты
	update := sq.Update("trips").
		Set("last_position_at", sq.Expr(
			"GREATEST(COALESCE(last_position_at, ?), ?)",
			position.RecordedAt,
			position.RecordedAt,
		)).
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{
			"id":     position.TripID,
			"status": "active",
		}).
		PlaceholderFormat(sq.Dollar)

	sql, args, err := update.ToSql()
	if err != nil {
		return fmt.Errorf("build update last position: %w", err)
	}

	result, err := exec.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("update last position: %w", err)
	}

	if result.RowsAffected() == 0 {
		trip, err := r.GetByID(ctx, position.TripID)
		if err != nil {
			return fmt.Errorf("get trip for position: %w", err)
		}

		if trip.Status == "completed" {
			return domain.ErrTripCompleted
		}

		return domain.ErrTripNotFound
	}

	insert := sq.Insert("trip_positions").
		Columns("trip_id", "latitude", "longitude", "recorded_at").
		Values(
			position.TripID,
			position.Latitude,
			position.Longitude,
			position.RecordedAt,
		).
		Suffix("RETURNING id").
		PlaceholderFormat(sq.Dollar)

	sql, args, err = insert.ToSql()
	if err != nil {
		return fmt.Errorf("build insert position: %w", err)
	}

	if err := exec.QueryRow(ctx, sql, args...).Scan(&position.ID); err != nil {
		return fmt.Errorf("insert position: %w", err)
	}

	return nil
}

// ListPositions возвращает все точки маршрута в порядке времени и ID
func (r *TripsRepository) ListPositions(
	ctx context.Context,
	tripID uuid.UUID,
) ([]model.Position, error) {
	exec := txmanager.ExecutorFromContext(ctx, r.pool)

	// Без отдельной проверки пустой маршрут невозможно отличить
	// от отсутствующей поездки.
	if _, err := r.GetByID(ctx, tripID); err != nil {
		return nil, fmt.Errorf("get trip for route: %w", err)
	}

	query := sq.Select("id", "trip_id", "latitude", "longitude", "recorded_at").
		From("trip_positions").
		Where(sq.Eq{"trip_id": tripID}).
		OrderBy("recorded_at ASC", "id ASC").
		PlaceholderFormat(sq.Dollar)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select positions: %w", err)
	}

	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("select positions: %w", err)
	}
	defer rows.Close()

	positions := make([]model.Position, 0)

	for rows.Next() {
		var position model.Position

		if err := rows.Scan(
			&position.ID,
			&position.TripID,
			&position.Latitude,
			&position.Longitude,
			&position.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan position: %w", err)
		}

		positions = append(positions, position)
	}

	if err := rows.Err(); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("read positions: %w", err)
	}

	return positions, nil
}
