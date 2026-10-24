package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/event_delivery/domain"
)

type Journal interface {
	Append(context.Context, domain.Event, []domain.Target) error
}
type Queue interface {
	Claim(context.Context, string, string, int32) (*domain.Claim, error)
	Complete(context.Context, domain.Claim) error
	Fail(context.Context, domain.Claim, int32) error
	ExpireExhausted(context.Context, string) (int64, error)
}
type Consumer interface {
	Deliver(context.Context, domain.Event) error
}
type Values interface{ ID() string }
