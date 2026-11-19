package positions

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/sonncat/template/internal/model"
)

type recordedMetrics struct {
	mu       sync.Mutex
	checks   map[string]int
	requests map[string]int
	dropped  int
}

func (m *recordedMetrics) PositionSaved(_ context.Context) {}

func (m *recordedMetrics) StaleCheck(_ context.Context, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checks[result]++
}

func (m *recordedMetrics) PositionRequest(
	_ context.Context,
	result string,
	_ time.Duration,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requests[result]++
}

func (m *recordedMetrics) PositionRequestDropped(_ context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.dropped++
}

func TestScannerAndWorkerObservability(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown tracer provider: %v", err)
		}
	}()

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
		1,
		testLogger(),
	)

	metrics := &recordedMetrics{
		checks:   make(map[string]int),
		requests: make(map[string]int),
	}

	scanner.SetObservability(metrics, provider.Tracer("test"))

	// На первом проходе одна задача помещается в очередь, другая не помещается
	scanner.Scan(context.Background())

	// На втором проходе первая задача повторяется, а вторая снова не помещается
	scanner.Scan(context.Background())

	push, _ := newRecordingPush(t, false)
	pool := NewWorkerPool(context.Background(), scanner, push, time.Second, testLogger())
	pool.Start(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Stop(ctx); err != nil {
		t.Fatalf("stop pool: %v", err)
	}

	metrics.mu.Lock()

	if metrics.checks["queued"] != 1 {
		t.Errorf("queued = %d, want 1", metrics.checks["queued"])
	}
	if metrics.checks["duplicate"] != 1 {
		t.Errorf("duplicate = %d, want 1", metrics.checks["duplicate"])
	}
	if metrics.checks["dropped"] != 2 {
		t.Errorf("dropped checks = %d, want 2", metrics.checks["dropped"])
	}
	if metrics.dropped != 2 {
		t.Errorf("dropped requests = %d, want 2", metrics.dropped)
	}
	if metrics.requests["success"] != 1 {
		t.Errorf("successful requests = %d, want 1", metrics.requests["success"])
	}

	metrics.mu.Unlock()

	spans := exporter.GetSpans()

	var scanSpanID string
	var workerLinkID string
	var foundPush bool

	for _, span := range spans {
		switch span.Name {
		case "positions.scan":
			// Первый проход поставил задачу в очередь
			if scanSpanID == "" {
				scanSpanID = span.SpanContext.SpanID().String()
			}

		case "positions.process":
			if len(span.Links) == 1 {
				workerLinkID = span.Links[0].SpanContext.SpanID().String()
			}

		case "push.request_position":
			foundPush = true
		}
	}

	if scanSpanID == "" {
		t.Error("scan span was not recorded")
	}
	if workerLinkID != scanSpanID {
		t.Errorf("worker link = %s, want scan span %s", workerLinkID, scanSpanID)
	}
	if !foundPush {
		t.Error("push request span was not recorded")
	}
}
