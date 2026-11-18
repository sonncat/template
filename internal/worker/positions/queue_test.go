package positions

import (
	"testing"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/model"
)

func TestQueueDeduplicationAndOverflow(t *testing.T) {
	q := newQueue(1)

	first := model.StaleTrip{TripID: uuid.New()}
	second := model.StaleTrip{TripID: uuid.New()}

	if got := q.enqueue(first); got != queued {
		t.Fatalf("first enqueue = %v, want queued", got)
	}

	if got := q.enqueue(first); got != duplicate {
		t.Errorf("duplicate enqueue = %v, want duplicate", got)
	}

	if got := q.enqueue(second); got != dropped {
		t.Errorf("enqueue into full queue = %v, want dropped", got)
	}

	item := <-q.channel
	if !q.isPending(item) {
		t.Fatal("queued task is not pending")
	}

	q.finish(item)

	if got := q.enqueue(first); got != queued {
		t.Errorf("enqueue after finish = %v, want queued", got)
	}
}

func TestQueueCancelDoesNotRemoveNewTask(t *testing.T) {
	q := newQueue(2)
	trip := model.StaleTrip{TripID: uuid.New()}

	if got := q.enqueue(trip); got != queued {
		t.Fatalf("first enqueue = %v, want queued", got)
	}

	oldTask := <-q.channel
	q.cancel(trip.TripID)

	if q.isPending(oldTask) {
		t.Fatal("cancelled task is still pending")
	}

	if got := q.enqueue(trip); got != queued {
		t.Fatalf("enqueue after cancel = %v, want queued", got)
	}

	newTask := <-q.channel

	// Завершение старой задачи не должно удалить новую
	q.finish(oldTask)

	if !q.isPending(newTask) {
		t.Fatal("old task removed the new task")
	}

	q.finish(newTask)

	if q.isPending(newTask) {
		t.Fatal("finished task is still pending")
	}
}
