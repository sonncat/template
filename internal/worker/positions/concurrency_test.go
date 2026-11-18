package positions

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/goleak"

	"github.com/sonncat/template/internal/model"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type notifyingReader struct {
	called chan struct{}
}

func (r *notifyingReader) ListStaleTrips(
	_ context.Context,
	_ time.Duration,
) ([]model.StaleTrip, error) {
	r.called <- struct{}{}

	return nil, nil
}

type blockingPush struct {
	started chan uuid.UUID
	release chan struct{}
}

func (p *blockingPush) RequestPosition(
	ctx context.Context,
	tripID uuid.UUID,
	_ uuid.UUID,
) error {
	p.started <- tripID

	select {
	case <-p.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestScannerRunsOnTickAndStopsOnCancellation(t *testing.T) {
	reader := &notifyingReader{
		called: make(chan struct{}, 1),
	}

	scanner := NewScanner(reader, time.Minute, 2, testLogger())
	ticks := make(chan time.Time)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		scanner.Run(ctx, ticks)
	}()

	ticks <- time.Now()

	select {
	case <-reader.called:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("scanner did not process tick")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scanner did not stop")
	}
}

func TestPoolRunsConfiguredNumberOfWorkers(t *testing.T) {
	trips := []model.StaleTrip{
		{TripID: uuid.New(), DriverID: uuid.New()},
		{TripID: uuid.New(), DriverID: uuid.New()},
		{TripID: uuid.New(), DriverID: uuid.New()},
	}

	scanner := NewScanner(
		&fakeStaleReader{trips: trips},
		time.Minute,
		len(trips),
		testLogger(),
	)

	scanner.Scan(context.Background())

	push := &blockingPush{
		started: make(chan uuid.UUID, len(trips)),
		release: make(chan struct{}),
	}
	defer close(push.release)

	pool := NewWorkerPool(scanner, push, 10*time.Second, testLogger())
	pool.Start(2)

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Два запроса одновременно ждут разрешения завершиться
	// Значит, работают два независимых воркера
	for range 2 {
		select {
		case <-push.started:
		case <-stopCtx.Done():
			pool.cancel()
			t.Fatal("two workers did not start requests")
		}
	}

	// Разрешаем завершить первые задачи и дождаться третьей
	push.release <- struct{}{}

	select {
	case <-push.started:
	case <-stopCtx.Done():
		pool.cancel()
		t.Fatal("third task was not started")
	}

	// Освобождаем два оставшихся запроса
	push.release <- struct{}{}
	push.release <- struct{}{}

	if err := pool.Stop(stopCtx); err != nil {
		t.Fatalf("stop pool: %v", err)
	}
}

func TestPoolCancelsPushWhenShutdownDeadlinePassed(t *testing.T) {
	trip := model.StaleTrip{
		TripID:   uuid.New(),
		DriverID: uuid.New(),
	}

	scanner := NewScanner(
		&fakeStaleReader{trips: []model.StaleTrip{trip}},
		time.Minute,
		1,
		testLogger(),
	)
	scanner.Scan(context.Background())

	push := &blockingPush{
		started: make(chan uuid.UUID, 1),
		release: make(chan struct{}),
	}
	defer close(push.release)

	pool := NewWorkerPool(scanner, push, time.Minute, testLogger())
	pool.Start(1)

	select {
	case <-push.started:
	case <-time.After(5 * time.Second):
		pool.cancel()
		t.Fatal("push request did not start")
	}

	// Имитируем уже истёкший общий срок остановки
	shutdownCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := pool.Stop(shutdownCtx); err == nil {
		t.Fatal("Stop returned nil after shutdown deadline")
	}

	// Вызов Push Service должен получить отмену и воркер должен выйти
	select {
	case <-pool.done:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
}

func TestPoolCancelStopsWorkers(t *testing.T) {
	trip := model.StaleTrip{
		TripID:   uuid.New(),
		DriverID: uuid.New(),
	}

	scanner := NewScanner(
		&fakeStaleReader{trips: []model.StaleTrip{trip}},
		time.Minute,
		1,
		testLogger(),
	)
	scanner.Scan(context.Background())

	push := &blockingPush{
		started: make(chan uuid.UUID, 1),
		release: make(chan struct{}),
	}
	defer close(push.release)

	pool := NewWorkerPool(scanner, push, time.Minute, testLogger())
	pool.Start(1)

	select {
	case <-push.started:
	case <-time.After(5 * time.Second):
		pool.Cancel()
		t.Fatal("push request did not start")
	}

	pool.Cancel()

	select {
	case <-pool.done:
	case <-time.After(5 * time.Second):
		t.Fatal("workers did not stop after cancellation")
	}
}

type cancellationReader struct {
	started chan struct{}
}

func (r *cancellationReader) ListStaleTrips(
	ctx context.Context,
	_ time.Duration,
) ([]model.StaleTrip, error) {
	close(r.started)
	<-ctx.Done()

	return nil, ctx.Err()
}

func TestScannerCancelsRunningQuery(t *testing.T) {
	reader := &cancellationReader{
		started: make(chan struct{}),
	}

	scanner := NewScanner(reader, time.Minute, 1, testLogger())
	ticks := make(chan time.Time, 1)
	ticks <- time.Now()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})

	go func() {
		defer close(done)
		scanner.Run(ctx, ticks)
	}()

	select {
	case <-reader.started:
	case <-time.After(5 * time.Second):
		t.Fatal("scanner query did not start")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scanner did not stop after query cancellation")
	}
}
