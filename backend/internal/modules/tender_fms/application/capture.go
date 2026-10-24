package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

type CaptureService struct {
	limits ports.CommandLimits
	u      ports.CaptureUnit
	auth   ports.Authorizer
	values ports.CommandValues
}

func NewCapture(u ports.CaptureUnit, a ports.Authorizer, v ports.CommandValues) *CaptureService {
	return &CaptureService{u: u, auth: a, values: v}
}
func (s *CaptureService) actor(ctx context.Context, token string) (string, error) {
	p, e := s.auth.Resolve(ctx, token)
	if e != nil {
		return "", e
	}
	if !p.Active || p.ID == "" {
		return "", domain.Unauthorized
	}
	return p.ID, nil
}
func captureActor(ctx context.Context, r ports.CaptureRepository, u ports.EligibleUsers, id string) (*domain.EligibleIdentity, error) {
	if e := r.Guard(ctx); e != nil {
		return nil, e
	}
	p, e := u.Get(ctx, id)
	if e != nil {
		return nil, e
	}
	if p == nil || !p.Principal.Active {
		return nil, domain.Unauthorized
	}
	if !p.Ready {
		return nil, domain.Forbidden
	}
	return p, nil
}
func (s *CaptureService) Authorize(ctx context.Context, token string, write bool) error {
	id, e := s.actor(ctx, token)
	if e != nil {
		return e
	}
	return s.u.Within(ctx, func(ctx context.Context, r ports.CaptureRepository, f ports.RecordFoundation, p ports.CaptureSources, u ports.EligibleUsers, n ports.AssignmentNotices) error {
		a, e := captureActor(ctx, r, u, id)
		if e != nil {
			return e
		}
		if write {
			if !domain.CanCapture(a.Principal) {
				return domain.Forbidden
			}
		} else if !a.Principal.Allows("tenders.register.view") || !a.Principal.Allows("tenders.register.read") {
			return domain.Forbidden
		}
		return nil
	})
}
func (s *CaptureService) Capture(ctx context.Context, token, key string, in domain.CaptureInput) (out domain.Captured, replay bool, e error) {
	actor, e := s.actor(ctx, token)
	if e == nil {
		e = s.limit(actor, "capture")
	}
	if e != nil {
		return out, false, e
	}
	if e = domain.CanonicalID(key); e != nil {
		return
	}
	in, e = in.Normalize()
	if e != nil {
		return
	}
	fp, e := s.values.Fingerprint(in)
	if e != nil {
		return
	}
	op := domain.CaptureOperation{Actor: actor, Command: "capture", Context: "named", Key: key, Fingerprint: fp}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.CaptureRepository, f ports.RecordFoundation, p ports.CaptureSources, u ports.EligibleUsers, n ports.AssignmentNotices) error {
		a, e := captureActor(ctx, r, u, actor)
		if e != nil {
			return e
		}
		if !domain.CanCapture(a.Principal) {
			return domain.Forbidden
		}
		if e = r.LockOperation(ctx, op); e != nil {
			return e
		}
		prev, e := r.Operation(ctx, op)
		if e != nil {
			return e
		}
		if prev != nil {
			if prev.Fingerprint != fp {
				return domain.CaptureFailure(409, "idempotency_conflict", "Operation key was used with different input")
			}
			record, e := f.Read(ctx, actor, prev.TenderID)
			if e != nil {
				return e
			}
			out, e = captureProjection(ctx, r, u, record, *a)
			replay = true
			return e
		}
		record, e := f.CreateNamed(ctx, actor, domain.NamedInput{Name: in.Name, ManagerID: in.ManagerID})
		if e != nil {
			return e
		}
		receipt, e := p.Consume(ctx, actor, in.ReceiptID, in.ReceiptVersion, record.ID)
		if e != nil {
			return e
		}
		if e = r.Bind(ctx, record, in, actor); e != nil {
			return e
		}
		members, e := p.MemberIDs(ctx, receipt)
		if e != nil {
			return e
		}
		if len(members) == 0 {
			return domain.CaptureFailure(409, "receipt_not_ready", "Verified package contains no sources")
		}
		for _, member := range members {
			document, e := p.Queue(ctx, receipt, member, actor, record.ID)
			if e != nil {
				return e
			}
			if e = r.LinkSource(ctx, record.ID, member, document, actor); e != nil {
				return e
			}
		}
		action, e := r.ManagerWork(ctx, record, actor)
		if e != nil {
			return e
		}
		notice, e := n.Deliver(ctx, action)
		if e != nil {
			return e
		}
		if e = r.LinkNotice(ctx, action.ID, notice); e != nil {
			return e
		}
		op.TenderID = record.ID
		if e = r.SaveOperation(ctx, op); e != nil {
			return e
		}
		out, e = captureProjection(ctx, r, u, record, *a)
		return e
	})
	return
}
func (s *CaptureService) Get(ctx context.Context, token, id string) (out domain.Captured, e error) {
	actor, e := s.actor(ctx, token)
	if e == nil {
		e = s.limit(actor, "read")
	}
	if e != nil {
		return out, e
	}
	if e = domain.CanonicalID(id); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.CaptureRepository, f ports.RecordFoundation, p ports.CaptureSources, u ports.EligibleUsers, n ports.AssignmentNotices) error {
		a, e := captureActor(ctx, r, u, actor)
		if e != nil {
			return e
		}
		record, e := f.Read(ctx, actor, id)
		if e != nil {
			return e
		}
		out, e = captureProjection(ctx, r, u, record, *a)
		return e
	})
	return
}
func (s *CaptureService) List(ctx context.Context, token string, page, size int32) ([]domain.Captured, int64, error) {
	return s.ListFiltered(ctx, token, page, size, "", "created", true)
}
func (s *CaptureService) ListFiltered(ctx context.Context, token string, page, size int32, search, sort string, descending bool) (out []domain.Captured, total int64, e error) {
	out = []domain.Captured{}
	if e = domain.ValidateRegisterQuery(search, sort); e != nil {
		return out, 0, e
	}
	actor, e := s.actor(ctx, token)
	if e == nil {
		e = s.limit(actor, "read")
	}
	if e != nil {
		return out, 0, e
	}
	if page < 1 || page > 100000 || (size != 20 && size != 50 && size != 100) {
		return out, 0, domain.Invalid("Invalid pagination")
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.CaptureRepository, f ports.RecordFoundation, p ports.CaptureSources, u ports.EligibleUsers, n ports.AssignmentNotices) error {
		a, e := captureActor(ctx, r, u, actor)
		if e != nil {
			return e
		}
		if !a.Principal.Allows("tenders.register.view") || !a.Principal.Allows("tenders.register.read") {
			return domain.Forbidden
		}
		ids, count, e := r.List(ctx, actor, a.Principal.Allows("tenders.portfolio.read"), page, size, search, sort, descending)
		if e != nil {
			return e
		}
		total = count
		for _, id := range ids {
			record, e := f.Read(ctx, actor, id)
			if e != nil {
				return e
			}
			v, e := captureProjection(ctx, r, u, record, *a)
			if e != nil {
				return e
			}
			out = append(out, captureCapabilities(v, *a))
		}
		return nil
	})
	return
}
func (s *CaptureService) Change(ctx context.Context, token, id, key string, in domain.ChangeManager) (out domain.Captured, replay bool, e error) {
	actor, e := s.actor(ctx, token)
	if e == nil {
		e = s.limit(actor, "assignment")
	}
	if e != nil {
		return out, false, e
	}
	for _, id := range []string{id, key} {
		if e = domain.CanonicalID(id); e != nil {
			return
		}
	}
	in, e = in.Normalize()
	if e != nil {
		return
	}
	fp, e := s.values.Fingerprint(in)
	if e != nil {
		return
	}
	op := domain.CaptureOperation{Actor: actor, Command: "manager", Context: id, Key: key, Fingerprint: fp}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.CaptureRepository, f ports.RecordFoundation, p ports.CaptureSources, u ports.EligibleUsers, n ports.AssignmentNotices) error {
		a, e := captureActor(ctx, r, u, actor)
		if e != nil {
			return e
		}
		if !a.Principal.Allows("tenders.register.assign") {
			return domain.Forbidden
		}
		if e = r.LockOperation(ctx, op); e != nil {
			return e
		}
		record, e := f.Read(ctx, actor, id)
		if e != nil {
			return e
		}
		if record.ManagerID != actor && record.SalesCreatorID != actor && !a.Principal.Allows("tenders.portfolio.write") {
			return domain.Forbidden
		}
		if record.ManagerID == actor && !a.CanManage() && !a.Principal.Allows("tenders.portfolio.write") {
			return domain.Forbidden
		}
		prev, e := r.Operation(ctx, op)
		if e != nil {
			return e
		}
		if prev != nil {
			if prev.Fingerprint != fp {
				return domain.CaptureFailure(409, "idempotency_conflict", "Operation key was used with different input")
			}
			out, e = captureProjection(ctx, r, u, record, *a)
			replay = true
			return e
		}
		if !record.Active || record.BusinessVersion != in.Version || record.AssignmentVersion != in.AssignmentVersion || record.ManagerID == in.ManagerID {
			return domain.CaptureFailure(409, "stale_version", "Refresh the active record and select a different Manager")
		}
		target, e := u.Get(ctx, in.ManagerID)
		if e != nil {
			return e
		}
		if target == nil || !target.CanManage() {
			return domain.Invalid("Select a ready, designated Manager with the required capabilities")
		}
		record, e = r.ChangeManager(ctx, record, in, actor)
		if e != nil {
			return e
		}
		action, e := r.ManagerWork(ctx, record, actor)
		if e != nil {
			return e
		}
		notice, e := n.Deliver(ctx, action)
		if e != nil {
			return e
		}
		if e = r.LinkNotice(ctx, action.ID, notice); e != nil {
			return e
		}
		op.TenderID = id
		if e = r.SaveOperation(ctx, op); e != nil {
			return e
		}
		out, e = captureProjection(ctx, r, u, record, *a)
		return e
	})
	return
}
func (s *CaptureService) Patch(ctx context.Context, token, id, key string, in domain.Patch) (out domain.Captured, replay bool, e error) {
	actor, e := s.actor(ctx, token)
	if e == nil {
		e = s.limit(actor, "assignment")
	}
	if e != nil {
		return out, false, e
	}
	for _, id := range []string{id, key} {
		if e = domain.CanonicalID(id); e != nil {
			return
		}
	}
	if in.Version < 1 || in.Version >= 9007199254740991 {
		return out, false, domain.Invalid("Current business version required")
	}
	check, e := (domain.CaptureInput{Name: in.Name, ReceiptID: id, ReceiptVersion: in.Version, ManagerID: id, SourceURL: in.SourceURL, ExternalReference: in.ExternalReference}).Normalize()
	if e != nil {
		return out, false, e
	}
	in.Name = check.Name
	fp, e := s.values.Fingerprint(in)
	if e != nil {
		return
	}
	op := domain.CaptureOperation{Actor: actor, Command: "patch", Context: id, Key: key, Fingerprint: fp}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.CaptureRepository, f ports.RecordFoundation, p ports.CaptureSources, u ports.EligibleUsers, n ports.AssignmentNotices) error {
		a, e := captureActor(ctx, r, u, actor)
		if e != nil {
			return e
		}
		if !a.Principal.Allows("tenders.register.write") {
			return domain.Forbidden
		}
		if e = r.LockOperation(ctx, op); e != nil {
			return e
		}
		record, e := f.Read(ctx, actor, id)
		if e != nil {
			return e
		}
		if record.SalesCreatorID != actor && record.ManagerID != actor && !a.Principal.Allows("tenders.portfolio.write") {
			return domain.Forbidden
		}
		prev, e := r.Operation(ctx, op)
		if e != nil {
			return e
		}
		if prev != nil {
			if prev.Fingerprint != fp {
				return domain.CaptureFailure(409, "idempotency_conflict", "Operation key was used with different input")
			}
			out, e = captureProjection(ctx, r, u, record, *a)
			replay = true
			return e
		}
		if !record.Active || record.BusinessVersion != in.Version {
			return domain.CaptureFailure(409, "stale_version", "Tender changed; refresh before editing")
		}
		record, e = r.Patch(ctx, record, in, actor)
		if e != nil {
			return e
		}
		op.TenderID = id
		if e = r.SaveOperation(ctx, op); e != nil {
			return e
		}
		out, e = captureProjection(ctx, r, u, record, *a)
		return e
	})
	return
}

