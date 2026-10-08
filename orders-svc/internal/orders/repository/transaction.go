package repository

import (
	"context"
	"time"
)

type transactionRollback interface {
	Rollback(context.Context) error
}

func rollbackTransaction(ctx context.Context, tx transactionRollback) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	tx.Rollback(cleanupCtx)
}
