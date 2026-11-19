package container

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestCloseStopsComponentsInOrder(t *testing.T) {
	ctrl := gomock.NewController(t)

	workers := NewMockpositionWorkers(ctrl)
	push := NewMockpushCloser(ctrl)
	telemetry := NewMocktelemetryProviders(ctrl)
	pool := NewMockdatabasePool(ctrl)

	scannerDone := make(chan struct{})
	scannerCancelled := false

	app := &Container{
		scannerCancel: func() {
			scannerCancelled = true
			close(scannerDone)
		},
		scannerDone: scannerDone,
		workers:     workers,
		pushClient:  push,
		telemetry:   telemetry,
		pool:        pool,
	}

	gomock.InOrder(
		workers.EXPECT().Stop(gomock.Any()).DoAndReturn(
			func(_ context.Context) error {
				if !scannerCancelled {
					t.Error("workers stopped before scanner cancellation")
				}
				return nil
			},
		),
		push.EXPECT().Close(),
		telemetry.EXPECT().Shutdown(gomock.Any()).Return(nil),
		pool.EXPECT().Close(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestCloseAttemptsCleanupAfterScannerTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)

	workers := NewMockpositionWorkers(ctrl)
	push := NewMockpushCloser(ctrl)
	telemetry := NewMocktelemetryProviders(ctrl)
	pool := NewMockdatabasePool(ctrl)

	scannerDone := make(chan struct{})
	poolClosed := make(chan struct{})

	app := &Container{
		scannerCancel: func() {},
		scannerDone:   scannerDone,
		workers:       workers,
		pushClient:    push,
		telemetry:     telemetry,
		pool:          pool,
	}

	gomock.InOrder(
		workers.EXPECT().Cancel(),
		push.EXPECT().Close(),
		telemetry.EXPECT().Shutdown(gomock.Any()).Return(context.Canceled),
		pool.EXPECT().Close().Do(func() {
			close(poolClosed)
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.Close(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Close() error = %v, want context.Canceled", err)
	}

	select {
	case <-poolClosed:
	case <-time.After(5 * time.Second):
		t.Fatal("database pool close was not called")
	}
}

func TestCloseContinuesAfterWorkerError(t *testing.T) {
	ctrl := gomock.NewController(t)

	workers := NewMockpositionWorkers(ctrl)
	push := NewMockpushCloser(ctrl)
	telemetry := NewMocktelemetryProviders(ctrl)
	pool := NewMockdatabasePool(ctrl)

	scannerDone := make(chan struct{})
	close(scannerDone)

	workerErr := errors.New("worker shutdown failed")

	app := &Container{
		scannerCancel: func() {},
		scannerDone:   scannerDone,
		workers:       workers,
		pushClient:    push,
		telemetry:     telemetry,
		pool:          pool,
	}

	gomock.InOrder(
		workers.EXPECT().Stop(gomock.Any()).Return(workerErr),
		push.EXPECT().Close(),
		telemetry.EXPECT().Shutdown(gomock.Any()).Return(nil),
		pool.EXPECT().Close(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.Close(ctx); !errors.Is(err, workerErr) {
		t.Errorf("Close() error = %v, want worker error", err)
	}
}
