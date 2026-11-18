package positions

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/model"
)

type fakeStaleReader struct {
	trips []model.StaleTrip
}

func (f *fakeStaleReader) ListStaleTrips(
	_ context.Context,
	_ time.Duration,
) ([]model.StaleTrip, error) {
	return f.trips, nil
}

type fakePush struct {
	mu       sync.Mutex
	calls    []uuid.UUID
	failOnce bool
}

func (f *fakePush) RequestPosition(
	_ context.Context,
	tripID uuid.UUID,
	_ uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, tripID)

	if f.failOnce {
		f.failOnce = false
		return errors.New("push service unavailable")
	}

	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestScannerDeduplicatesAndDropsWhenFull(t *testing.T) {
	first := model.StaleTrip{TripID: uuid.New()}
	second := model.StaleTrip{TripID: uuid.New()}

	reader := &fakeStaleReader{
		trips: []model.StaleTrip{first, second},
	}

	scanner := NewScanner(reader, time.Minute, 1, testLogger())

	scanner.Scan(context.Background())
	scanner.Scan(context.Background())

	if got := len(scanner.queue.channel); got != 1 {
		t.Fatalf("queue length = %d, want 1", got)
	}

	item := <-scanner.queue.channel
	if item.trip.TripID != first.TripID {
		t.Errorf("queued trip = %s, want %s", item.trip.TripID, first.TripID)
	}
}

func TestPoolSkipsCancelledTaskAndContinuesAfterPushError(t *testing.T) {
	cancelled := model.StaleTrip{
		TripID:   uuid.New(),
		DriverID: uuid.New(),
	}
	first := model.StaleTrip{
		TripID:   uuid.New(),
		DriverID: uuid.New(),
	}
	second := model.StaleTrip{
		TripID:   uuid.New(),
		DriverID: uuid.New(),
	}

	reader := &fakeStaleReader{
		trips: []model.StaleTrip{cancelled, first, second},
	}

	scanner := NewScanner(reader, time.Minute, 3, testLogger())
	scanner.Scan(context.Background())
	scanner.Cancel(cancelled.TripID)

	push := &fakePush{failOnce: true}
	pool := NewWorkerPool(context.Background(), scanner, push, time.Second, testLogger())

	// Один воркер позволяет проверить порядок обработки и то, что после ошибки он берёт следующую задачу
	pool.Start(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Stop(ctx); err != nil {
		t.Fatalf("stop pool: %v", err)
	}

	push.mu.Lock()
	defer push.mu.Unlock()

	if len(push.calls) != 2 {
		t.Fatalf("push calls = %d, want 2", len(push.calls))
	}

	if push.calls[0] != first.TripID || push.calls[1] != second.TripID {
		t.Errorf("push calls = %v, want [%s %s]", push.calls, first.TripID, second.TripID)
	}
}
