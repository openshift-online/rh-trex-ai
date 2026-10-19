package db

import (
	"context"

	"gorm.io/gorm"

	dbContext "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db/db_context"
)

// BindTransaction makes the session run its statements on the transaction stored
// in ctx by NewContext, so DAO writes commit or roll back with the request.
// When ctx holds no open transaction, the session is returned unchanged.
//
// The session must own its Statement, which is the case for one created with
// gorm.Session{Context: ctx}.
func BindTransaction(ctx context.Context, session *gorm.DB) *gorm.DB {
	tx, ok := dbContext.Transaction(ctx)
	if !ok || tx == nil || tx.Tx() == nil {
		return session
	}
	session.Statement.ConnPool = tx.Tx()
	return session
}

// HasTransaction reports whether ctx holds an open request transaction.
func HasTransaction(ctx context.Context) bool {
	tx, ok := dbContext.Transaction(ctx)
	return ok && tx != nil && tx.Tx() != nil
}
