package application

import (
	"context"
	"errors"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type BOService struct {
	u       ports.BOUnitOfWork
	a       ports.BOAuthorizer
	v       ports.BOValues
	printer ports.BOPrinter
}

func NewBO(u ports.BOUnitOfWork, a ports.BOAuthorizer, v ports.BOValues, printers ...ports.BOPrinter) *BOService {
	out := &BOService{u: u, a: a, v: v}
	if len(printers) > 0 {
		out.printer = printers[0]
	}
	return out
}
func (s *BOService) actor(ctx context.Context, t string, actions ...string) (domain.Principal, error) {
	p, e := s.a.Resolve(ctx, t)
	if e != nil {
		return p, e
	}
	if !p.Active || p.ID == "" {
		return p, domain.BOUnauthorized
	}
	for _, permission := range actions {
		if !p.Can(permission) {
			return p, domain.BOForbidden
		}
	}
	return p, nil
}
func (s *BOService) List(ctx context.Context, t, project string) (out domain.BOListing, e error) {
	p, e := s.actor(ctx, t, "projects.purchase_bo.read")
	if e != nil {
		return
	}
	if project, e = domain.BOID(project); e != nil {
		return
	}
	out.Lists = []domain.BORecord{}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.BORepository, c ports.BOContext) error {
		if e := c.Scope(ctx, p, project, "projects.purchase_bo.read", false); e != nil {
			return e
		}
		out.Lists, e = r.List(ctx, project)
		if e != nil {
			return e
		}
		if p.Can("projects.purchase_bo.write") {
			out.CanWrite, e = bOWriteAvailable(ctx, c, p, project, "projects.purchase_bo.write")
			if e != nil {
				return e
			}
		}
		if p.Can("projects.purchase_bo.complete") && p.Can("projects.items.write") {
			out.CanComplete, e = bOWriteAvailable(ctx, c, p, project, "projects.purchase_bo.complete")
			if e != nil {
				return e
			}
		}
		return nil
	})
	return
}
func (s *BOService) History(ctx context.Context, t, project, id string) (out []domain.BOEvent, e error) {
	p, e := s.actor(ctx, t, "projects.purchase_bo.read")
	if e != nil {
		return
	}
	if project, e = domain.BOID(project); e != nil {
		return
	}
	if id, e = domain.BOID(id); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.BORepository, c ports.BOContext) error {
		if e := c.Scope(ctx, p, project, "projects.purchase_bo.read", false); e != nil {
			return e
		}
		v, e := r.Get(ctx, project, id)
		if e != nil {
			return e
		}
		if v == nil {
			return domain.BOMissing
		}
		out, e = r.History(ctx, project, id)
		return e
	})
	return
}
func (s *BOService) Save(ctx context.Context, t, project, id string, input domain.BOSave) (out domain.BORecord, e error) {
	p, e := s.actor(ctx, t, "projects.purchase_bo.read", "projects.purchase_bo.write")
	if e != nil {
		return
	}
	if project, e = domain.BOID(project); e != nil {
		return
	}
	if id, e = domain.BOID(id); e != nil {
		return
	}
	input, e = input.Validate()
	if e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.BORepository, c ports.BOContext) error {
		if e := c.Scope(ctx, p, project, "projects.purchase_bo.write", true); e != nil {
			return e
		}
		record, e := r.Get(ctx, project, id)
		if e != nil {
			return e
		}
		if record == nil {
			if input.Version != 0 {
				return domain.BOStale
			}
			record = &domain.BORecord{ID: id, ProjectID: project, PreparerID: p.ID, State: "draft", Active: true, Version: 0, Lines: []domain.BOLine{}}
		}
		if e := domain.BOEditable(*record, p, input.Version); e != nil {
			return e
		}
		before := domain.BOCopy(*record)
		var prior *domain.BORecord
		if record.Version > 0 {
			prior = &before
		}
		if input.Name != nil {
			record.Name = *input.Name
		}
		lists, e := r.List(ctx, project)
		if e != nil {
			return e
		}
		used := map[string]bool{}
		for _, other := range lists {
			if other.ID != id && other.Active {
				for _, line := range other.Lines {
					if line.ItemID != nil {
						used[line.Snapshot.EngineeringKey] = true
					}
				}
			}
		}
		previous := map[string]domain.BOLine{}
		incoming := map[string]bool{}
		for _, line := range record.Lines {
			previous[line.ID] = line
		}
		for _, line := range input.Lines {
			incoming[line.ID] = true
		}
		for _, line := range record.Lines {
			if line.DemandID != nil && !incoming[line.ID] {
				return domain.BOInvalid("lines", "Previously completed items cannot be removed")
			}
		}
		lines := []domain.BOLine{}
		seen := map[string]bool{}
		for _, raw := range input.Lines {
			line := domain.BOLine{ID: raw.ID, ItemID: raw.ItemID, Quantity: raw.Quantity, RequiredDate: raw.RequiredDate, Reference: raw.Reference, Notes: raw.Notes}
			old, exists := previous[line.ID]
			if exists && old.DemandID != nil {
				if line.ItemID == nil || old.ItemID == nil || *line.ItemID != *old.ItemID {
					return domain.BOInvalid("item_id", "A previously completed item's SKU cannot be replaced")
				}
				line.DemandID = old.DemandID
			}
			if line.ItemID != nil {
				snapshot, e := c.Item(ctx, project, *line.ItemID)
				if e != nil {
					return e
				}
				line.Snapshot = snapshot
				if seen[snapshot.EngineeringKey] || used[snapshot.EngineeringKey] {
					return domain.BOInvalid("item_id", "This item already exists in the project's BO lists; edit the existing requirement")
				}
				seen[snapshot.EngineeringKey] = true
			}
			if line.DemandID != nil {
				line.Snapshot.DemandVersion = old.Snapshot.DemandVersion
			}
			if e := domain.BOValidateLine(line, false); e != nil {
				return e
			}
			lines = append(lines, line)
		}
		record.Lines = lines
		record.Version++
		out, e = r.Save(ctx, *record, p.ID)
		if e != nil {
			return e
		}
		return r.Event(ctx, domain.BOEvent{ID: s.v.ID(), SubjectID: id, Action: "save_draft", Reason: input.Reason, Before: prior, After: out, ActorID: p.ID, CreatedAt: out.UpdatedAt})
	})
	return
}
func (s *BOService) Change(ctx context.Context, t, project, id, action string, input domain.BOChange) (out domain.BORecord, e error) {
	permissions := []string{"projects.purchase_bo.read", "projects.purchase_bo.complete", "projects.items.write"}
	if action == "reopen" {
		permissions = append(permissions, "projects.purchase_bo.write")
	}
	p, e := s.actor(ctx, t, permissions...)
	if e != nil {
		return
	}
	if project, e = domain.BOID(project); e != nil {
		return
	}
	if id, e = domain.BOID(id); e != nil {
		return
	}
	input.Reason, e = domain.BOReason(input.Reason)
	if e != nil {
		return
	}
	if input.Version < 1 {
		return out, domain.BOInvalid("version", "Use the saved BO version")
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.BORepository, c ports.BOContext) error {
		out, e = s.changeRecord(ctx, r, c, p, project, id, action, input)
		return e
	})
	return
}

