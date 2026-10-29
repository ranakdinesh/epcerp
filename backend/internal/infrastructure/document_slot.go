package infrastructure

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

const documentSlotKey int64 = 735841920001

func WithDocumentSlot(ctx context.Context, pool *pgxpool.Pool, work func(context.Context) (bool, error)) (bool, error) {
	c, e := pool.Acquire(ctx)
	if e != nil {
		return false, e
	}
	defer c.Release()
	var acquired bool
	if e = c.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", documentSlotKey).Scan(&acquired); e != nil || !acquired {
		return false, e
	}
	child, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-child.Done():
				return
			case <-t.C:
				ping, stop := context.WithTimeout(child, 5*time.Second)
				var n int
				err := c.QueryRow(ping, "SELECT 1").Scan(&n)
				stop()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()
	worked, err := work(child)
	close(done)
	<-stopped
	cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer stop()
	if _, e = c.Exec(cleanup, "SELECT pg_advisory_unlock($1)", documentSlotKey); e != nil {
		c.Conn().Close(cleanup)
	}
	return worked, err
}