func captureCapabilities(v domain.Captured, a domain.EligibleIdentity) domain.Captured {
	p := a.Principal
	scope := v.Record.SalesCreatorID == p.ID || v.Record.ManagerID == p.ID || p.Allows("tenders.portfolio.write")
	v.CanEdit = scope && p.Allows("tenders.register.write")
	v.CanAssign = scope && p.Allows("tenders.register.assign") && (v.Record.ManagerID != p.ID || a.CanManage() || p.Allows("tenders.portfolio.write"))
	return v
}

func captureProjection(ctx context.Context, r ports.CaptureRepository, u ports.EligibleUsers, record domain.Record, a domain.EligibleIdentity) (domain.Captured, error) {
	out, e := r.Projection(ctx, record)
	if e != nil {
		return out, e
	}
	manager, e := u.Get(ctx, record.ManagerID)
	if e != nil {
		return out, e
	}
	if manager != nil {
		out.ManagerName = manager.DisplayName
	}
	return captureCapabilities(out, a), nil
}

func (s *CaptureService) WithLimits(l ports.CommandLimits) *CaptureService { s.limits = l; return s }
func (s *CaptureService) limit(actor, operation string) error {
	if s.limits != nil && !s.limits.Allow(actor, operation) {
		return domain.CaptureFailure(429, "rate_limited", "Too many Tender requests")
	}
	return nil
}
