// Transaction infrastructure is used by outbound unit-of-work adapters. Core
// application services declare their own driver-free capability ports.
package database

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"time"
)

// Connection provides sqlc DBTX query methods, deliberately excluding Commit,
// Rollback and Begin. All repositories and audit adapters receive this SAME value.
type Connection interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Beginner interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}
type Manager struct{ beginner Beginner }
type transactionKey struct{}

var ErrNestedTransaction = errors.New("nested independent transaction is not permitted")

func NewManager(beginner Beginner) *Manager { return &Manager{beginner: beginner} }

// Within commits only after work succeeds and the request is still live. Cleanup
// uses a separate bounded context so cancellation cannot strand an open transaction.
// There are no automatic retries: commit errors can have an ambiguous outcome.
func (m *Manager) Within(ctx context.Context, work func(context.Context, Connection) error) (err error) {
	if ctx.Value(transactionKey{}) != nil {
		return ErrNestedTransaction
	}
	if work == nil {
		return errors.New("transaction work is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	tx, err := m.beginner.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			rollbackErr := tx.Rollback(cleanup)
			if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				err = errors.Join(err, rollbackErr)
			}
		}
	}()
	txCtx := context.WithValue(ctx, transactionKey{}, queryConnection{tx: tx})
	if err = work(txCtx, queryConnection{tx: tx}); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

type queryConnection struct{ tx pgx.Tx }

func (q queryConnection) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return q.tx.Exec(ctx, sql, args...)
}
func (q queryConnection) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return q.tx.Query(ctx, sql, args...)
}
func (q queryConnection) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return q.tx.QueryRow(ctx, sql, args...)
}

// IsTransactionConnection prevents publication through a pool or a different unit of work.
func IsTransactionConnection(ctx context.Context, c Connection) bool {
	bound, ok := c.(queryConnection)
	active, live := ctx.Value(transactionKey{}).(queryConnection)
	return ok && live && bound.tx == active.tx
}
