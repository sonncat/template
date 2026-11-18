package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover перехватывает панику, записывает её в лог и возвращает ошибку клиенту
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.ErrorContext(
						r.Context(),
						"request panic",
						"panic", recovered,
						"stack", string(debug.Stack()),
					)

					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(
						`{"type":"about:blank","title":"Internal Server Error","status":500,"code":"internal_error","detail":"Internal server error"}`,
					))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
