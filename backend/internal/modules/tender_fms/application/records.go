package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

// RecordsService is a transaction participant; the caller owns commit and replay.
type RecordsService struct {
	r      ports.RecordRepository
	users  ports.EligibleUsers
	values ports.Values
}

func NewRecords(r ports.RecordRepository, u ports.EligibleUsers, v ports.Values) *RecordsService {
	return &RecordsService{r, u, v}
}
func (s *RecordsService) actor(ctx context.Context, id string) (*domain.EligibleIdentity, error) {
	if _, e := domain.ID(id); e != nil {
		return nil, domain.Unauthorized
	}
	if e := s.r.LockIdentity(ctx); e != nil {
		return nil, e
	}
	u, e := s.users.Get(ctx, id)
	if e != nil {
		return nil, e
	}
	if u == nil || !u.Principal.Active {
		return nil, domain.Unauthorized
	}
	if !u.Ready {
		return nil, domain.Forbidden
	}
	return u, nil
}
func (s *RecordsService) CreateNamed(ctx context.Context, actor string, in domain.NamedInput) (domain.Record, error) {
	var out domain.Record
	a, e := s.actor(ctx, actor)
	if e != nil {
		return out, e
	}
	if !domain.CanCapture(a.Principal) {
		return out, domain.Forbidden
	}
	in, e = in.Normalize()
	if e != nil {
		return out, e
	}
	target, e := s.users.Get(ctx, in.ManagerID)
	if e != nil {
		return out, e
	}
	if target == nil || !target.CanManage() {
		return out, domain.Invalid("Select a ready, designated Tender Manager with current required permissions")
	}
	return s.r.CreateNamed(ctx, in, a.Principal.ID, s.values.ID(), s.values.ID(), s.values.ID())
}
func (s *RecordsService) Read(ctx context.Context, actor, id string) (domain.Record, error) {
	var out domain.Record
	a, e := s.actor(ctx, actor)
	if e != nil {
		return out, e
	}
	if !a.Principal.Allows("tenders.register.view") || !a.Principal.Allows("tenders.register.read") {
		return out, domain.Forbidden
	}
	id, e = domain.ID(id)
	if e != nil {
		return out, e
	}
	out, e = s.r.GetNamed(ctx, id)
	if e != nil {
		return out, e
	}
	participant, e := s.r.ParticipantActive(ctx, id, a.Principal.ID)
	if e != nil {
		return domain.Record{}, e
	}
	if !out.CanRead(a.Principal) && !participant {
		return domain.Record{}, domain.Missing
	}
	return out, nil
}
func (s *RecordsService) ManagerOptions(ctx context.Context, actor string) ([]domain.Candidate, error) {
	out := []domain.Candidate{}
	a, e := s.actor(ctx, actor)
	if e != nil {
		return out, e
	}
	if !domain.CanCapture(a.Principal) {
		return out, domain.Forbidden
	}
	candidates, e := s.users.List(ctx)
	if e != nil {
		return out, e
	}
	for _, v := range candidates {
		u, e := s.users.Get(ctx, v.ID)
		if e != nil {
			return nil, e
		}
		if u != nil && u.CanManage() {
			out = append(out, v)
		}
	}
	return out, nil
}

// GrantParticipant records explicit metadata sharing; future task integrations must
// consume this owned command, never insert an access row from another module.
func (s *RecordsService) GrantParticipant(ctx context.Context, actor, id string, in domain.ParticipantGrant) (int64, error) {
	a, e := s.actor(ctx, actor)
	if e != nil {
		return 0, e
	}
	if !a.Principal.Allows("tenders.register.assign") || !a.Principal.Allows("tenders.register.read") || !a.Principal.Allows("tenders.register.view") {
		return 0, domain.Forbidden
	}
	id, e = domain.ID(id)
	if e != nil {
		return 0, e
	}
	v, e := s.r.GetNamed(ctx, id)
	if e != nil {
		return 0, e
	}
	if v.ManagerID != a.Principal.ID && !a.Principal.Allows("tenders.portfolio.write") {
		return 0, domain.Missing
	}
	if v.ManagerID == a.Principal.ID && !a.CanManage() && !a.Principal.Allows("tenders.portfolio.write") {
		return 0, domain.Forbidden
	}
	if !v.Active {
		return 0, domain.Invalid("Tender is inactive")
	}
	in, e = in.Normalize()
	if e != nil {
		return 0, e
	}
	if in.Active {
		target, e := s.users.Get(ctx, in.UserID)
		if e != nil {
			return 0, e
		}
		if target == nil || !target.Ready || !target.Principal.Allows("tenders.register.read") || !target.Principal.Allows("tenders.register.view") {
			return 0, domain.Invalid("Select an active ready participant with Tender read permissions")
		}
	}
	return s.r.SaveParticipant(ctx, id, a.Principal.ID, s.values.ID(), in)
}
