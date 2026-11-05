package http

import (
	"encoding/json"
	"net/http"

	"github.com/sonncat/template/internal/domain"
	"github.com/sonncat/template/internal/http/domainerror"
)

// writeValidationError передаёт ошибку поля в общий переводчик ошибок
func writeValidationError(w http.ResponseWriter, r *http.Request, field, message string) {
	writeDomainError(w, r, &domain.ValidationError{
		Field:   field,
		Message: message,
	})
}

// writeDomainError переводит ошибку приложения в HTTP-ответ
func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	status, problem := domainerror.Problem(err)
	problem.Instance = &r.URL.Path

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}
