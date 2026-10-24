package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type IndentsService struct {
	u ports.IndentsUnitOfWork
	a ports.IndentsAuthorizer
	v ports.IndentsValues
}

func NewIndents(u ports.IndentsUnitOfWork, a ports.IndentsAuthorizer, v ports.IndentsValues) *IndentsService {
	return &IndentsService{u, a, v}
}
func (s *IndentsService) actor(ctx context.Context, t string, actions ...string) (domain.Principal, error) {
	p, e := s.a.Resolve(ctx, t)
	if e != nil {
		return p, e
	}
	if !p.Active || p.ID == "" {
		return p, domain.IndentsUnauthorized
	}
	for _, permission := range actions {
		if !p.Can(permission) {
			return p, domain.IndentsForbidden
		}
	}
	return p, nil
}

func (s *IndentsService) List(ctx context.Context, t, project string) (out []domain.IndentsRecord, e error) {
	defer func() {
		if e != nil {
			out = []domain.IndentsRecord(nil)
		}
	}()
	p, e := s.actor(ctx, t, "projects.purchase_indents.read")
	if e != nil {
		return
	}
	if project, e = domain.IndentsID(project); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.IndentsRepository, c ports.IndentsContext) error {
		if e := c.Scope(ctx, p, project, "projects.purchase_indents.read", false); e != nil {
			return e
		}
		out, e = r.List(ctx, project)
		return e
	})
	return
}
func (s *IndentsService) History(ctx context.Context, t, project, id string) (out []domain.IndentsEvent, e error) {
	defer func() {
		if e != nil {
			out = []domain.IndentsEvent(nil)
		}
	}()
	p, e := s.actor(ctx, t, "projects.purchase_indents.read")
	if e != nil {
		return
	}
	if project, e = domain.IndentsID(project); e != nil {
		return
	}
	if id, e = domain.IndentsID(id); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.IndentsRepository, c ports.IndentsContext) error {
		if e := c.Scope(ctx, p, project, "projects.purchase_indents.read", false); e != nil {
			return e
		}
		record, e := r.Get(ctx, project, id)
		if e != nil {
			return e
		}
		if record == nil {
			return domain.IndentsMissing
		}
		out, e = r.History(ctx, project, id)
		return e
	})
	return
}

