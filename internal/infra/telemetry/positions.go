package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// PositionMetrics хранит инструменты наблюдения за опросом координат
type PositionMetrics struct {
	saved    metric.Int64Counter
	checks   metric.Int64Counter
	requests metric.Int64Counter
	duration metric.Float64Histogram
}

// NewPositionMetrics создаёт метрики координат и регистрирует gauges
func NewPositionMetrics(
	meter metric.Meter,
	queueSize func() int64,
	activeWorkers func() int64,
) (*PositionMetrics, error) {
	saved, err := meter.Int64Counter("tripgo.positions.saved")
	if err != nil {
		return nil, fmt.Errorf("create positions saved counter: %w", err)
	}

	checks, err := meter.Int64Counter("tripgo.stale_checks")
	if err != nil {
		return nil, fmt.Errorf("create stale checks counter: %w", err)
	}

	requests, err := meter.Int64Counter("tripgo.position_requests")
	if err != nil {
		return nil, fmt.Errorf("create position requests counter: %w", err)
	}

	duration, err := meter.Float64Histogram(
		"tripgo.position_requests.duration",
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create position requests duration: %w", err)
	}

	queueGauge, err := meter.Int64ObservableGauge("tripgo.position_requests.queue.size")
	if err != nil {
		return nil, fmt.Errorf("create queue size gauge: %w", err)
	}

	workersGauge, err := meter.Int64ObservableGauge("tripgo.position_requests.workers.active")
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	_, err = meter.RegisterCallback(
		func(_ context.Context, observer metric.Observer) error {
			observer.ObserveInt64(queueGauge, queueSize())
			observer.ObserveInt64(workersGauge, activeWorkers())

			return nil
		},
		queueGauge,
		workersGauge,
	)
	if err != nil {
		return nil, fmt.Errorf("register position metrics callback: %w", err)
	}

	return &PositionMetrics{
		saved:    saved,
		checks:   checks,
		requests: requests,
		duration: duration,
	}, nil
}

// PositionSaved учитывает успешно сохранённую координату
func (m *PositionMetrics) PositionSaved(ctx context.Context) {
	m.saved.Add(ctx, 1)
}

// StaleCheck учитывает результат постановки задачи сканером
func (m *PositionMetrics) StaleCheck(ctx context.Context, result string) {
	m.checks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

// PositionRequest учитывает результат попытки запроса координаты
func (m *PositionMetrics) PositionRequest(
	ctx context.Context,
	result string,
	elapsed time.Duration,
) {
	attributes := metric.WithAttributes(attribute.String("result", result))

	m.requests.Add(ctx, 1, attributes)
	m.duration.Record(ctx, elapsed.Seconds(), attributes)
}

// PositionRequestDropped учитывает задачу, отброшенную из-за заполненной очереди
func (m *PositionMetrics) PositionRequestDropped(ctx context.Context) {
	m.requests.Add(
		ctx,
		1,
		metric.WithAttributes(attribute.String("result", "dropped")),
	)
}
