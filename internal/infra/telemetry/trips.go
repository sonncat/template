package telemetry

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/sonncat/template/internal/model"
)

type createTripUseCase interface {
	Execute(ctx context.Context, trip *model.Trip) (*model.Trip, error)
}

type finishTripUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}

// TripEvents содержит счётчики и логгер событий поездок
type TripEvents struct {
	created   metric.Int64Counter
	completed metric.Int64Counter
	logger    *slog.Logger
}

// NewTripEvents создаёт счётчики событий поездок
func NewTripEvents(meter metric.Meter, logger *slog.Logger) (*TripEvents, error) {
	created, err := meter.Int64Counter(
		"tripgo.trips.created",
		metric.WithDescription("Number of created trips"),
	)
	if err != nil {
		return nil, fmt.Errorf("create trips created counter: %w", err)
	}

	completed, err := meter.Int64Counter(
		"tripgo.trips.completed",
		metric.WithDescription("Number of completed trips"),
	)
	if err != nil {
		return nil, fmt.Errorf("create trips completed counter: %w", err)
	}

	return &TripEvents{
		created:   created,
		completed: completed,
		logger:    logger,
	}, nil
}

// ObserveCreate оборачивает usecase создания наблюдением за успешным результатом
func (e *TripEvents) ObserveCreate(next createTripUseCase) *ObservedCreate {
	return &ObservedCreate{next: next, events: e}
}

// ObserveFinish оборачивает usecase завершения наблюдением за успешным результатом
func (e *TripEvents) ObserveFinish(next finishTripUseCase) *ObservedFinish {
	return &ObservedFinish{next: next, events: e}
}

type ObservedCreate struct {
	next   createTripUseCase
	events *TripEvents
}

// Execute создаёт поездку и записывает событие после успешного результата
func (o *ObservedCreate) Execute(ctx context.Context, trip *model.Trip) (*model.Trip, error) {
	created, err := o.next.Execute(ctx, trip)
	if err != nil {
		return nil, err
	}

	o.events.created.Add(ctx, 1)
	o.events.logEvent(ctx, "trip created")

	return created, nil
}

type ObservedFinish struct {
	next   finishTripUseCase
	events *TripEvents
}

// Execute завершает поездку и записывает событие после успешного результата
func (o *ObservedFinish) Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	finished, err := o.next.Execute(ctx, id)
	if err != nil {
		return nil, err
	}

	o.events.completed.Add(ctx, 1)
	o.events.logEvent(ctx, "trip completed")

	return finished, nil
}

func (e *TripEvents) logEvent(ctx context.Context, message string) {
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
		e.logger.InfoContext(
			ctx,
			message,
			"trace_id", spanContext.TraceID().String(),
			"span_id", spanContext.SpanID().String(),
		)
		return
	}

	e.logger.InfoContext(ctx, message)
}
