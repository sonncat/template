package domainerror

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/sonncat/template/internal/domain"
)

func TestProblemValidationError(t *testing.T) {
	err := fmt.Errorf("validate trip: %w", &domain.ValidationError{
		Field:   "price",
		Message: "must be greater than zero",
	})

	status, body := Problem(err)

	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
	if body.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", body.Code)
	}
	if body.Detail == nil || *body.Detail != "price: must be greater than zero" {
		t.Errorf("detail = %v, want field and validation message", body.Detail)
	}
}
