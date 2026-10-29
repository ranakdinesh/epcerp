package application

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

// DraftPublisher atomically turns verified draft evidence into durable reading
// jobs. Each draft has its own transaction, so one revoked owner cannot starve others.
type DraftPublisher struct{ u ports.DraftUnit }

func NewDraftPublisher(u ports.DraftUnit) *DraftPublisher { return &DraftPublisher{u} }
func (s *DraftPublisher) Once(ctx context.Context) error {
	var pending []domain.DocumentDraft
	if e := s.u.Within(ctx, func(ctx context.Context, r ports.DraftRepository) error {
		var e error
		pending, e = r.Pending(ctx)
		return e
	}); e != nil {
		return e
	}
	var failures error
	for _, ref := range pending {
		e := s.u.Within(ctx, func(ctx context.Context, r ports.DraftRepository) error {
			if e := r.Authorize(ctx, ref.OwnerID, true); e != nil {
				return e
			}
			d, e := r.Get(ctx, ref.ID, ref.OwnerID)
			if e != nil {
				return e
			}
			return r.Publish(ctx, d)
		})
		failures = errors.Join(failures, e)
	}
	return failures
}
