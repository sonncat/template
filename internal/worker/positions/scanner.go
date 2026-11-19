package positions

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/sonncat/template/internal/model"
)

// staleTripsReader нужен сканеру для получения устаревших поездок
//
//go:generate go tool mockgen -source=scanner.go -destination=scanner_mocks_test.go -package=positions
type staleTripsReader interface {
	ListStaleTrips(ctx context.Context, staleAfter time.Duration) ([]model.StaleTrip, error)
}

// Scanner находит устаревшие поездки и ставит их в очередь
type positionMetrics interface {
	PositionSaved(ctx context.Context)
	StaleCheck(ctx context.Context, result string)
	PositionRequest(ctx context.Context, result string, elapsed time.Duration)
	PositionRequestDropped(ctx context.Context)
}

type Scanner struct {
	scanMu              sync.Mutex
	cancelledDuringScan map[uuid.UUID]struct{}
	metrics             positionMetrics
	tracer              trace.Tracer
	reader              staleTripsReader
	queue               *queue
	staleAfter          time.Duration
	queryTimeout        time.Duration
	logger              *slog.Logger
}

// NewScanner создаёт сканер и его ограниченную очередь
func NewScanner(
	reader staleTripsReader,
	staleAfter time.Duration,
	queueSize int,
	logger *slog.Logger,
) *Scanner {
	return &Scanner{
		reader:     reader,
		queue:      newQueue(queueSize),
		staleAfter: staleAfter,
		logger:     logger,
		tracer:     noop.NewTracerProvider().Tracer("positions"),
	}
}

// SetObservability подключает метрики и трассировку до запуска сканера
func (s *Scanner) SetObservability(metrics positionMetrics, tracer trace.Tracer) {
	s.metrics = metrics
	s.tracer = tracer
}

// SetQueryTimeout задаёт таймаут запроса устаревших поездок
func (s *Scanner) SetQueryTimeout(timeout time.Duration) {
	s.queryTimeout = timeout
}

// QueueSize возвращает количество задач в канале
func (s *Scanner) QueueSize() int64 {
	return int64(len(s.queue.channel))
}

// Scan выполняет одну проверку. Следующую проверку вызывающий код
// начинает только после завершения предыдущей
func (s *Scanner) Scan(ctx context.Context) {
	s.scanMu.Lock()
	s.cancelledDuringScan = make(map[uuid.UUID]struct{})
	s.scanMu.Unlock()

	defer func() {
		s.scanMu.Lock()
		s.cancelledDuringScan = nil
		s.scanMu.Unlock()
	}()

	ctx, span := s.tracer.Start(ctx, "positions.scan")
	defer span.End()

	queryCtx := ctx
	cancel := func() {}

	if s.queryTimeout > 0 {
		queryCtx, cancel = context.WithTimeout(ctx, s.queryTimeout)
	}

	trips, err := s.reader.ListStaleTrips(queryCtx, s.staleAfter)
	cancel()

	if err != nil {
		if ctx.Err() == nil {
			s.logger.Error("select stale trips", "error", err)
		}

		return
	}

	for _, trip := range trips {
		if ctx.Err() != nil {
			return
		}

		s.scanMu.Lock()

		if _, cancelled := s.cancelledDuringScan[trip.TripID]; cancelled {
			s.scanMu.Unlock()
			continue
		}

		result := s.queue.enqueueWithSpan(trip, span.SpanContext())
		s.scanMu.Unlock()

		switch result {
		case queued:
			if s.metrics != nil {
				s.metrics.StaleCheck(ctx, "queued")
			}
		case duplicate:
			if s.metrics != nil {
				s.metrics.StaleCheck(ctx, "duplicate")
			}
		case dropped:
			if s.metrics != nil {
				s.metrics.StaleCheck(ctx, "dropped")
				s.metrics.PositionRequestDropped(ctx)
			}

			s.logger.Warn("position request queue is full", "trip_id", trip.TripID)
		}
	}
}

// Run выполняет проверки по сигналам тикера до отмены контекста
// Канал передаётся извне, чтобы в тестах управлять тиками
func (s *Scanner) Run(ctx context.Context, ticks <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ticks:
			if !ok {
				return
			}

			s.Scan(ctx)
		}
	}
}

// Cancel помечает ожидающую задачу поездки отменённой
func (s *Scanner) Cancel(tripID uuid.UUID) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	if s.cancelledDuringScan != nil {
		s.cancelledDuringScan[tripID] = struct{}{}
	}

	s.queue.cancel(tripID)
}
