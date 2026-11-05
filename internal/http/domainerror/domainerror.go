package domainerror

import (
	"errors"
	"net/http"

	"github.com/sonncat/template/api"
	"github.com/sonncat/template/internal/domain"
)

// Problem переводит ошибку приложения в HTTP-статус и тело ответа
func Problem(err error) (int, api.Problem) {
	status := http.StatusInternalServerError
	code := "internal_error"
	detail := "Internal server error"

	var validationErr *domain.ValidationError

	switch {
	case errors.As(err, &validationErr):
		status = http.StatusBadRequest
		code = "invalid_request"
		detail = validationErr.Error()
	case errors.Is(err, domain.ErrTripNotFound):
		status = http.StatusNotFound
		code = "trip_not_found"
		detail = "Trip was not found"
	case errors.Is(err, domain.ErrTripCompleted):
		status = http.StatusConflict
		code = "trip_completed"
		detail = "Operation is not allowed for a completed trip"
	case errors.Is(err, domain.ErrDriverBusy):
		status = http.StatusConflict
		code = "driver_busy"
		detail = "Driver already has an active trip"
	case errors.Is(err, domain.ErrInvalidRequest):
		status = http.StatusBadRequest
		code = "invalid_request"
		detail = "Invalid request"
	}

	return status, api.Problem{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: int32(status),
		Detail: &detail,
		Code:   code,
	}
}
