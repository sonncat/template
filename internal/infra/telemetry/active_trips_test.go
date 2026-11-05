package telemetry

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/mock/gomock"
)

func TestActiveTripsMetric(t *testing.T) {
	ctrl := gomock.NewController(t)
	counter := NewMockactiveTripsCounter(ctrl)

	counter.EXPECT().
		CountActive(gomock.Any()).
		Return(int64(3), nil)

	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))

	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown meter provider: %v", err)
		}
	})

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	err := RegisterActiveTrips(provider.Meter("test"), counter, time.Second, logger)
	if err != nil {
		t.Fatalf("register metric: %v", err)
	}

	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}

	for _, scope := range data.ScopeMetrics {
		for _, measured := range scope.Metrics {
			if measured.Name != "tripgo.trips.active" {
				continue
			}

			gauge, ok := measured.Data.(metricdata.Gauge[int64])
			if !ok {
				t.Fatalf("unexpected metric type: %T", measured.Data)
			}
			if len(gauge.DataPoints) != 1 {
				t.Fatalf("data points = %d, want 1", len(gauge.DataPoints))
			}
			if gauge.DataPoints[0].Value != 3 {
				t.Errorf("value = %d, want 3", gauge.DataPoints[0].Value)
			}
			return
		}
	}

	t.Fatal("active trips metric not found")
}
