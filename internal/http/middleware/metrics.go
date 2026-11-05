package middleware

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ActiveRequests создаёт middleware для подсчёта обрабатываемых HTTP-запросов
func ActiveRequests(
	meter metric.Meter,
	router chi.Router,
) (func(http.Handler) http.Handler, error) {
	active, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of HTTP requests currently being handled"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active requests counter: %w", err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Отдельный контекст маршрутизации позволяет узнать шаблон
			// до вызова обработчика, когда запрос ещё активен
			routeContext := chi.NewRouteContext()
			route := "unknown"

			if router.Match(routeContext, r.Method, r.URL.Path) {
				if pattern := routeContext.RoutePattern(); pattern != "" {
					route = pattern
				}
			}

			attrs := metric.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", route),
			)

			active.Add(r.Context(), 1, attrs)
			defer active.Add(r.Context(), -1, attrs)

			next.ServeHTTP(w, r)
		})
	}, nil
}
