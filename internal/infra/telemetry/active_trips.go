package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/metric"
)

// RegisterActiveTrips регистрирует метрику текущего числа активных поездок
func RegisterActiveTrips(
	meter metric.Meter,
	counter activeTripsCounter,
	timeout time.Duration,
	logger *slog.Logger,
) error {
	_, err := meter.Int64ObservableGauge(
		"tripgo.trips.active",
		metric.WithDescription("Number of active trips"),
		metric.WithInt64Callback(func(ctx context.Context, observer metric.Int64Observer) error {
			queryCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			count, err := counter.CountActive(queryCtx)
			if err != nil {
				logger.WarnContext(ctx, "cannot collect active trips metric", "error", err)
				return nil
			}

			observer.Observe(count)
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("create active trips metric: %w", err)
	}

	return nil
}