func (s *IndentsService) Save(ctx context.Context, t, project, id string, input domain.IndentsSave) (out domain.IndentsRecord, e error) {
	defer func() {
		if e != nil {
			out = domain.IndentsRecord{}
		}
	}()
	p, e := s.actor(ctx, t, "projects.purchase_indents.write")
	if e != nil {
		return
	}
	if project, e = domain.IndentsID(project); e != nil {
		return
	}
	if id != "" {
		if id, e = domain.IndentsID(id); e != nil {
			return
		}
	}
	input, e = input.Validate()
	if e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.IndentsRepository, c ports.IndentsContext) error {
		if e := c.Scope(ctx, p, project, "projects.purchase_indents.write", true); e != nil {
			return e
		}
		if e := c.Validate(ctx, project, input.Data); e != nil {
			return e
		}
		var before *domain.IndentsRecord
		record := domain.IndentsRecord{ID: id, ProjectID: project, Active: true, Version: 1}
		action := "create"
		number := domain.IndentsNumber(s.v.ID())
		if id != "" {
			existing, e := r.Get(ctx, project, id)
			if e != nil {
				return e
			}
			if existing == nil {
				return domain.IndentsMissing
			}
			if existing.Version != input.Version {
				return domain.IndentsStale
			}
			if existing.Data.State != "draft" {
				return domain.IndentsInvalid("", "Only draft indents can be edited")
			}
			copy := domain.IndentsCopy(*existing)
			before = &copy
			record.Version = existing.Version + 1
			number = existing.Data.Number
			action = "edit"
		} else {
			if input.Version != 1 {
				return domain.IndentsStale
			}
			record.ID = s.v.ID()
		}
		for _, line := range input.Data.Lines {
			source, e := c.Line(ctx, project, line.DemandID)
			if e != nil {
				return e
			}
			if !source.Active {
				return domain.IndentsInvalid("demand_id", "Select an active project requirement")
			}
			q, e := domain.IndentsQuantity(line.Quantity)
			if e != nil {
				return e
			}
			total, e := domain.IndentsQuantity(source.Quantity)
			if e != nil {
				return e
			}
			if q > total {
				return domain.IndentsInvalid("quantity", "Requested quantity exceeds project requirement")
			}
		}
		record.Data = domain.IndentsData{IndentsInput: input.Data, Number: number, State: "draft", Released: []domain.IndentsRelease{}}
		out, e = r.Save(ctx, record, p.ID)
		if e != nil {
			return e
		}
		return r.Event(ctx, domain.IndentsEvent{ID: s.v.ID(), SubjectID: record.ID, Action: action, Reason: input.Reason, Before: before, After: out, ActorID: p.ID, CreatedAt: out.UpdatedAt})
	})
	return
}
func (s *IndentsService) Transition(ctx context.Context, t, project, id string, input domain.IndentsAction) (out domain.IndentsRecord, e error) {
	defer func() {
		if e != nil {
			out = domain.IndentsRecord{}
		}
	}()
	input, e = input.Validate()
	if e != nil {
		return
	}
	permission := "projects.purchase_indents.write"
	if input.Action == "review" || input.Action == "return" {
		permission = "projects.purchase_indents.review"
	}
	if input.Action == "release" {
		permission = "projects.purchase_indents.release"
	}
	p, e := s.actor(ctx, t, permission)
	if e != nil {
		return
	}
	if project, e = domain.IndentsID(project); e != nil {
		return
	}
	if id, e = domain.IndentsID(id); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.IndentsRepository, c ports.IndentsContext) error {
		if e := c.Scope(ctx, p, project, permission, true); e != nil {
			return e
		}
		record, e := r.Get(ctx, project, id)
		if e != nil {
			return e
		}
		if record == nil {
			return domain.IndentsMissing
		}
		if record.Version != input.Version {
			return domain.IndentsStale
		}
		before := domain.IndentsCopy(*record)
		state := record.Data.State
		now := s.v.Now()
		switch input.Action {
		case "submit":
			if state != "draft" {
				return domain.IndentsInvalid("", "Only drafts may be submitted")
			}
			if e := c.Validate(ctx, project, record.Data.IndentsInput); e != nil {
				return e
			}
			record.Data.State = "submitted"
		case "return":
			if state != "submitted" && state != "reviewed" {
				return domain.IndentsInvalid("", "Only unreleased review work may be returned")
			}
			record.Data.State = "draft"
			record.Data.Review = nil
		case "review":
			if state != "submitted" {
				return domain.IndentsInvalid("", "Submit before review")
			}
			if e := c.Validate(ctx, project, record.Data.IndentsInput); e != nil {
				return e
			}
			versions := map[string]int32{}
			for _, line := range record.Data.Lines {
				source, e := c.Line(ctx, project, line.DemandID)
				if e != nil {
					return e
				}
				if !source.Active {
					return domain.IndentsInvalid("demand_id", "Requirement is inactive")
				}
				versions[line.DemandID] = source.DemandVersion
			}
			record.Data.Review = &domain.IndentsReview{ActorID: p.ID, At: now, Evidence: input.Evidence, DemandVersions: versions}
			record.Data.State = "reviewed"
		case "release":
			if state != "reviewed" && state != "part_released" {
				return domain.IndentsInvalid("", "Reviewed demand is required before release")
			}
			if e := c.Validate(ctx, project, record.Data.IndentsInput); e != nil {
				return e
			}
			if len(input.LineIDs) == 0 || record.Data.Review == nil {
				return domain.IndentsInvalid("line_ids", "Select distinct reviewed lines for release")
			}
			lines := map[string]domain.IndentsLine{}
			for _, line := range record.Data.Lines {
				lines[line.DemandID] = line
			}
			prior := map[string]bool{}
			for _, line := range record.Data.Released {
				prior[line.DemandID] = true
			}
			all, e := r.List(ctx, project)
			if e != nil {
				return e
			}
			additions := []domain.IndentsRelease{}
			for _, demand := range input.LineIDs {
				line, exists := lines[demand]
				if !exists || prior[demand] {
					return domain.IndentsInvalid("line_ids", "Unknown or previously released line")
				}
				source, e := c.Line(ctx, project, demand)
				if e != nil {
					return e
				}
				if record.Data.Review.DemandVersions[demand] != source.DemandVersion {
					return domain.IndentsInvalid("demand_id", "Project requirement changed after review; create a revised indent for remaining demand")
				}
				committed := int64(0)
				for _, other := range all {
					for _, released := range other.Data.Released {
						if released.DemandID == demand {
							q, e := domain.IndentsQuantity(released.Quantity)
							if e != nil {
								return e
							}
							if committed > int64(9223372036854775807)-q {
								return domain.IndentsInvalid("quantity", "Released quantity exceeds supported requirement")
							}
							committed += q
						}
					}
				}
				if e := domain.IndentsReleaseGuard(line, source, committed); e != nil {
					return e
				}
				additions = append(additions, domain.IndentsRelease{IndentsLine: line, At: now, ActorID: p.ID, Evidence: input.Evidence, Source: source})
			}
			record.Data.Released = append(record.Data.Released, additions...)
			record.Data.State = "part_released"
			if len(record.Data.Released) == len(record.Data.Lines) {
				record.Data.State = "released"
			}
		case "withdraw":
			if state == "released" {
				return domain.IndentsInvalid("", "Released commitments cannot be withdrawn here")
			}
			if state == "withdrawn" {
				return domain.IndentsInvalid("", "Indent already withdrawn")
			}
			record.Data.State = "withdrawn"
			record.Active = false
		}
		record.Version++
		out, e = r.Save(ctx, *record, p.ID)
		if e != nil {
			return e
		}
		return r.Event(ctx, domain.IndentsEvent{ID: s.v.ID(), SubjectID: id, Action: input.Action, Reason: input.Reason, Before: &before, After: out, ActorID: p.ID, CreatedAt: out.UpdatedAt})
	})
	return
}
