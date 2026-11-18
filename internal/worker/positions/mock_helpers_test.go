package positions

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/sonncat/template/internal/model"
)

func newMockStaleReader(t *testing.T, trips []model.StaleTrip) *MockstaleTripsReader {
	t.Helper()

	ctrl := gomock.NewController(t)
	reader := NewMockstaleTripsReader(ctrl)

	reader.EXPECT().
		ListStaleTrips(gomock.Any(), gomock.Any()).
		Return(trips, nil).
		AnyTimes()

	return reader
}

type pushCallState struct {
	mu       sync.Mutex
	calls    []uuid.UUID
	failOnce bool
}

func newRecordingPush(t *testing.T, failOnce bool) (*MockpositionRequester, *pushCallState) {
	t.Helper()

	ctrl := gomock.NewController(t)
	push := NewMockpositionRequester(ctrl)
	state := &pushCallState{failOnce: failOnce}

	push.EXPECT().
		RequestPosition(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tripID, _ uuid.UUID) error {
			state.mu.Lock()
			defer state.mu.Unlock()

			state.calls = append(state.calls, tripID)

			if state.failOnce {
				state.failOnce = false
				return errors.New("push service unavailable")
			}

			return nil
		}).
		AnyTimes()

	return push, state
}
