package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}

	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}

	return w.ResponseWriter.Write(body)
}

// Unwrap позволяет стандартным HTTP-инструментам получить исходный writer
func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Logging записывает результат HTTP-запроса и уточняет имя его спана
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			writer := &responseWriter{ResponseWriter: w}

			next.ServeHTTP(writer, r)

			status := writer.status
			if status == 0 {
				status = http.StatusOK
			}

			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unknown"
			}

			span := trace.SpanFromContext(r.Context())
			if span.IsRecording() {
				span.SetName(r.Method + " " + route)
				span.SetAttributes(attribute.String("http.route", route))
			}

			attrs := []any{
				"method", r.Method,
				"route", route,
				"status", status,
				"duration_ms", time.Since(started).Milliseconds(),
			}

			spanContext := span.SpanContext()
			if spanContext.IsValid() {
				attrs = append(attrs,
					"trace_id", spanContext.TraceID().String(),
					"span_id", spanContext.SpanID().String(),
				)
			}

			if status >= http.StatusInternalServerError {
				logger.ErrorContext(r.Context(), "request handled", attrs...)
			} else {
				logger.InfoContext(r.Context(), "request handled", attrs...)
			}
		})
	}
}
