package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/ports"
)

func DeliverQuotation(ctx context.Context, r ports.Repository, v ports.Values, event, recipient, title, link string) error {
	source, e := r.Source(ctx, event)
	if e != nil {
		return e
	}
	if source == nil {
		return domain.SourceConflict
	}
	n, e := domain.AddressedQuotation(v.ID(), *source, recipient, title, link)
	if e != nil {
		return e
	}
	saved, e := r.Create(ctx, n)
	if e != nil {
		return e
	}
	// Idempotent source/recipient insertion retains its first delivered history.
	if saved.ID != n.ID {
		return nil
	}
	return r.Event(ctx, domain.Event{ID: v.ID(), Action: "delivered", After: saved, ActorID: source.ActorID, At: source.At})
}

func DeliverSupplierReview(ctx context.Context, r ports.Repository, v ports.Values, event, recipient, title, link string) error {
	source, e := r.Source(ctx, event)
	if e != nil {
		return e
	}
	if source == nil {
		return domain.SourceConflict
	}
	n, e := domain.AddressedSupplierReview(v.ID(), *source, recipient, title, link)
	if e != nil {
		return e
	}
	saved, e := r.Create(ctx, n)
	if e != nil {
		return e
	}
	// Idempotent source/recipient insertion retains its first delivered history.
	if saved.ID != n.ID {
		return nil
	}
	return r.Event(ctx, domain.Event{ID: v.ID(), Action: "delivered", After: saved, ActorID: source.ActorID, At: source.At})
}

func DeliverTender(ctx context.Context, r ports.Repository, v ports.Values, sources ports.TenderSources, action string) (string, error) {
	a, e := sources.Read(ctx, action)
	if e != nil {
		return "", e
	}
	source, e := r.Source(ctx, a.EventID)
	if e != nil {
		return "", e
	}
	if source == nil {
		return "", domain.SourceConflict
	}
	n, e := domain.AddressedTender(v.ID(), *source, a)
	if e != nil {
		return "", e
	}
	saved, e := r.Create(ctx, n)
	if e != nil {
		return "", e
	}
	if saved.ID == n.ID {
		if e = r.Event(ctx, domain.Event{ID: v.ID(), Action: "delivered", After: saved, ActorID: source.ActorID, At: source.At}); e != nil {
			return "", e
		}
	}
	return saved.ID, nil
}
