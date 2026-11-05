package telemetry

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

// Providers содержит провайдеры трасс и метрик сервиса
type Providers struct {
	Tracer *trace.TracerProvider
	Meter  *metric.MeterProvider
}

// New создаёт провайдеры и экспортёры телеметрии
// Настройки экспортёров читаются из переменных окружения OpenTelemetry
func New(ctx context.Context) (*Providers, error) {
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			attribute.String("service.name", "trip-service"),
			attribute.String("service.namespace", "tripgo"),
			attribute.String("service.version", "dev"),
			attribute.String("deployment.environment", "local"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}

	spanExporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}

	metricReader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		if shutdownErr := spanExporter.Shutdown(ctx); shutdownErr != nil {
			return nil, errors.Join(
				fmt.Errorf("create metric reader: %w", err),
				fmt.Errorf("shutdown trace exporter: %w", shutdownErr),
			)
		}

		return nil, fmt.Errorf("create metric reader: %w", err)
	}

	tracer := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithBatcher(spanExporter),
	)

	meter := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metricReader),
	)

	return &Providers{
		Tracer: tracer,
		Meter:  meter,
	}, nil
}

// Shutdown отправляет оставшиеся данные и закрывает провайдеры
func (p *Providers) Shutdown(ctx context.Context) error {
	return errors.Join(
		p.Tracer.Shutdown(ctx),
		p.Meter.Shutdown(ctx),
	)
}
