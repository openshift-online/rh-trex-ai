package db

import (
	"context"
	"errors"
	"testing"
)

func TestTxResult(t *testing.T) {
	if TxResultFromContext(context.Background()) != nil {
		t.Fatal("no result without WithTxResult")
	}
	SetTxResult(context.Background(), TxCommitted, nil) // must not panic

	ctx, r := WithTxResult(context.Background())
	if r.Outcome != TxPending || TxResultFromContext(ctx) != r {
		t.Fatalf("unexpected fresh result: %+v", r)
	}
	err := errors.New("boom")
	SetTxResult(ctx, TxCommitFailed, err)
	if r.Outcome != TxCommitFailed || r.Err != err || r.Outcome.String() != "commit failed" {
		t.Fatalf("unexpected result: %+v %s", r, r.Outcome)
	}
}
