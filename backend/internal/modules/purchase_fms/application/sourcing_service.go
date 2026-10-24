package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type SourcingService struct {
	u ports.SourcingUnitOfWork
	a ports.SourcingAuthorizer
	v ports.SourcingValues
}

func NewSourcing(u ports.SourcingUnitOfWork, a ports.SourcingAuthorizer, v ports.SourcingValues) *SourcingService {
	return &SourcingService{u, a, v}
}
func (s *SourcingService) actor(ctx context.Context, token, p, id string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil {
		return a, e
	}
	if !a.Active || a.ID == "" {
		return a, domain.SourcingUnauthorized
	}
	for _, v := range []string{a.ID, p, id} {
		if e = domain.SourcingID(v); e != nil {
			return a, e
		}
	}
	return a, nil
}
func (s *SourcingService) Workspace(ctx context.Context, token, p, id string) (out domain.SourcingWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.SourcingRepository, c ports.SourcingContext) error {
		out, e = s.workspace(ctx, r, c, a, p, id)
		return e
	})
	if e != nil {
		out = domain.SourcingWorkspace{}
	}
	return
}
func (s *SourcingService) workspace(ctx context.Context, r ports.SourcingRepository, c ports.SourcingContext, a domain.Principal, p, id string) (out domain.SourcingWorkspace, e error) {
	caps, e := c.Access(ctx, a, p, id)
	if e != nil {
		return out, e
	}
	if !caps.Read {
		return out, domain.SourcingForbidden
	}
	out.Source, e = c.Source(ctx, p, id)
	if e == nil && out.Source.PackageID != id {
		e = domain.SourcingInvalid("source", "Source does not belong to this package")
	}
	if e != nil {
		return out, e
	}
	record, e := r.Get(ctx, p, id)
	if e != nil {
		return out, e
	}
	if record != nil && !record.Active {
		return out, domain.SourcingMissing
	}
	if record != nil {
		copy := domain.SourcingCopy(*record)
		out.Case = &copy
		out.Comparison, e = domain.SourcingComparisons(copy, domain.SourcingToday(s.v.Now()))
		if e != nil {
			return out, e
		}
	} else {
		out.Comparison = []domain.SourcingComparison{}
	}
	out.Actions = []string{}
	out.Approvers = []domain.SourcingMember{}
	out.Suppliers = []domain.SourcingSupplier{}
	out.History = []domain.SourcingEvent{}
	out.Evidence = []domain.SourcingApprovedEvidence{}
	out.TaskOnly = !caps.Scoped
	if !out.TaskOnly && record != nil {
		out.History, e = r.History(ctx, p, id)
		if e != nil {
			return out, e
		}
	}
	if out.Source.Guard() != nil {
		if record != nil && (record.State == "pending" || record.State == "approved") && domain.SourcingAccessGuard(a, caps, record, "reopen") == nil {
			out.Actions = append(out.Actions, "reopen")
		}
		return out, nil
	}
	if caps.Configure || caps.Approve || caps.Work[4] || caps.Work[5] {
		out.Evidence, e = c.Evidence(ctx, p, id)
		if e != nil {
			return out, e
		}
	}
	out.TaskOnly = !caps.Scoped
	members, e := c.Approvers(ctx, p)
	if e != nil {
		return out, e
	}
	if caps.Configure {
		out.Approvers = members
	}
	if caps.Write && (caps.Configure || caps.Work[3] || caps.Work[4] || caps.Work[5]) {
		out.Suppliers, e = c.Suppliers(ctx, p, id)
		if e != nil {
			return out, e
		}
	}
	for _, action := range []string{"configure", "rfq", "dispatch", "no_bid", "quote", "propose", "decide", "reopen", "release"} {
		if domain.SourcingAccessGuard(a, caps, record, action) != nil {
			continue
		}
		if record == nil {
			if action == "configure" {
				out.Actions = append(out.Actions, action)
			}
			continue
		}
		state := record.State
		if record.AwardReference != "" && action != "release" {
			continue
		}
		valid := state == "draft" && (action == "configure" || action == "rfq" || action == "dispatch" || action == "no_bid" || action == "quote" || action == "propose") || state == "pending" && (action == "decide" || action == "reopen") || state == "approved" && (action == "release" || action == "reopen")
		if action == "decide" {
			found := false
			for _, m := range members {
				found = found || m.ID == a.ID
			}
			valid = valid && found
		}
		if valid {
			out.Actions = append(out.Actions, action)
		}
	}
	return out, nil
}
func (s *SourcingService) Execute(ctx context.Context, token, p, id string, q domain.SourcingCommand) (out domain.SourcingWorkspace, e error) {
	a, e := s.actor(ctx, token, p, id)
	if e != nil {
		return out, e
	}
	q, e = q.Validate()
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.SourcingRepository, c ports.SourcingContext) error {
		caps, err := c.Access(ctx, a, p, id)
		if err != nil {
			return err
		}
		if !caps.Read {
			return domain.SourcingForbidden
		}
		before, err := r.Get(ctx, p, id)
		if err != nil {
			return err
		}
		if before != nil && before.AwardReference != "" && q.Action != "release" {
			return domain.SourcingInvalid("action", "Award carries a completed Finance decision; revise the source quotation through its owning workflow")
		}
		if err = domain.SourcingAccessGuard(a, caps, before, q.Action); err != nil {
			return err
		}
		version := int32(0)
		if before != nil {
			version = before.Version
		}
		if q.Version != version {
			return domain.SourcingStale
		}

		src, err := c.Source(ctx, p, id)
		if err == nil && src.PackageID != id {
			err = domain.SourcingInvalid("source", "Source does not belong to this package")
		}
		if err != nil {
			return err
		}
		var suppliers []domain.SourcingSupplier
		var members []domain.SourcingMember
		if q.Action != "reopen" {
			suppliers, err = c.Suppliers(ctx, p, id)
			if err != nil {
				return err
			}
			members, err = c.Approvers(ctx, p)
			if err != nil {
				return err
			}
		}
		var proof *domain.SourcingProof
		if q.Action == "propose" || q.Action == "decide" || q.Action == "release" {
			if before == nil {
				return domain.SourcingMissing
			}
			candidate := domain.SourcingCopy(*before)
			if q.Action == "propose" {
				candidate.SelectedOfferID = q.OfferID
			}
			offer, err := domain.SourcingSelected(candidate, domain.SourcingToday(s.v.Now()))
			if err != nil {
				return err
			}
			proof, err = c.VerifySelection(ctx, p, id, *offer)
			if err != nil {
				return err
			}
		}
		next, err := domain.SourcingApply(before, a, caps, q, src, suppliers, members, proof, s.v.Now(), s.v.ID(), s.v.ID(), p)
		if err != nil {
			return err
		}
		var allocation domain.SourcingAllocation
		if next.State != "draft" {
			if proof == nil {
				return domain.SourcingInvalid("financial_approval", "Current approval evidence is required for reservation")
			}
			balances, err := r.LockAllocation(ctx, p, next.Source.DemandID, next.ID, proof.Reference+":"+proof.Fingerprint)
			if err != nil {
				return err
			}
			offer, err := domain.SourcingSelected(next, domain.SourcingToday(s.v.Now()))
			if err != nil {
				return err
			}
			amount, err := domain.SourcingTotal(*offer, next.Source.Quantity)
			if err != nil {
				return err
			}
			allocation, err = domain.SourcingPlanAllocation(next.Source.Quantity, proof.Quantity, proof.Total, balances.Quantity, balances.Amount, amount)
			if err != nil {
				return err
			}
		}
		saved, err := r.Save(ctx, next, a.ID)
		if err != nil {
			return err
		}
		if err = r.StoreAllocation(ctx, saved, proof, allocation); err != nil {
			return err
		}
		event := domain.SourcingEvent{ID: s.v.ID(), CaseID: saved.ID, Action: q.Action, Reason: q.Reason, Before: before, After: saved, At: saved.UpdatedAt, ActorID: a.ID}
		if err = r.Event(ctx, event); err != nil {
			return err
		}
		out, err = s.workspace(ctx, r, c, a, p, id)
		return err
	})
	if e != nil {
		out = domain.SourcingWorkspace{}
	}
	return
}
