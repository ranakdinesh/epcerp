package database

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
)

type fakeTx struct {
	pgx.Tx
	commits, rollbacks, writes int
	commitErr, rollbackErr     error
	cleanupCanceled            bool
	cleanupDeadline            bool
}

func (t *fakeTx) Commit(context.Context) error { t.commits++; return t.commitErr }
func (t *fakeTx) Rollback(ctx context.Context) error {
	t.rollbacks++
	t.cleanupCanceled = ctx.Err() != nil
	_, t.cleanupDeadline = ctx.Deadline()
	return t.rollbackErr
}
func (t *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	t.writes++
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

type fakePool struct {
	tx     *fakeTx
	err    error
	begins int
}

func (p *fakePool) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	p.begins++
	return p.tx, p.err
}

func TestTransactionOwnershipAndAtomicCallback(t *testing.T) {
	tx := &fakeTx{}
	pool := &fakePool{tx: tx}
	manager := NewManager(pool)
	err := manager.Within(context.Background(), func(ctx context.Context, conn Connection) error {
		if _, ok := conn.(interface{ Commit(context.Context) error }); ok {
			t.Fatal("callback can commit")
		}
		if _, ok := conn.(interface{ Rollback(context.Context) error }); ok {
			t.Fatal("callback can rollback")
		}
		if !errors.Is(manager.Within(ctx, func(context.Context, Connection) error { return nil }), ErrNestedTransaction) {
			t.Fatal("nested independent commit permitted")
		}
		_, err := conn.Exec(ctx, "business fixture")
		if err != nil {
			return err
		}
		_, err = conn.Exec(ctx, "audit fixture")
		return err
	})
	if err != nil || pool.begins != 1 || tx.commits != 1 || tx.rollbacks != 0 || tx.writes != 2 {
		t.Fatal("unit of work ownership failed")
	}
}
func TestRollbackOnErrorCancellationCommitFailureAndPanic(t *testing.T) {
	rejected := errors.New("business rejected")
	for _, mode := range []string{"rejected", "canceled", "commit-failed", "panic"} {
		t.Run(mode, func(t *testing.T) {
			tx := &fakeTx{}
			if mode == "commit-failed" {
				tx.commitErr = rejected
			}
			manager := NewManager(&fakePool{tx: tx})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			callback := func(context.Context, Connection) error {
				switch mode {
				case "rejected":
					return rejected
				case "canceled":
					cancel()
				case "panic":
					panic(rejected)
				}
				return nil
			}
			var err error
			func() {
				defer func() {
					value := recover()
					if mode == "panic" && value != rejected {
						t.Fatal("panic lost")
					}
					if mode != "panic" && value != nil {
						t.Fatal("unexpected panic")
					}
				}()
				err = manager.Within(ctx, callback)
			}()
			if mode == "rejected" || mode == "commit-failed" {
				if !errors.Is(err, rejected) {
					t.Fatal("original error lost")
				}
			}
			if mode == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatal("canceled request committed")
			}
			if tx.rollbacks != 1 || tx.cleanupCanceled || !tx.cleanupDeadline {
				t.Fatal("rollback cleanup was not independent and bounded")
			}
			if mode != "commit-failed" && tx.commits != 0 {
				t.Fatal("failed callback committed")
			}
		})
	}
}
func TestBeginFailureCanceledInputAndRollbackFailure(t *testing.T) {
	cause := errors.New("begin failed")
	pool := &fakePool{err: cause}
	called := false
	err := NewManager(pool).Within(context.Background(), func(context.Context, Connection) error { called = true; return nil })
	if !errors.Is(err, cause) || called {
		t.Fatal("begin failure ran callback")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(NewManager(pool).Within(ctx, func(context.Context, Connection) error { return nil }), context.Canceled) || pool.begins != 1 {
		t.Fatal("canceled input began a transaction")
	}
	rollback := errors.New("rollback failed")
	tx := &fakeTx{rollbackErr: rollback}
	err = NewManager(&fakePool{tx: tx}).Within(context.Background(), func(context.Context, Connection) error { return cause })
	if !errors.Is(err, cause) || !errors.Is(err, rollback) {
		t.Fatal("cleanup failure lost")
	}
}

func TestEventPublicationRequiresMatchingTransaction(t *testing.T) {
	pool := &fakePool{tx: &fakeTx{}}
	m := NewManager(pool)
	var held Connection
	var heldContext context.Context
	if IsTransactionConnection(context.Background(), pool.tx) {
		t.Fatal("raw transaction accepted")
	}
	if err := m.Within(context.Background(), func(ctx context.Context, c Connection) error {
		held, heldContext = c, ctx
		if !IsTransactionConnection(ctx, c) {
			t.Fatal("matching transaction rejected")
		}
		if IsTransactionConnection(context.Background(), c) {
			t.Fatal("unbound context accepted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	other := NewManager(&fakePool{tx: &fakeTx{}})
	if err := other.Within(context.Background(), func(ctx context.Context, c Connection) error {
		if IsTransactionConnection(ctx, held) || IsTransactionConnection(heldContext, c) {
			t.Fatal("different transaction accepted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
