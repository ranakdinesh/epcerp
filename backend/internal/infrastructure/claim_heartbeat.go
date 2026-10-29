package infrastructure

import (
	"context"
	"sync"
	"time"
)

// KeepClaim cancels parser work if its fenced lease can no longer be renewed.
func KeepClaim(ctx context.Context, renew func(context.Context) error) (context.Context, func()) {
	child, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			call, stop := context.WithTimeout(child, 5*time.Second)
			e := renew(call)
			stop()
			if e != nil {
				cancel()
				return
			}
			select {
			case <-child.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return child, func() { once.Do(cancel); <-done }
}