func bOWriteAvailable(ctx context.Context, c ports.BOContext, a domain.Principal, project, permission string) (bool, error) {
	e := c.Scope(ctx, a, project, permission, true)
	if e == nil {
		return true, nil
	}
	var rejected *domain.BOError
	if errors.As(e, &rejected) && (rejected.Status == 403 || rejected.Status == 422) {
		return false, nil
	}
	return false, e
}

func (s *BOService) changeRecord(ctx context.Context, r ports.BORepository, c ports.BOContext, p domain.Principal, project, id, action string, input domain.BOChange) (out domain.BORecord, e error) {
	work := func() error {
		if e := c.Scope(ctx, p, project, "projects.purchase_bo.complete", true); e != nil {
			return e
		}
		if e := c.Scope(ctx, p, project, "projects.items.write", true); e != nil {
			return e
		}
		record, e := r.Get(ctx, project, id)
		if e != nil {
			return e
		}
		if record == nil {
			return domain.BOMissing
		}
		if action == "complete" && record.State == "completed" && record.CompletedBy != nil && *record.CompletedBy == p.ID && input.Version == record.Version-1 {
			out = *record
			return nil
		}
		before := domain.BOCopy(*record)
		if action == "reopen" {
			if record.State != "completed" {
				return domain.BOInvalid("", "Only completed BO lists can be reopened")
			}
			record.State = "draft"
		}
		if e := domain.BOEditable(*record, p, input.Version); e != nil {
			return e
		}
		switch action {
		case "reopen":
			for i, line := range record.Lines {
				version, e := c.DemandVersion(ctx, project, line)
				if e != nil {
					return e
				}
				record.Lines[i].Snapshot.DemandVersion = &version
			}
			record.CompletedAt = nil
			record.CompletedBy = nil
		case "complete":
			if len(record.Lines) == 0 {
				return domain.BOInvalid("lines", "Add at least one item before completing the BO list")
			}
			keys := map[string]bool{}
			for i, line := range record.Lines {
				if e := domain.BOValidateLine(line, true); e != nil {
					return e
				}
				snapshot, e := c.Item(ctx, project, *line.ItemID)
				if e != nil {
					return e
				}
				snapshot.DemandVersion = line.Snapshot.DemandVersion
				line.Snapshot = snapshot
				if e := domain.BOValidateLine(line, true); e != nil {
					return e
				}
				if keys[snapshot.EngineeringKey] {
					return domain.BOInvalid("item_id", "Duplicate engineering requirement; combine quantities or use location allocations")
				}
				keys[snapshot.EngineeringKey] = true
				record.Lines[i] = line
			}
			for i, line := range record.Lines {
				id, e := c.Publish(ctx, project, line, p.ID, input.Reason)
				if e != nil {
					return e
				}
				record.Lines[i].DemandID = &id
			}
			record.State = "completed"
			now := s.v.Now()
			record.CompletedAt = &now
			record.CompletedBy = &p.ID
		default:
			return domain.BOInvalid("action", "Choose complete or reopen")
		}
		record.Version++
		out, e = r.Save(ctx, *record, p.ID)
		if e != nil {
			return e
		}
		return r.Event(ctx, domain.BOEvent{ID: s.v.ID(), SubjectID: id, Action: action + "_bo_list", Reason: input.Reason, Before: &before, After: out, ActorID: p.ID, CreatedAt: out.UpdatedAt})
	}
	e = work()
	return
}

