package finish

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

func TestHandleTripCompleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	usecase := NewMocktripUseCase(ctrl)

	id := uuid.New()
	usecase.EXPECT().
		Execute(gomock.Any(), id).
		Return(nil, domain.ErrTripCompleted)

	handler := New(usecase, time.Second)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id.String()+"/finish", nil)
	response := httptest.NewRecorder()

	handler.Handle(response, request, id)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":"trip_completed"`) {
		t.Errorf("response = %s, want trip_completed", response.Body.String())
	}
}
