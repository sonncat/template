package create

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/sonncat/template/internal/domain"
)

func TestHandleDriverBusy(t *testing.T) {
	ctrl := gomock.NewController(t)
	usecase := NewMocktripUseCase(ctrl)

	usecase.EXPECT().
		Execute(gomock.Any(), gomock.Any()).
		Return(nil, domain.ErrDriverBusy)

	handler := New(usecase, time.Second)

	body := `{
		"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c",
		"driver_id":"7aa2363d-b025-4b6d-82af-f1c81c6e6bd0",
		"start_point":{"latitude":59.9398,"longitude":30.3146},
		"end_point":{"latitude":59.9290,"longitude":30.3626},
		"price":1450
	}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(body))
	response := httptest.NewRecorder()

	handler.Handle(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":"driver_busy"`) {
		t.Errorf("response = %s, want driver_busy", response.Body.String())
	}
}