func (s *BOService) ChangeRegister(ctx context.Context, t, project, action string, in domain.BORegisterChange) (out domain.BOListing, e error) {
	p, e := s.actor(ctx, t, "projects.purchase_bo.read", "projects.purchase_bo.write", "projects.purchase_bo.complete", "projects.items.write")
	if e != nil {
		return
	}
	if project, e = domain.BOID(project); e != nil {
		return
	}
	if in.Reason, e = domain.BOReason(in.Reason); e != nil {
		return
	}
	if action != "complete" && action != "reopen" {
		return out, domain.BOInvalid("action", "Choose complete or reopen")
	}
	if len(in.Records) == 0 || len(in.Records) > 1000 {
		return out, domain.BOInvalid("records", "Provide the current saved register")
	}
	expected := map[string]int32{}
	for _, v := range in.Records {
		if _, e := domain.BOID(v.ID); e != nil {
			return out, e
		}
		if v.Version < 1 || expected[v.ID] != 0 {
			return out, domain.BOInvalid("records", "Provide unique saved records and versions")
		}
		expected[v.ID] = v.Version
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.BORepository, c ports.BOContext) error {
		if e := c.Scope(ctx, p, project, "projects.purchase_bo.complete", true); e != nil {
			return e
		}
		records, e := r.List(ctx, project)
		if e != nil {
			return e
		}
		if len(records) != len(expected) {
			return domain.BOStale
		}
		target := 0
		for _, record := range records {
			if expected[record.ID] != record.Version {
				return domain.BOStale
			}
			if (action == "complete" && record.State == "draft") || (action == "reopen" && record.State == "completed") {
				target++
				copy := domain.BOCopy(record)
				copy.State = "draft"
				if e := domain.BOEditable(copy, p, record.Version); e != nil {
					return e
				}
			}
		}
		if target == 0 {
			return domain.BOInvalid("records", "The register is already in this state")
		}
		for _, record := range records {
			if (action == "complete" && record.State == "draft") || (action == "reopen" && record.State == "completed") {
				if _, e := s.changeRecord(ctx, r, c, p, project, record.ID, action, domain.BOChange{Version: record.Version, Reason: in.Reason}); e != nil {
					return e
				}
			}
		}
		out.Lists, e = r.List(ctx, project)
		out.CanWrite = true
		out.CanComplete = true
		return e
	})
	return
}
func (s *BOService) Print(ctx context.Context, t, project string) (data []byte, code string, e error) {
	p, e := s.actor(ctx, t, "projects.purchase_bo.read")
	if e != nil {
		return
	}
	if project, e = domain.BOID(project); e != nil {
		return
	}
	if s.printer == nil {
		return nil, "", &domain.BOError{Status: 503, Message: "PDF rendering is unavailable"}
	}
	snapshot := domain.BOPrint{GeneratedAt: s.v.Now()}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.BORepository, c ports.BOContext) error {
		if e := c.Scope(ctx, p, project, "projects.purchase_bo.read", false); e != nil {
			return e
		}
		var err error
		snapshot.Project, err = c.Project(ctx, project)
		if err != nil {
			return err
		}
		snapshot.Records, err = r.List(ctx, project)
		if err != nil {
			return err
		}
		for _, record := range snapshot.Records {
			events, err := r.History(ctx, project, record.ID)
			if err != nil {
				return err
			}
			for _, event := range events {
				if event.Action == "complete_bo_list" || event.Action == "reopen_bo_list" {
					snapshot.Comments = append(snapshot.Comments, event.CreatedAt.Format("2006-01-02 15:04 MST")+" - "+event.Action+": "+event.Reason)
					break
				}
			}
		}
		return nil
	})
	if e != nil {
		return
	}
	code = snapshot.Project.Number
	data, e = s.printer.Render(ctx, snapshot)
	return
}
