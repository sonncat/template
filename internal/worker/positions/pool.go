package positions

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

//go:generate go tool mockgen -source=pool.go -destination=pool_mocks_test.go -package=positions
type positionRequester interface {
	RequestPosition(ctx context.Context, tripID, driverID uuid.UUID) error
}

// WorkerPool обрабатывает задачи из очереди сканера
type WorkerPool struct {
	scanner *Scanner
	push    positionRequester
	timeout time.Duration
	logger  *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	active atomic.Int64
}

// NewWorkerPool создаёт пул. Воркеры запускаются отдельно вызовом Start
func NewWorkerPool(
	parentCtx context.Context,
	scanner *Scanner,
	push positionRequester,
	timeout time.Duration,
	logger *slog.Logger,
) *WorkerPool {
	ctx, cancel := context.WithCancel(parentCtx)

	return &WorkerPool{
		scanner: scanner,
		push:    push,
		timeout: timeout,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
		done:    make(chan struct{}),
	}
}

// Start запускает фиксированное число воркеров
// Вызывать один раз при запуске приложения
func (p *WorkerPool) Start(workers int) {
	p.logger.Info("position workers started", "workers", workers)

	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)

		go func() {
			defer wg.Done()
			p.runWorker()
		}()
	}

	go func() {
		wg.Wait()
		p.logger.Info("position workers stopped")
		close(p.done)
	}()
}

// ActiveWorkers возвращает число воркеров, занятых запросами
func (p *WorkerPool) ActiveWorkers() int64 {
	return p.active.Load()
}

func (p *WorkerPool) runWorker() {
	for {
		select {
		case <-p.ctx.Done():
			return

		case item, ok := <-p.scanner.queue.channel:
			if !ok {
				return
			}

			if !p.scanner.queue.claim(item) {
				continue
			}

			p.active.Add(1)

			ctx, cancel := context.WithTimeout(p.ctx, p.timeout)

			options := []trace.SpanStartOption{trace.WithNewRoot()}
			if item.span.IsValid() {
				options = append(options, trace.WithLinks(trace.Link{
					SpanContext: item.span,
				}))
			}

			ctx, workerSpan := p.scanner.tracer.Start(
				ctx,
				"positions.process",
				options...,
			)

			pushCtx, pushSpan := p.scanner.tracer.Start(
				ctx,
				"push.request_position",
				trace.WithSpanKind(trace.SpanKindClient),
			)

			started := time.Now()
			err := p.push.RequestPosition(
				pushCtx,
				item.trip.TripID,
				item.trip.DriverID,
			)
			elapsed := time.Since(started)

			if err != nil {
				pushSpan.RecordError(err)
			}

			pushSpan.End()

			if p.scanner.metrics != nil {
				result := "success"
				if err != nil {
					result = "error"
				}

				p.scanner.metrics.PositionRequest(ctx, result, elapsed)
			}

			workerSpan.End()
			cancel()
			p.scanner.queue.finish(item)
			p.active.Add(-1)

			if err != nil {
				p.logger.WarnContext(
					ctx,
					"request position",
					"trip_id", item.trip.TripID,
					"trace_id", workerSpan.SpanContext().TraceID().String(),
					"span_id", workerSpan.SpanContext().SpanID().String(),
					"error", err,
				)
			}
		}
	}
}

// Cancel отменяет обработку задач при принудительной остановке
func (p *WorkerPool) Cancel() {
	p.cancel()
}

// Stop закрывает очередь после остановки сканера и ждёт воркеров
func (p *WorkerPool) Stop(ctx context.Context) error {
	close(p.scanner.queue.channel)

	select {
	case <-p.done:
		p.cancel()
		return nil

	case <-ctx.Done():
		p.cancel()
		return ctx.Err()
	}
}
