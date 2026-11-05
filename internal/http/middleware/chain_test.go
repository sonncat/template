package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestPanicIsRecordedByMiddleware(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown meter provider: %v", err)
		}
	})

	router := chi.NewRouter()

	active, err := ActiveRequests(provider.Meter("test"), router)
	if err != nil {
		t.Fatalf("create active requests middleware: %v", err)
	}

	router.Use(Logging(logger))
	router.Use(active)
	router.Use(Recover(logger))

	router.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	handler := Recover(logger)(otelhttp.NewHandler(
		router,
		"HTTP request",
		otelhttp.WithMeterProvider(provider),
	))

	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if !strings.Contains(logs.String(), `"msg":"request handled"`) {
		t.Error("request log is missing")
	}
	if !strings.Contains(logs.String(), `"status":500`) {
		t.Error("request log does not contain status 500")
	}

	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}

	found := false

	for _, scope := range data.ScopeMetrics {
		for _, measured := range scope.Metrics {
			if measured.Name != "http.server.request.duration" {
				continue
			}

			histogram, ok := measured.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("unexpected duration metric type: %T", measured.Data)
			}

			for _, point := range histogram.DataPoints {
				status, exists := point.Attributes.Value("http.response.status_code")
				if exists && status.AsInt64() == http.StatusInternalServerError && point.Count > 0 {
					found = true
				}
			}
		}
	}

	if !found {
		t.Error("HTTP duration metric for status 500 was not recorded")
	}
}
