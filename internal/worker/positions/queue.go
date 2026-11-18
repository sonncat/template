package positions

import (
	"sync"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/model"
)

type enqueueResult int

const (
	queued enqueueResult = iota
	duplicate
	dropped
)

type task struct {
	trip model.StaleTrip
}

type queue struct {
	mu      sync.Mutex
	channel chan *task
	pending map[uuid.UUID]*task
}

func newQueue(size int) *queue {
	return &queue{
		channel: make(chan *task, size),
		pending: make(map[uuid.UUID]*task),
	}
}

// enqueue не блокируется, если очередь заполнена
func (q *queue) enqueue(trip model.StaleTrip) enqueueResult {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.pending[trip.TripID]; exists {
		return duplicate
	}

	item := &task{trip: trip}
	q.pending[trip.TripID] = item

	select {
	case q.channel <- item:
		return queued
	default:
		delete(q.pending, trip.TripID)
		return dropped
	}
}

// cancel помечает задачу отменённой. Если она уже лежит в канале, воркер пропустит её при получении
func (q *queue) cancel(tripID uuid.UUID) {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.pending, tripID)
}

// isPending проверяет именно эту задачу, а не только ID поездки
// Это важно, если старую задачу отменили и затем поставили новую
func (q *queue) isPending(item *task) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.pending[item.trip.TripID] == item
}

// finish удаляет задачу после обработки, не затрагивая более новую
// задачу той же поездки
func (q *queue) finish(item *task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.pending[item.trip.TripID] == item {
		delete(q.pending, item.trip.TripID)
	}
}
