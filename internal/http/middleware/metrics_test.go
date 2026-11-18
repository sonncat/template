package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestActiveRequests(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown meter provider: %v", err)
		}
	})

	router := chi.NewRouter()

	activeMiddleware, err := ActiveRequests(
		provider.Meter("test"),
		router,
	)
	if err != nil {
		t.Fatalf("create middleware: %v", err)
	}

	router.Use(activeMiddleware)

	router.Get("/trips/{tripID}", func(w http.ResponseWriter, r *http.Request) {
		value, route := readActiveRequests(t, reader)

		if value != 1 {
			t.Errorf("active requests during handler = %d, want 1", value)
		}
		if route != "/trips/{tripID}" {
			t.Errorf("route = %q, want /trips/{tripID}", route)
		}

		w.WriteHeader(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodGet, "/trips/123", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}

	value, _ := readActiveRequests(t, reader)
	if value != 0 {
		t.Errorf("active requests after handler = %d, want 0", value)
	}
}

func readActiveRequests(t *testing.T, reader *sdkmetric.ManualReader) (int64, string) {
	t.Helper()

	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}

	for _, scope := range data.ScopeMetrics {
		for _, measured := range scope.Metrics {
			if measured.Name != "http.server.active_requests" {
				continue
			}

			sum, ok := measured.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("unexpected metric type: %T", measured.Data)
			}
			if len(sum.DataPoints) != 1 {
				t.Fatalf("data points = %d, want 1", len(sum.DataPoints))
			}

			point := sum.DataPoints[0]
			routeValue, _ := point.Attributes.Value(attribute.Key("http.route"))
			return point.Value, routeValue.AsString()
		}
	}

	t.Fatal("active requests metric not found")
	return 0, ""
}
