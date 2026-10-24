package application

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/health/domain"
	"testing"
)

type probe struct {
	err   error
	ctx   context.Context
	calls int
}

func (p *probe) Check(ctx context.Context) error { p.ctx = ctx; p.calls++; return p.err }

func TestReadinessAndLiveness(t *testing.T) {
	for _, test := range []struct {
		err  error
		want domain.Status
	}{
		{nil, domain.OK}, {errors.New("private connection details"), domain.Unavailable},
	} {
		p := &probe{err: test.err}
		service := New(p)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if service.Live() != domain.OK || p.calls != 0 {
			t.Fatal("liveness touched database")
		}
		if got := service.Ready(ctx); got != test.want {
			t.Fatalf("got %s", got)
		}
		if p.ctx != ctx || p.calls != 1 {
			t.Fatal("probe lost request context")
		}
	}
}
