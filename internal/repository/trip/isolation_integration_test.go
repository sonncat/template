//go:build integration

package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sonncat/template/internal/infra/postgres/transaction"
)

func TestTransactionUsesReadCommitted(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required for integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	defer pool.Close()

	manager := txmanager.NewPgxTxManager(pool)
	var isolation string

	err = manager.Do(ctx, func(txCtx context.Context) error {
		exec := txmanager.ExecutorFromContext(txCtx, pool)
		return exec.QueryRow(txCtx, "SHOW transaction_isolation").Scan(&isolation)
	})
	if err != nil {
		t.Fatalf("read transaction isolation: %v", err)
	}

	if isolation != "read committed" {
		t.Errorf("isolation = %q, want read committed", isolation)
	}
}
