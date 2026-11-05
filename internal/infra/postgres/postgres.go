package db

import (
	"context"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"

	"github.com/sonncat/template/internal/config"
)

func NewPool(
	ctx context.Context,
	cfg config.Database,
	tracerProvider trace.TracerProvider,
	meterProvider metric.MeterProvider,
) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	duration, err := meterProvider.Meter("trip-service/postgres").Float64Histogram(
		"db.client.operation.duration",
		metric.WithDescription("Duration of database operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create database duration metric: %w", err)
	}

	// otelpgx создаёт спаны; свою метрику длительности мы записываем в
	// queryTracer, чтобы добавить таблицу и не дублировать измерения
	tracer := otelpgx.NewTracer(
		otelpgx.WithTracerProvider(tracerProvider),
		otelpgx.WithMeterProvider(noop.NewMeterProvider()),
	)

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	poolCfg.ConnConfig.Tracer = &queryTracer{
		Tracer:   tracer,
		duration: duration,
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
