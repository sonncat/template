package http

import (
	"encoding/json"
	"net/http"

	"github.com/sonncat/template/api"
	"github.com/sonncat/template/internal/http/domainerror"
)

// writeProblem отдаёт ошибку по RFC 9457 в application/problem+json
func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, detail string) {
	problem := api.Problem{
		Type:     "about:blank",
		Title:    http.StatusText(status),
		Status:   int32(status),
		Detail:   &detail,
		Instance: &r.URL.Path,
		Code:     code,
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}

// writeDomainError переводит ошибку приложения в HTTP-ответ
func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	status, problem := domainerror.Problem(err)
	problem.Instance = &r.URL.Path

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}
