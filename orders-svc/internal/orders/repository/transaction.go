package repository

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

type transactionRollback interface {
	Rollback(context.Context) error
}

func rollbackTransaction(ctx context.Context, tx transactionRollback) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := tx.Rollback(cleanupCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		log.Printf("WARN: transaction rollback failed: %v", err)
	}
}
