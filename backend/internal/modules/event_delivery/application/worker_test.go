package application

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/event_delivery/domain"
	"testing"
)

const testID = "00000000-0000-4000-8000-000000000001"

type values struct{}

func (values) ID() string { return testID }

type queue struct {
	claim                 *domain.Claim
	completed, failed     int
	delay                 int32
	claimErr, completeErr error
}

func (q *queue) Claim(context.Context, string, string, int32) (*domain.Claim, error) {
	return q.claim, q.claimErr
}
func (q *queue) Complete(context.Context, domain.Claim) error { q.completed++; return q.completeErr }
func (q *queue) Fail(_ context.Context, _ domain.Claim, delay int32) error {
	q.failed++
	q.delay = delay
	return nil
}
func (q *queue) ExpireExhausted(context.Context, string) (int64, error) { return 0, nil }

type consume func(context.Context, domain.Event) error

func (c consume) Deliver(ctx context.Context, e domain.Event) error { return c(ctx, e) }
func worker(t *testing.T, q *queue, c consume) *Worker {
	t.Helper()
	w, err := NewWorker(q, c, values{}, "inbox", domain.Policy{LeaseSeconds: 3, HandlerSeconds: 1, RetrySeconds: 2, MaxRetrySeconds: 8})
	if err != nil {
		t.Fatal(err)
	}
	return w
}
func TestWorkerOutcomes(t *testing.T) {
	q := &queue{claim: &domain.Claim{ID: testID, Attempt: 3, Event: domain.Event{ID: testID}}}
	w := worker(t, q, func(ctx context.Context, e domain.Event) error {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("missing handler deadline")
		}
		if e.ID != testID {
			t.Fatal("wrong event")
		}
		return nil
	})
	if ran, err := w.RunOne(context.Background()); !ran || err != nil || q.completed != 1 || q.failed != 0 {
		t.Fatal("success", err)
	}
	q = &queue{claim: &domain.Claim{Attempt: 3}}
	w = worker(t, q, func(context.Context, domain.Event) error { return errors.New("sensitive transport error") })
	if ran, err := w.RunOne(context.Background()); !ran || err != nil || q.completed != 0 || q.failed != 1 || q.delay != 8 {
		t.Fatal("failure scheduling", err)
	}
	q = &queue{}
	w = worker(t, q, func(context.Context, domain.Event) error { t.Fatal("empty queue delivered"); return nil })
	if ran, err := w.RunOne(context.Background()); ran || err != nil {
		t.Fatal("empty queue", err)
	}
}
func TestCancelledWorkerLeavesLeaseForRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	q := &queue{claim: &domain.Claim{Attempt: 1}}
	w := worker(t, q, func(context.Context, domain.Event) error { cancel(); return nil })
	if ran, err := w.RunOne(ctx); !ran || !errors.Is(err, context.Canceled) || q.completed != 0 || q.failed != 0 {
		t.Fatal("cancelled delivery acknowledged", err)
	}
}
func TestPanicLeavesLeaseForRecovery(t *testing.T) {
	q := &queue{claim: &domain.Claim{Attempt: 1}}
	w := worker(t, q, func(context.Context, domain.Event) error { panic("worker interrupted") })
	defer func() {
		if recover() == nil || q.completed != 0 || q.failed != 0 {
			t.Fatal("panic acknowledged")
		}
	}()
	_, _ = w.RunOne(context.Background())
}
func TestHandlerDeadline(t *testing.T) {
	q := &queue{claim: &domain.Claim{Attempt: 1}}
	w := worker(t, q, func(ctx context.Context, _ domain.Event) error { <-ctx.Done(); return nil })
	if ran, err := w.RunOne(context.Background()); !ran || err != nil || q.failed != 1 || q.completed != 0 {
		t.Fatal("late success acknowledged", err)
	}
}

type journal struct{ calls int }

func (j *journal) Append(context.Context, domain.Event, []domain.Target) error { j.calls++; return nil }
func TestRecorderRejectsInvalidEventBeforePersistence(t *testing.T) {
	j := &journal{}
	r := NewRecorder(j)
	if err := r.Record(context.Background(), domain.Event{}, nil); err != domain.ErrInvalid || j.calls != 0 {
		t.Fatal("invalid event recorded", err)
	}
}
