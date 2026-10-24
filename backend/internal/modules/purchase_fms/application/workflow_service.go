package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type WorkflowService struct {
	u ports.WorkflowUnitOfWork
	a ports.WorkflowAuthorizer
	v ports.WorkflowValues
}

func NewWorkflow(u ports.WorkflowUnitOfWork, a ports.WorkflowAuthorizer, v ports.WorkflowValues) *WorkflowService {
	return &WorkflowService{u, a, v}
}
func (s *WorkflowService) actor(ctx context.Context, token, p string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil {
		return a, e
	}
	if !a.Active || a.ID == "" {
		return a, domain.WorkflowUnauthorized
	}
	if e = domain.WorkflowID(p); e != nil {
		return a, e
	}
	return a, nil
}
func (s *WorkflowService) Listing(ctx context.Context, token, p string, filters ...domain.WorkflowPageFilter) (out domain.WorkflowListing, e error) {
	a, e := s.actor(ctx, token, p)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.WorkflowRepository, c ports.WorkflowContext) error {
		caps, err := c.Actions(ctx, a, p)
		if err != nil {
			return err
		}
		if !caps.Read {
			return domain.WorkflowForbidden
		}
		filter := domain.WorkflowPageFilter{}
		if len(filters) > 0 {
			filter = filters[0]
		}
		filter, err = filter.Validate()
		if err != nil {
			return err
		}
		paged, ok := r.(ports.WorkflowPageRepository)
		if !ok {
			return domain.WorkflowInvalid("packages", "Paged package listing is unavailable")
		}
		out.Page = filter.Page
		out.Size = filter.Size
		out.Packages, out.Total, err = paged.Page(ctx, p, filter)
		if err != nil {
			return err
		}
		out.ActorID = a.ID
		out.CanCreate = caps.Write
		out.Candidates = []domain.WorkflowCandidate{}
		out.Members = []domain.WorkflowMember{}
		if caps.Write {
			out.Candidates, err = c.Candidates(ctx, p)
			if err != nil {
				return err
			}
			out.Members, err = c.Members(ctx, p)
		}
		return err
	})
	if e != nil {
		out = domain.WorkflowListing{}
	}
	return
}
func workflowTaskAssigned(s domain.WorkflowStep, id string) bool {
	return (s.DoerID != nil && *s.DoerID == id) || (s.ResolverID != nil && *s.ResolverID == id) || (s.BlockedResolverID != nil && *s.BlockedResolverID == id)
}
func workflowWorkspace(ctx context.Context, r ports.WorkflowRepository, c ports.WorkflowContext, a domain.Principal, p, id string) (out domain.WorkflowWorkspace, e error) {
	caps, e := c.Actions(ctx, a, p)
	if e != nil {
		return out, e
	}
	record, e := r.Get(ctx, p, id)
	if e != nil {
		return out, e
	}
	if record == nil || !record.Active {
		return out, domain.WorkflowMissing
	}
	if !caps.Read {
		assigned := false
		for _, step := range record.Steps {
			assigned = assigned || workflowTaskAssigned(step, a.ID)
		}
		if !assigned || (!caps.Execute && !caps.Resolve) {
			return out, domain.WorkflowForbidden
		}
		out.TaskOnly = true
	}
	out.Package = domain.WorkflowCopy(*record)
	out.Actions = map[int][]string{}
	out.Members = []domain.WorkflowMember{}
	out.History = []domain.WorkflowEvent{}
	out.CanAssign = caps.Write && !out.TaskOnly
	out.Source, e = c.Source(ctx, p, record.IndentID, record.DemandID)
	if e != nil {
		return out, e
	}
	if out.TaskOnly {
		steps := []domain.WorkflowStep{}
		for _, step := range out.Package.Steps {
			if workflowTaskAssigned(step, a.ID) {
				steps = append(steps, step)
			}
		}
		out.Package.Steps = steps
	} else {
		out.History, e = r.History(ctx, p, id)
		if e != nil {
			return out, e
		}
		if caps.Write {
			out.Members, e = c.Members(ctx, p)
			if e != nil {
				return out, e
			}
		}
	}
	for _, step := range out.Package.Steps {
		actions := []string{}
		for _, action := range []string{"save", "start", "block", "resolve"} {
			if domain.WorkflowWorkGuard(a, caps, step, action) == nil && out.Source.Guard() == nil {
				actions = append(actions, action)
			}
		}
		out.Actions[step.Number] = actions
	}
	return out, nil
}
func (s *WorkflowService) Workspace(ctx context.Context, token, p, id string) (out domain.WorkflowWorkspace, e error) {
	a, e := s.actor(ctx, token, p)
	if e != nil {
		return out, e
	}
	if e = domain.WorkflowID(id); e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.WorkflowRepository, c ports.WorkflowContext) error {
		out, e = workflowWorkspace(ctx, r, c, a, p, id)
		return e
	})
	if e != nil {
		out = domain.WorkflowWorkspace{}
	}
	return
}
func (s *WorkflowService) Create(ctx context.Context, token, p string, q domain.WorkflowCreate) (out domain.WorkflowWorkspace, e error) {
	a, e := s.actor(ctx, token, p)
	if e != nil {
		return out, e
	}
	if e = domain.WorkflowID(q.IndentID); e != nil {
		return out, e
	}
	if e = domain.WorkflowID(q.DemandID); e != nil {
		return out, e
	}
	q.Reason, e = domain.WorkflowText("reason", q.Reason, 2000, true)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.WorkflowRepository, c ports.WorkflowContext) error {
		caps, err := c.Actions(ctx, a, p)
		if err != nil {
			return err
		}
		if !caps.Read || !caps.Write {
			return domain.WorkflowForbidden
		}
		source, err := c.Source(ctx, p, q.IndentID, q.DemandID)
		if err != nil {
			return err
		}
		if err = source.Guard(); err != nil {
			return err
		}
		exists, err := r.Exists(ctx, p, q.IndentID, q.DemandID)
		if err != nil {
			return err
		}
		if exists {
			return domain.WorkflowDuplicate
		}
		now := s.v.Now()
		id := s.v.ID()
		record := domain.WorkflowPackage{ID: id, ProjectID: p, IndentID: q.IndentID, DemandID: q.DemandID, Code: "PF-" + id, Description: source.Description, Source: source, Version: 1, Active: true, CreatedAt: now, UpdatedAt: now, CreatedBy: &a.ID, UpdatedBy: &a.ID, Steps: domain.WorkflowDefinitions()}
		for i := range record.Steps {
			step := &record.Steps[i]
			if step.Number <= 2 {
				step.State = "source_linked"
			}
			step.UpdatedAt = now
			step.UpdatedBy = a.ID
		}
		return s.persist(ctx, r, c, a, record, nil, "create", q.Reason, &out)
	})
	if e != nil {
		out = domain.WorkflowWorkspace{}
	}
	return
}
func (s *WorkflowService) persist(ctx context.Context, r ports.WorkflowRepository, c ports.WorkflowContext, a domain.Principal, record domain.WorkflowPackage, before *domain.WorkflowPackage, action, reason string, out *domain.WorkflowWorkspace) error {
	saved, e := r.Save(ctx, record, a.ID)
	if e != nil {
		return e
	}
	if e = r.Event(ctx, domain.WorkflowEvent{ID: s.v.ID(), Action: action, Reason: reason, Before: before, After: saved, At: s.v.Now(), ActorID: a.ID}); e != nil {
		return e
	}
	*out, e = workflowWorkspace(ctx, r, c, a, saved.ProjectID, saved.ID)
	return e
}
func (s *WorkflowService) Assign(ctx context.Context, token, p, id string, q domain.WorkflowAssign) (out domain.WorkflowWorkspace, e error) {
	a, e := s.actor(ctx, token, p)
	if e != nil {
		return out, e
	}
	if e = domain.WorkflowID(id); e != nil {
		return out, e
	}
	q.Reason, e = domain.WorkflowText("reason", q.Reason, 2000, true)
	if e != nil {
		return out, e
	}
	if q.Version < 1 {
		return out, domain.WorkflowInvalid("version", "Use the saved package version")
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.WorkflowRepository, c ports.WorkflowContext) error {
		caps, err := c.Actions(ctx, a, p)
		if err != nil {
			return err
		}
		if !caps.Read || !caps.Write {
			return domain.WorkflowForbidden
		}
		record, err := r.Get(ctx, p, id)
		if err != nil {
			return err
		}
		if record == nil || !record.Active {
			return domain.WorkflowMissing
		}
		if record.Version != q.Version {
			return domain.WorkflowStale
		}
		source, err := c.Source(ctx, p, record.IndentID, record.DemandID)
		if err != nil {
			return err
		}
		if err = source.Guard(); err != nil {
			return err
		}
		members, err := c.Members(ctx, p)
		if err != nil {
			return err
		}
		before := domain.WorkflowCopy(*record)
		updated, err := domain.WorkflowApplyAssignments(*record, q.Rows, members, a.ID, s.v.Now())
		if err != nil {
			return err
		}
		updated.Version++
		return s.persist(ctx, r, c, a, updated, &before, "assign", q.Reason, &out)
	})
	if e != nil {
		out = domain.WorkflowWorkspace{}
	}
	return
}
func (s *WorkflowService) Command(ctx context.Context, token, p, id string, number int, q domain.WorkflowCommand) (out domain.WorkflowWorkspace, e error) {
	a, e := s.actor(ctx, token, p)
	if e != nil {
		return out, e
	}
	if e = domain.WorkflowID(id); e != nil {
		return out, e
	}
	q.Reason, e = domain.WorkflowText("reason", q.Reason, 2000, true)
	if e != nil {
		return out, e
	}
	if q.Version < 1 {
		return out, domain.WorkflowInvalid("version", "Use the saved package version")
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.WorkflowRepository, c ports.WorkflowContext) error {
		caps, err := c.Actions(ctx, a, p)
		if err != nil {
			return err
		}
		record, err := r.Get(ctx, p, id)
		if err != nil {
			return err
		}
		if record == nil || !record.Active {
			return domain.WorkflowMissing
		}
		if record.Version != q.Version {
			return domain.WorkflowStale
		}
		if len(record.Steps) != 20 {
			return domain.WorkflowInvalid("package", "Invalid package step catalogue")
		}
		if number < 1 || number > 20 {
			return domain.WorkflowInvalid("step", "Unknown purchase step")
		}
		if err = domain.WorkflowWorkGuard(a, caps, record.Steps[number-1], q.Action); err != nil {
			return err
		}
		source, err := c.Source(ctx, p, record.IndentID, record.DemandID)
		if err != nil {
			return err
		}
		if err = source.Guard(); err != nil {
			return err
		}
		before := domain.WorkflowCopy(*record)
		updated, err := domain.WorkflowApplyCommand(*record, number, q, a, caps, s.v.Now())
		if err != nil {
			return err
		}
		updated.Version++
		return s.persist(ctx, r, c, a, updated, &before, q.Action, q.Reason, &out)
	})
	if e != nil {
		out = domain.WorkflowWorkspace{}
	}
	return
}
