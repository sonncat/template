package positions

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/sonncat/template/internal/model"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type blockingPush struct {
	mock    *MockpositionRequester
	started chan uuid.UUID
	release chan struct{}
}

func newBlockingPush(t *testing.T, capacity int) *blockingPush {
	t.Helper()

	ctrl := gomock.NewController(t)
	push := &blockingPush{
		mock:    NewMockpositionRequester(ctrl),
		started: make(chan uuid.UUID, capacity),
		release: make(chan struct{}),
	}

	push.mock.EXPECT().
		RequestPosition(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, tripID, _ uuid.UUID) error {
			push.started <- tripID

			select {
			case <-push.release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}).
		AnyTimes()

	return push
}

func TestScannerRunsOnTickAndStopsOnCancellation(t *testing.T) {
	called := make(chan struct{}, 1)
	ctrl := gomock.NewController(t)
	reader := NewMockstaleTripsReader(ctrl)

	reader.EXPECT().
		ListStaleTrips(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ time.Duration) ([]model.StaleTrip, error) {
			called <- struct{}{}
			return nil, nil
		}).
		AnyTimes()

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
	case <-called:
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
		newMockStaleReader(t, trips),
		time.Minute,
		len(trips),
		testLogger(),
	)

	scanner.Scan(context.Background())

	push := newBlockingPush(t, len(trips))
	defer close(push.release)

	pool := NewWorkerPool(context.Background(), scanner, push.mock, 10*time.Second, testLogger())
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
		newMockStaleReader(t, []model.StaleTrip{trip}),
		time.Minute,
		1,
		testLogger(),
	)
	scanner.Scan(context.Background())

	push := newBlockingPush(t, 1)
	defer close(push.release)

	pool := NewWorkerPool(context.Background(), scanner, push.mock, time.Minute, testLogger())
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
		newMockStaleReader(t, []model.StaleTrip{trip}),
		time.Minute,
		1,
		testLogger(),
	)
	scanner.Scan(context.Background())

	push := newBlockingPush(t, 1)
	defer close(push.release)

	pool := NewWorkerPool(context.Background(), scanner, push.mock, time.Minute, testLogger())
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

func TestScannerCancelsRunningQuery(t *testing.T) {
	started := make(chan struct{})
	ctrl := gomock.NewController(t)
	reader := NewMockstaleTripsReader(ctrl)

	reader.EXPECT().
		ListStaleTrips(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ time.Duration) ([]model.StaleTrip, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		}).
		Times(1)

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
	case <-started:
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

func TestPoolStopDrainsQueuedTasks(t *testing.T) {
	first := model.StaleTrip{
		TripID:   uuid.New(),
		DriverID: uuid.New(),
	}
	second := model.StaleTrip{
		TripID:   uuid.New(),
		DriverID: uuid.New(),
	}

	scanner := NewScanner(
		newMockStaleReader(t, []model.StaleTrip{first, second}),
		time.Minute,
		2,
		testLogger(),
	)
	scanner.Scan(context.Background())

	push := newBlockingPush(t, 2)
	defer close(push.release)

	pool := NewWorkerPool(context.Background(), scanner, push.mock, time.Minute, testLogger())
	pool.Start(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case got := <-push.started:
		if got != first.TripID {
			pool.Cancel()
			t.Fatalf("first task = %s, want %s", got, first.TripID)
		}
	case <-ctx.Done():
		pool.Cancel()
		t.Fatal("first task did not start")
	}

	stopped := make(chan error, 1)
	go func() {
		stopped <- pool.Stop(ctx)
	}()

	select {
	case push.release <- struct{}{}:
	case <-ctx.Done():
		pool.Cancel()
		t.Fatal("first task did not finish")
	}

	select {
	case got := <-push.started:
		if got != second.TripID {
			pool.Cancel()
			t.Fatalf("second task = %s, want %s", got, second.TripID)
		}
	case <-ctx.Done():
		pool.Cancel()
		t.Fatal("queued task did not start during shutdown")
	}

	select {
	case push.release <- struct{}{}:
	case <-ctx.Done():
		pool.Cancel()
		t.Fatal("second task did not finish")
	}

	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("stop pool: %v", err)
		}
	case <-ctx.Done():
		pool.Cancel()
		t.Fatal("pool did not stop after draining tasks")
	}
}

func TestScannerSetsQueryDeadline(t *testing.T) {
	deadlineSet := false
	ctrl := gomock.NewController(t)
	reader := NewMockstaleTripsReader(ctrl)

	reader.EXPECT().
		ListStaleTrips(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ time.Duration) ([]model.StaleTrip, error) {
			_, deadlineSet = ctx.Deadline()
			return nil, nil
		}).
		Times(1)

	scanner := NewScanner(reader, time.Minute, 1, testLogger())
	scanner.SetQueryTimeout(3 * time.Second)

	scanner.Scan(context.Background())

	if !deadlineSet {
		t.Fatal("scanner query has no deadline")
	}
}
