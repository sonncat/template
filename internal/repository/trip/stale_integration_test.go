//go:build integration

package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sonncat/template/internal/repository/trip"
)

func TestListStaleTrips(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required for integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	t.Cleanup(pool.Close)

	repo := repository.NewTripsRepository(pool)

	oldestID := uuid.New()
	staleID := uuid.New()
	freshID := uuid.New()
	completedID := uuid.New()

	ids := []uuid.UUID{oldestID, staleID, freshID, completedID}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cleanupCancel()

		if _, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM trips WHERE id = ANY($1)",
			ids,
		); err != nil {
			t.Errorf("cleanup trips: %v", err)
		}
	})

	now := time.Now().UTC()

	insertTrip := func(
		id uuid.UUID,
		status string,
		startedAt time.Time,
		lastPositionAt *time.Time,
		finishedAt *time.Time,
	) {
		t.Helper()

		_, err := pool.Exec(ctx, `
			INSERT INTO trips (
				id, user_id, driver_id,
				start_latitude, start_longitude,
				end_latitude, end_longitude,
				price, status, started_at,
				last_position_at, finished_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`,
			id, uuid.New(), uuid.New(),
			59.93, 30.33,
			59.92, 30.32,
			1000, status, startedAt,
			lastPositionAt, finishedAt,
		)
		if err != nil {
			t.Fatalf("insert trip %v: %v", id, err)
		}
	}

	// Самая старая поездка ещё не получала координат.
	insertTrip(oldestID, "active", now.Add(-10*time.Minute), nil, nil)

	// У этой поездки есть координата, но она устарела.
	staleTime := now.Add(-5 * time.Minute)
	insertTrip(staleID, "active", now.Add(-10*time.Minute), &staleTime, nil)

	// Поездка старая, но координата свежая.
	freshTime := now
	insertTrip(freshID, "active", now.Add(-10*time.Minute), &freshTime, nil)

	// У завершённой поездки тоже может быть старая координата.
	finishedTime := now.Add(-time.Minute)
	insertTrip(completedID, "completed", now.Add(-10*time.Minute), &staleTime, &finishedTime)

	got, err := repo.ListStaleTrips(ctx, time.Minute)
	if err != nil {
		t.Fatalf("list stale trips: %v", err)
	}

	// В общей учебной базе могут быть чужие поездки, поэтому проверяем
	// только идентификаторы, созданные этим тестом.
	ownTrips := make([]uuid.UUID, 0, len(ids))

	for _, trip := range got {
		for _, id := range ids {
			if trip.TripID == id {
				ownTrips = append(ownTrips, id)
			}
		}
	}

	if len(ownTrips) != 2 {
		t.Fatalf("selected test trips = %v, want two stale active trips", ownTrips)
	}

	if ownTrips[0] != oldestID || ownTrips[1] != staleID {
		t.Errorf(
			"selected order = %v, want [%v %v]",
			ownTrips,
			oldestID,
			staleID,
		)
	}
}
