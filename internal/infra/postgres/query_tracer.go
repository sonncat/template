package db

import (
	"context"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type queryInfoKey struct{}

type queryInfo struct {
	started    time.Time
	operation  string
	collection string
}

// queryTracer дополняет otelpgx меткой таблицы и длительностью SQL-запроса
type queryTracer struct {
	*otelpgx.Tracer
	duration metric.Float64Histogram
}

func (t *queryTracer) TraceQueryStart(
	ctx context.Context,
	conn *pgx.Conn,
	data pgx.TraceQueryStartData,
) context.Context {
	ctx = t.Tracer.TraceQueryStart(ctx, conn, data)

	info := queryInfo{
		started:    time.Now(),
		operation:  otelpgx.SQLOperationName(ctx, data.SQL),
		collection: queryCollection(data.SQL),
	}

	if info.collection != "" {
		trace.SpanFromContext(ctx).SetAttributes(
			attribute.String("db.collection.name", info.collection),
		)
	}

	return context.WithValue(ctx, queryInfoKey{}, info)
}

func (t *queryTracer) TraceQueryEnd(
	ctx context.Context,
	conn *pgx.Conn,
	data pgx.TraceQueryEndData,
) {
	info, ok := ctx.Value(queryInfoKey{}).(queryInfo)
	if ok {
		attrs := make([]attribute.KeyValue, 0, 3)

		if info.operation != "" {
			attrs = append(attrs, attribute.String("db.operation.name", info.operation))
		}
		if info.collection != "" {
			attrs = append(attrs, attribute.String("db.collection.name", info.collection))
		}
		if data.Err != nil {
			attrs = append(attrs, attribute.String("error.type", "database_error"))

			span := trace.SpanFromContext(ctx)
			span.SetStatus(codes.Error, "database error")
			span.SetAttributes(attribute.String("error.type", "database_error"))
		}

		t.duration.Record(
			ctx,
			time.Since(info.started).Seconds(),
			metric.WithAttributes(attrs...),
		)
	}

	t.Tracer.TraceQueryEnd(ctx, conn, data)
}

// queryCollection возвращает только известные таблицы проекта
func queryCollection(sql string) string {
	words := strings.Fields(sql)

	for i, word := range words {
		keyword := strings.ToUpper(word)

		if keyword != "FROM" && keyword != "INTO" && keyword != "UPDATE" {
			continue
		}
		if i+1 >= len(words) {
			return ""
		}

		table := strings.ToLower(strings.Trim(words[i+1], `";(),`))

		switch table {
		case "trips", "trip_status_history":
			return table
		default:
			return ""
		}
	}

	return ""
}
