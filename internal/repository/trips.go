package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sonncat/template/internal/domain"
	"github.com/sonncat/template/internal/txmanager"
)

type Trip struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
	Status         string
	StartedAt      time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type TripsRepository struct {
	pool *pgxpool.Pool
}

func NewTripsRepository(pool *pgxpool.Pool) *TripsRepository {
	return &TripsRepository{pool: pool}
}

func (r *TripsRepository) Create(ctx context.Context, t *Trip) error {
	exec := txmanager.ExecutorFromContext(ctx, r.pool)

	insertTrip := sq.Insert("trips").
		Columns(
			"id", "user_id", "driver_id",
			"start_latitude", "start_longitude", "end_latitude", "end_longitude",
			"price", "status", "started_at",
		).
		Values(
			t.ID, t.UserID, t.DriverID,
			t.StartLatitude, t.StartLongitude, t.EndLatitude, t.EndLongitude,
			t.Price, t.Status, t.StartedAt,
		).
		PlaceholderFormat(sq.Dollar)

	sql, args, err := insertTrip.ToSql()
	if err != nil {
		return fmt.Errorf("build insert trip: %w", err)
	}

	if _, err := exec.Exec(ctx, sql, args...); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDriverBusy
		}
		return fmt.Errorf("insert trip: %w", err)
	}

	insertHistory := sq.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason").
		Values(t.ID, nil, t.Status, "trip created").
		PlaceholderFormat(sq.Dollar)

	sql, args, err = insertHistory.ToSql()
	if err != nil {
		return fmt.Errorf("build insert history: %w", err)
	}

	if _, err := exec.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("insert trip history: %w", err)
	}

	return nil
}

func (r *TripsRepository) GetByID(ctx context.Context, id uuid.UUID) (*Trip, error) {
	exec := txmanager.ExecutorFromContext(ctx, r.pool)

	q := sq.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude", "end_latitude", "end_longitude",
		"price", "status", "started_at", "finished_at", "created_at", "updated_at",
	).
		From("trips").
		Where(sq.Eq{"id": id}).
		PlaceholderFormat(sq.Dollar)

	sql, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select trip: %w", err)
	}

	var t Trip
	err = exec.QueryRow(ctx, sql, args...).Scan(
		&t.ID, &t.UserID, &t.DriverID,
		&t.StartLatitude, &t.StartLongitude, &t.EndLatitude, &t.EndLongitude,
		&t.Price, &t.Status, &t.StartedAt, &t.FinishedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTripNotFound
		}
		return nil, fmt.Errorf("select trip: %w", err)
	}
	return &t, nil
}

func (r *TripsRepository) Finish(ctx context.Context, id uuid.UUID) (*Trip, error) {
	exec := txmanager.ExecutorFromContext(ctx, r.pool)

	update := sq.Update("trips").
		Set("status", "completed").
		Set("finished_at", sq.Expr("now()")).
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"id": id, "status": "active"}).
		PlaceholderFormat(sq.Dollar)

	sql, args, err := update.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build update trip: %w", err)
	}

	tag, err := exec.Exec(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("update trip: %w", err)
	}

	if tag.RowsAffected() == 0 {
		existing, err := r.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if existing.Status == "completed" {
			return nil, domain.ErrTripCompleted
		}
		return nil, domain.ErrTripNotFound
	}

	insertHistory := sq.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason").
		Values(id, "active", "completed", "trip finished").
		PlaceholderFormat(sq.Dollar)

	sql, args, err = insertHistory.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert history: %w", err)
	}
	if _, err := exec.Exec(ctx, sql, args...); err != nil {
		return nil, fmt.Errorf("insert trip history: %w", err)
	}

	return r.GetByID(ctx, id)
}
