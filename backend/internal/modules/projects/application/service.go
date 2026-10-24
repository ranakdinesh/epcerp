package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/ports"
)

type Service struct {
	unit   ports.UnitOfWork
	auth   ports.Authorizer
	values ports.Values
}

func New(u ports.UnitOfWork, a ports.Authorizer, v ports.Values) *Service { return &Service{u, a, v} }
func (s *Service) actor(ctx context.Context, token string) (domain.Principal, error) {
	p, err := s.auth.Resolve(ctx, token)
	if err != nil {
		return p, err
	}
	if !p.Active || p.ID == "" {
		return p, domain.ErrUnauthorized
	}
	if !p.Allows("projects.master.read") {
		return p, domain.ErrForbidden
	}
	return p, nil
}
func (s *Service) List(ctx context.Context, token string, f domain.Filter) (domain.Page, error) {
	p, err := s.actor(ctx, token)
	if err != nil {
		return domain.Page{}, err
	}
	f, err = f.Validate()
	if err != nil {
		return domain.Page{}, err
	}
	var result domain.Page
	manager := p.ID
	if p.Portfolio() {
		manager = ""
	}
	err = s.unit.Within(ctx, func(ctx context.Context, r ports.Repository, _ ports.Clients, _ ports.Users, _ ports.Calendar, purchasing ports.Purchasing) error {
		var e error
		if scoped, ok := r.(ports.ScopedReadRepository); ok {
			result, e = scoped.ListReadable(ctx, f, p)
		} else {
			result, e = r.List(ctx, f, manager)
		}
		if e != nil {
			return e
		}
		for i := range result.Items {
			result.Items[i].ProcurementExecutiveID, e = purchasing.Owner(ctx, result.Items[i].ID)
			if e != nil {
				return e
			}
		}
		return nil
	})
	return result, err
}
func (s *Service) Get(ctx context.Context, token, id string) (*domain.Project, error) {
	p, err := s.actor(ctx, token)
	if err != nil {
		return nil, err
	}
	if id, err = domain.ID(id); err != nil {
		return nil, err
	}
	var result *domain.Project
	err = s.unit.Within(ctx, func(ctx context.Context, r ports.Repository, _ ports.Clients, _ ports.Users, _ ports.Calendar, purchasing ports.Purchasing) error {
		var e error
		result, e = r.Get(ctx, id, false)
		if e != nil {
			return e
		}
		if result == nil {
			return domain.ErrMissing
		}
		if allowed, e := canReadProject(ctx, r, p, *result); e != nil {
			return e
		} else if !allowed {
			return domain.ErrForbidden
		}
		result.ProcurementExecutiveID, e = purchasing.Owner(ctx, id)
		return e
	})
	return result, err
}
func (s *Service) Options(ctx context.Context, token string) (domain.Options, error) {
	p, err := s.actor(ctx, token)
	if err != nil {
		return domain.Options{}, err
	}
	if !(p.Allows("projects.master.write") && p.Allows("projects.portfolio.write")) && !p.Allows("projects.manager_assignments.write") {
		return domain.Options{}, domain.ErrForbidden
	}
	result := domain.Options{Clients: []domain.Client{}, Managers: []domain.Candidate{}}
	err = s.unit.Within(ctx, func(ctx context.Context, _ ports.Repository, c ports.Clients, u ports.Users, calendar ports.Calendar, purchasing ports.Purchasing) error {
		users, e := u.List(ctx)
		if e != nil {
			return e
		}
		for _, user := range users {

			if user.Eligible() {
				result.Managers = append(result.Managers, user)
			}
		}
		result.Procurement, e = purchasing.Candidates(ctx)
		if e != nil {
			return e
		}
		if p.Allows("projects.master.write") && p.Allows("projects.portfolio.write") {
			result.Clients, e = c.List(ctx)
		}
		return e
	})
	return result, err
}
func (s *Service) Create(ctx context.Context, token string, input domain.Create) (*domain.Project, error) {
	p, err := s.actor(ctx, token)
	if err != nil {
		return nil, err
	}
	if !p.Allows("projects.master.write") || !p.Allows("projects.portfolio.write") || !p.Allows("projects.purchase_assignments.write") {
		return nil, domain.ErrForbidden
	}
	input, err = input.Validate()
	if err != nil {
		return nil, err
	}
	var saved domain.Project
	err = s.unit.Within(ctx, func(ctx context.Context, r ports.Repository, c ports.Clients, u ports.Users, calendar ports.Calendar, purchasing ports.Purchasing) error {
		client, e := c.Get(ctx, input.ClientID)
		if e != nil {
			return e
		}
		if client == nil || !client.Active {
			return &domain.Invalid{Field: "client_id", Message: "Select an active client"}
		}
		manager, e := u.Get(ctx, input.ManagerID)
		if e != nil {
			return e
		}
		if manager == nil || !manager.Eligible() {
			return &domain.Invalid{Field: "project_manager_id", Message: "Select an active manager with project read/write permissions"}
		}
		input.Capture, e = calculate(ctx, input.Capture, calendar)
		if e != nil {
			return e
		}
		saved, e = r.Create(ctx, domain.Project{Capture: input.Capture, ID: s.values.ID(), Name: input.Name, NameKey: domain.NameKey(input.Name), ClientID: input.ClientID, ManagerID: input.ManagerID, CreatedBy: p.ID, UpdatedBy: p.ID})
		if e != nil {
			return e
		}
		if e = r.Assignment(ctx, domain.Assignment{ID: s.values.ID(), ProjectID: saved.ID, ManagerID: saved.ManagerID, Version: saved.Version, Reason: "Initial project manager assignment", ActorID: p.ID, OccurredAt: saved.CreatedAt}); e != nil {
			return e
		}
		if e = purchasing.Initialize(ctx, saved.ID, input.ProcurementExecutiveID, p.ID); e != nil {
			return e
		}
		saved.ProcurementExecutiveID = input.ProcurementExecutiveID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}
func (s *Service) Transfer(ctx context.Context, token, id string, input domain.Transfer) (*domain.Project, error) {
	p, err := s.actor(ctx, token)
	if err != nil {
		return nil, err
	}
	if !p.Allows("projects.manager_assignments.write") {
		return nil, domain.ErrForbidden
	}
	if id, err = domain.ID(id); err != nil {
		return nil, err
	}
	input, err = input.Validate()
	if err != nil {
		return nil, err
	}
	var saved domain.Project
	err = s.unit.Within(ctx, func(ctx context.Context, r ports.Repository, _ ports.Clients, u ports.Users, calendar ports.Calendar, purchasing ports.Purchasing) error {
		project, e := r.Get(ctx, id, true)
		if e != nil {
			return e
		}
		if project == nil {
			return domain.ErrMissing
		}
		if project.Version != input.ExpectedVersion {
			return domain.ErrStale
		}
		if project.ManagerID == input.ManagerID {
			return &domain.Invalid{Field: "project_manager_id", Message: "Choose a different project manager"}
		}
		manager, e := u.Get(ctx, input.ManagerID)
		if e != nil {
			return e
		}
		if manager == nil || !manager.Eligible() {
			return &domain.Invalid{Field: "project_manager_id", Message: "Select an active manager with project read/write permissions"}
		}
		previous := project.ManagerID
		project.UpdatedBy = p.ID
		saved, e = r.Transfer(ctx, *project, input.ManagerID)
		if e != nil {
			return e
		}
		if e = r.Assignment(ctx, domain.Assignment{ID: s.values.ID(), ProjectID: id, BeforeManagerID: previous, ManagerID: saved.ManagerID, Version: saved.Version, Reason: input.Reason, ActorID: p.ID, OccurredAt: saved.UpdatedAt}); e != nil {
			return e
		}
		saved.ProcurementExecutiveID, e = purchasing.Owner(ctx, id)
		return e
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}
func (s *Service) History(ctx context.Context, token, id string) ([]domain.Assignment, error) {
	p, err := s.actor(ctx, token)
	if err != nil {
		return nil, err
	}
	if id, err = domain.ID(id); err != nil {
		return nil, err
	}
	var result []domain.Assignment
	err = s.unit.Within(ctx, func(ctx context.Context, r ports.Repository, _ ports.Clients, _ ports.Users, _ ports.Calendar, purchasing ports.Purchasing) error {
		project, e := r.Get(ctx, id, false)
		if e != nil {
			return e
		}
		if project == nil {
			return domain.ErrMissing
		}
		if allowed, e := canReadProject(ctx, r, p, *project); e != nil {
			return e
		} else if !allowed {
			return domain.ErrForbidden
		}
		result, e = r.History(ctx, id)
		return e
	})
	return result, err
}

func calculate(ctx context.Context, c domain.Capture, calendar ports.Calendar) (domain.Capture, error) {
	var w *domain.WorkingCount
	if c.ActualStartDate != "" && c.PlannedCompletionDate != "" {
		v, e := calendar.Count(ctx, c.ActualStartDate, c.PlannedCompletionDate)
		if e != nil {
			return c, e
		}
		w = &v
	}
	c = c.WithWorkingCount(w)
	return c, nil
}
func (s *Service) Preview(ctx context.Context, token string, c domain.Capture) (domain.Calculation, error) {
	p, e := s.actor(ctx, token)
	if e != nil {
		return domain.Calculation{}, e
	}
	if !p.Allows("projects.master.write") || !p.Allows("projects.portfolio.write") {
		return domain.Calculation{}, domain.ErrForbidden
	}
	c, e = c.ValidateSchedule()
	if e != nil {
		return domain.Calculation{}, e
	}
	var out domain.Capture
	e = s.unit.Within(ctx, func(ctx context.Context, _ ports.Repository, _ ports.Clients, _ ports.Users, calendar ports.Calendar, purchasing ports.Purchasing) error {
		var e error
		out, e = calculate(ctx, c, calendar)
		return e
	})
	return out.Calculation, e
}
func Recalculate(ctx context.Context, r ports.Repository, calendar ports.Calendar, actor string) (int, error) {
	rows, e := r.RecalculationProjects(ctx)
	if e != nil {
		return 0, e
	}
	for _, p := range rows {
		c, e := calculate(ctx, p.Capture, calendar)
		if e != nil {
			return 0, e
		}
		p.Capture = c
		if e = r.SaveCalculation(ctx, p, actor); e != nil {
			return 0, e
		}
	}
	return len(rows), nil
}

func (s *Service) Metadata() *MetadataService { return NewMetadata(s.unit, s.auth) }
