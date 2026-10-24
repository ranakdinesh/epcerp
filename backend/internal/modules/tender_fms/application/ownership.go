package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

type OwnershipService struct {
	unit   ports.OwnershipUnit
	auth   ports.Authorizer
	values ports.Values
}

func NewOwnership(u ports.OwnershipUnit, a ports.Authorizer, v ports.Values) *OwnershipService {
	return &OwnershipService{u, a, v}
}
func ownershipEligible(u domain.Principal) bool {
	if !u.Active {
		return false
	}
	for _, p := range []string{"tenders.register.view", "tenders.register.read", "tenders.register.write", "tenders.processing.read", "tenders.processing.write"} {
		if !u.Allows(p) {
			return false
		}
	}
	return true
}
func (s *OwnershipService) actor(ctx context.Context, token string) (domain.Principal, error) {
	a, e := s.auth.Resolve(ctx, token)
	if e != nil || !a.Active {
		return a, domain.Unauthorized
	}
	if !a.Allows("tenders.register.assign") {
		return a, domain.Forbidden
	}
	return a, nil
}
func (s *OwnershipService) Options(ctx context.Context, token, id string) (domain.Intake, []domain.OwnershipCandidate, error) {
	var intake domain.Intake
	out := []domain.OwnershipCandidate{}
	a, e := s.actor(ctx, token)
	if e != nil {
		return intake, out, e
	}
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.OwnershipRepository, u ports.OwnershipUsers) error {
		live, e := u.Get(ctx, a.ID)
		if e != nil {
			return e
		}
		if live == nil || !live.Active || !live.Allows("tenders.register.assign") {
			return domain.Forbidden
		}
		intake, e = r.Get(ctx, id)
		if e != nil {
			return e
		}
		if e = authorize(*live, intake, "write"); e != nil {
			return e
		}
		choices, e := u.List(ctx)
		if e != nil {
			return e
		}
		for _, c := range choices {
			p, e := u.Get(ctx, c.ID)
			if e != nil {
				return e
			}
			if p != nil && p.ID != intake.OwnerID && ownershipEligible(*p) {
				out = append(out, c)
			}
		}
		return nil
	})
	return intake, out, e
}
func (s *OwnershipService) Handover(ctx context.Context, token, id string, in domain.OwnershipInput) (domain.Intake, error) {
	var out domain.Intake
	in = in.Normalized()
	if e := in.Validate(); e != nil {
		return out, e
	}
	a, e := s.actor(ctx, token)
	if e != nil {
		return out, e
	}
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.OwnershipRepository, u ports.OwnershipUsers) error {
		live, e := u.Get(ctx, a.ID)
		if e != nil {
			return e
		}
		if live == nil || !live.Active || !live.Allows("tenders.register.assign") {
			return domain.Forbidden
		}
		current, e := r.Get(ctx, id)
		if e != nil {
			return e
		}
		if e = authorize(*live, current, "write"); e != nil {
			return e
		}
		if !current.Active || current.Version != in.Version || current.OwnerID == in.OwnerID {
			return domain.Invalid("Refresh the active intake and choose a different owner")
		}
		candidate, e := u.Get(ctx, in.OwnerID)
		if e != nil {
			return e
		}
		if candidate == nil || !ownershipEligible(*candidate) {
			return domain.Invalid("Choose an active owner with all tender reading and processing permissions")
		}
		out, e = r.SaveOwner(ctx, id, in, a.ID, s.values.ID())
		return e
	})
	return out, e
}
