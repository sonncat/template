package get

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonncat/template/internal/domain"
)

func TestHandleTripNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	usecase := NewMocktripUseCase(ctrl)

	id := uuid.New()
	usecase.EXPECT().
		Execute(gomock.Any(), id).
		Return(nil, domain.ErrTripNotFound)

	handler := New(usecase, time.Second)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+id.String(), nil)
	response := httptest.NewRecorder()

	handler.Handle(response, request, id)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":"trip_not_found"`) {
		t.Errorf("response = %s, want trip_not_found", response.Body.String())
	}
}
