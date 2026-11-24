package db

import "context"

// TxOutcome is how a request transaction ended.
type TxOutcome int

const (
	// TxPending means the transaction has not been resolved yet.
	TxPending TxOutcome = iota
	// TxCommitted means the transaction committed.
	TxCommitted
	// TxRolledBack means the transaction was rolled back on purpose.
	TxRolledBack
	// TxCommitFailed means the commit failed, so nothing was persisted.
	TxCommitFailed
	// TxRollbackFailed means the rollback itself failed.
	TxRollbackFailed
)

func (o TxOutcome) String() string {
	switch o {
	case TxCommitted:
		return "committed"
	case TxRolledBack:
		return "rolled back"
	case TxCommitFailed:
		return "commit failed"
	case TxRollbackFailed:
		return "rollback failed"
	}
	return "pending"
}

// TxResult records how the request transaction ended. A caller that wraps
// the transaction middleware or interceptor (for example the audit hook)
// stores one with WithTxResult and reads it after the request completes.
// Resolve fills it in place.
type TxResult struct {
	Outcome TxOutcome
	Err     error
}

type txResultKey struct{}

// WithTxResult returns a context carrying a fresh TxResult.
func WithTxResult(ctx context.Context) (context.Context, *TxResult) {
	r := &TxResult{}
	return context.WithValue(ctx, txResultKey{}, r), r
}

// TxResultFromContext returns the TxResult stored by WithTxResult, or nil.
func TxResultFromContext(ctx context.Context) *TxResult {
	r, _ := ctx.Value(txResultKey{}).(*TxResult)
	return r
}

// SetTxResult records the outcome in the context's TxResult, if any.
// Resolve calls it; tests may call it to simulate an outcome.
func SetTxResult(ctx context.Context, outcome TxOutcome, err error) {
	if r := TxResultFromContext(ctx); r != nil {
		r.Outcome = outcome
		r.Err = err
	}
}
