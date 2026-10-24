package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/event_delivery/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/event_delivery/ports"
)

type Worker struct {
	queue    ports.Queue
	consumer ports.Consumer
	values   ports.Values
	name     string
	policy   domain.Policy
}

func NewWorker(q ports.Queue, c ports.Consumer, v ports.Values, name string, p domain.Policy) (*Worker, error) {
	if q == nil || c == nil || v == nil || !domain.ValidCode(name) {
		return nil, domain.ErrInvalid
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &Worker{q, c, v, name, p}, nil
}

// RunOne returns whether work was claimed, not whether delivery succeeded.
// Delivery is at least once; consumers must deduplicate effects using Event.ID.
// No acknowledgment is made on cancellation or panic; a lease allows recovery.
func (w *Worker) RunOne(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if _, err := w.queue.ExpireExhausted(ctx, w.name); err != nil {
		return false, err
	}
	token := w.values.ID()
	if !domain.ValidID(token) {
		return false, domain.ErrInvalid
	}
	claim, err := w.queue.Claim(ctx, w.name, token, w.policy.LeaseSeconds)
	if err != nil || claim == nil {
		return false, err
	}
	deliveryCtx, cancel := context.WithTimeout(ctx, w.policy.HandlerTimeout())
	defer cancel()
	err = w.consumer.Deliver(deliveryCtx, claim.Event)
	// A consumer that ignores its deadline must not be acknowledged as successful.
	if err == nil {
		err = deliveryCtx.Err()
	}
	if ctx.Err() != nil {
		return true, ctx.Err()
	}
	if err != nil {
		return true, w.queue.Fail(ctx, *claim, w.policy.RetryDelay(claim.Attempt))
	}
	return true, w.queue.Complete(ctx, *claim)
}
