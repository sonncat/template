package domainerror

import (
	"context"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/sonncat/template/api"
)

type loggerKey struct{}

// WithLogger добавляет логгер в контекст обработки HTTP-запроса
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// ProblemForRequest переводит ошибку и записывает причину внутренних ошибок
func ProblemForRequest(r *http.Request, err error) (int, api.Problem) {
	status, problem := Problem(err)
	if status < http.StatusInternalServerError {
		return status, problem
	}

	span := trace.SpanFromContext(r.Context())
	if span.IsRecording() {
		span.RecordError(err)
		span.SetStatus(codes.Error, "internal error")
		span.SetAttributes(attribute.String("error.type", "internal_error"))
	}

	logger, ok := r.Context().Value(loggerKey{}).(*slog.Logger)
	if ok && logger != nil {
		attrs := []any{"error", err}

		spanContext := span.SpanContext()
		if spanContext.IsValid() {
			attrs = append(attrs,
				"trace_id", spanContext.TraceID().String(),
				"span_id", spanContext.SpanID().String(),
			)
		}

		logger.ErrorContext(r.Context(), "request failed", attrs...)
	}

	return status, problem
}
