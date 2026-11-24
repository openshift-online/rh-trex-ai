package db

import (
	"context"
	"log/slog"

	dbContext "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db/db_context"
)

// NewContext returns a new context with transaction stored in it.
// Upon error, the original context is still returned along with an error
func NewContext(ctx context.Context, connection SessionFactory) (context.Context, error) {
	tx, err := newTransaction(ctx, connection)
	if err != nil {
		return ctx, err
	}

	ctx = dbContext.WithTransaction(ctx, tx)

	return ctx, nil
}

// Resolve resolves the current transaction according to the rollback flag.
// It records the outcome in the context's TxResult, if any. See ResolveErr
// to also get the commit or rollback error.
func Resolve(ctx context.Context) {
	_ = ResolveErr(ctx)
}

// ResolveErr resolves the current transaction according to the rollback flag
// and returns the error of a failed commit or rollback. The outcome is also
// recorded in the context's TxResult, if any.
func ResolveErr(ctx context.Context) error {
	tx, ok := dbContext.Transaction(ctx)
	if !ok {
		slog.ErrorContext(ctx, "Could not retrieve transaction from context")
		return nil
	}

	if tx.MarkedForRollback() {
		if err := tx.Rollback(); err != nil {
			slog.ErrorContext(ctx, "Could not rollback transaction", "error", err)
			SetTxResult(ctx, TxRollbackFailed, err)
			return err
		}
		slog.InfoContext(ctx, "Rolled back transaction")
		SetTxResult(ctx, TxRolledBack, nil)
		return nil
	}
	if err := tx.Commit(); err != nil {
		// TransactionMiddleware and TransactionUnaryInterceptor resolve before
		// the response is sent, so the client receives an error, not success.
		slog.ErrorContext(ctx, "Could not commit transaction", "error", err)
		SetTxResult(ctx, TxCommitFailed, err)
		return err
	}
	SetTxResult(ctx, TxCommitted, nil)
	return nil
}

// MarkForRollback flags the transaction stored in the context for rollback and logs whatever error caused the rollback
func MarkForRollback(ctx context.Context, err error) {
	transaction, ok := dbContext.Transaction(ctx)
	if !ok {
		slog.ErrorContext(ctx, "Failed to mark transaction for rollback: could not retrieve transaction from context")
		return
	}
	transaction.SetRollbackFlag(true)
	slog.InfoContext(ctx, "Marked transaction for rollback", "error", err)
}
