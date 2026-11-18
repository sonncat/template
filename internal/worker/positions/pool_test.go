package positions

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/model"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestScannerDeduplicatesAndDropsWhenFull(t *testing.T) {
	first := model.StaleTrip{TripID: uuid.New()}
	second := model.StaleTrip{TripID: uuid.New()}

	reader := newMockStaleReader(t, []model.StaleTrip{first, second})

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

	reader := newMockStaleReader(t, []model.StaleTrip{cancelled, first, second})

	scanner := NewScanner(reader, time.Minute, 3, testLogger())
	scanner.Scan(context.Background())
	scanner.Cancel(cancelled.TripID)

	push, pushState := newRecordingPush(t, true)
	pool := NewWorkerPool(context.Background(), scanner, push, time.Second, testLogger())

	// Один воркер позволяет проверить порядок обработки и то, что после ошибки он берёт следующую задачу
	pool.Start(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Stop(ctx); err != nil {
		t.Fatalf("stop pool: %v", err)
	}

	pushState.mu.Lock()
	defer pushState.mu.Unlock()

	if len(pushState.calls) != 2 {
		t.Fatalf("push calls = %d, want 2", len(pushState.calls))
	}

	if pushState.calls[0] != first.TripID || pushState.calls[1] != second.TripID {
		t.Errorf("push calls = %v, want [%s %s]", pushState.calls, first.TripID, second.TripID)
	}
}
