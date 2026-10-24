package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type IndentGroupsService struct {
	u ports.IndentGroupsUnitOfWork
	a ports.IndentGroupsAuthorizer
	v ports.IndentGroupsValues
}

func NewIndentGroups(u ports.IndentGroupsUnitOfWork, a ports.IndentGroupsAuthorizer, v ports.IndentGroupsValues) *IndentGroupsService {
	return &IndentGroupsService{u, a, v}
}
func (s *IndentGroupsService) actor(ctx context.Context, t string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, t)
	if e != nil {
		return a, e
	}
	if !a.Active || a.ID == "" {
		return a, domain.IndentGroupsUnauthorized
	}
	if !a.Can("projects.purchase_bo.read") || !a.Can("projects.purchase_indent_groups.read") {
		return a, domain.IndentGroupsForbidden
	}
	return a, nil
}
func (s *IndentGroupsService) Workspace(ctx context.Context, t, project, bo string) (out domain.IndentGroupsWorkspace, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return
	}
	allowed := false
	for _, p := range []string{"projects.purchase_indent_groups.view", "projects.purchase_group_vendors.view", "projects.purchase_item_vendor_report.view", "projects.purchase_indent_register.view", "projects.purchase_indent_documents.view"} {
		if a.Can(p) {
			allowed = true
		}
	}
	if !allowed {
		return out, domain.IndentGroupsForbidden
	}
	if project, e = domain.IndentGroupsID(project); e != nil {
		return
	}
	if bo, e = domain.IndentGroupsID(bo); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.IndentGroupsRepository, c ports.IndentGroupsContext) error {
		write, e := c.Scope(ctx, a, project, "projects.purchase_indent_groups.read", false)
		if e != nil {
			return e
		}
		out, e = s.workspace(ctx, r, c, a, project, bo, write)
		return e
	})
	return
}
func (s *IndentGroupsService) workspace(ctx context.Context, r ports.IndentGroupsRepository, c ports.IndentGroupsContext, a domain.Principal, project, bo string, write bool) (domain.IndentGroupsWorkspace, error) {
	source, e := c.Source(ctx, project, bo)
	if e != nil {
		return domain.IndentGroupsWorkspace{}, e
	}
	record, e := r.Get(ctx, project, bo)
	if e != nil {
		return domain.IndentGroupsWorkspace{}, e
	}
	current, e := c.Vendors(ctx)
	if e != nil {
		return domain.IndentGroupsWorkspace{}, e
	}
	vendors := domain.IndentGroupsCandidates(record, current)
	out := domain.IndentGroupsWorkspace{Record: record, Vendors: vendors, CanWrite: write && source.Available && a.Can("projects.purchase_indent_groups.write"), CanManageVendors: write && source.Available && a.Can("projects.purchase_group_vendors.write"), History: []domain.IndentGroupsEvent{}, ItemReport: domain.IndentGroupsReport(record, source.Lines, vendors)}
	if record != nil {
		out.History, e = r.History(ctx, project, bo)
	}
	return out, e
}
func (s *IndentGroupsService) groups(raw []domain.IndentGroupsGroupInput) []domain.IndentGroupsGroup {
	out := []domain.IndentGroupsGroup{}
	for _, g := range raw {
		out = append(out, domain.IndentGroupsGroup{IndentGroupsGroupInput: g, NameKey: s.v.NameKey(g.Name), Invitations: []domain.IndentGroupsInvitation{}, ScopeRevision: 1})
	}
	return out
}
func (s *IndentGroupsService) Execute(ctx context.Context, t, project, bo string, input domain.IndentGroupsCommand) (out domain.IndentGroupsWorkspace, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return
	}
	if project, e = domain.IndentGroupsID(project); e != nil {
		return
	}
	if bo, e = domain.IndentGroupsID(bo); e != nil {
		return
	}
	input, e = input.Validate()
	if e != nil {
		return
	}
	permission := "projects.purchase_indent_groups.write"
	if input.Action == "set_vendors" || input.Action == "add_vendors" {
		permission = "projects.purchase_group_vendors.write"
	}
	if input.Action == "invite" {
		permission = "projects.purchase_indent_documents.dispatch"
	}
	issueOnly := input.Action == "create" && !a.Can(permission) && a.Can("projects.purchase_indent_documents.generate")
	if issueOnly {
		permission = "projects.purchase_indent_documents.generate"
	}
	if !a.Can(permission) {
		return out, domain.IndentGroupsForbidden
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.IndentGroupsRepository, c ports.IndentGroupsContext) error {
		write, e := c.Scope(ctx, a, project, permission, true)
		if e != nil {
			return e
		}
		source, e := c.Source(ctx, project, bo)
		if e != nil {
			return e
		}
		if !source.Available {
			return domain.IndentGroupsInvalid("", "Project purchasing is unavailable, closed or on hold")
		}
		record, e := r.Get(ctx, project, bo)
		if e != nil {
			return e
		}
		var before *domain.IndentGroupsRecord
		if record == nil {
			record = &domain.IndentGroupsRecord{BOID: bo, ProjectID: project, State: "draft", Active: true, Groups: []domain.IndentGroupsGroup{}, SourceLines: []domain.IndentGroupsLine{}, SourceVendors: []domain.IndentGroupsVendor{}, SelectedLineIDs: []string{}}
		} else {
			copy := domain.IndentGroupsCopy(*record)
			before = &copy
		}
		if input.Version != record.Version {
			return domain.IndentGroupsStale
		}
		for i, g := range record.Groups {
			record.Groups[i].NameKey = s.v.NameKey(g.Name)
		}
		candidates, e := c.Vendors(ctx)
		if e != nil {
			return e
		}
		vendors := domain.IndentGroupsVendorIDs(candidates)
		lines := domain.IndentGroupsLineIDs(source.Lines)
		find := func(id *string) *domain.IndentGroupsGroup {
			if id == nil {
				return nil
			}
			for i := range record.Groups {
				if record.Groups[i].ID == *id {
					return &record.Groups[i]
				}
			}
			return nil
		}
		if (input.Action == "save" || input.Action == "create" || input.Action == "save_groups") && len(input.Groups) > 0 {
			for _, g := range input.Groups {
				old := find(&g.ID)
				changed := old == nil && len(g.VendorIDs) > 0 || old != nil && !domain.IndentGroupsOrderedEqual(old.VendorIDs, g.VendorIDs)
				if changed && !a.Can("projects.purchase_group_vendors.write") {
					return domain.IndentGroupsForbidden
				}
			}
			if issueOnly {
				if len(input.Groups) != len(record.Groups) {
					return domain.IndentGroupsForbidden
				}
				for i, g := range input.Groups {
					old := record.Groups[i]
					if g.ID != old.ID || g.Name != old.Name || !domain.IndentGroupsOrderedEqual(g.LineIDs, old.LineIDs) || !domain.IndentGroupsOrderedEqual(g.VendorIDs, old.VendorIDs) {
						return domain.IndentGroupsForbidden
					}
				}
			}
		}
		issued := record.State == "created"
		switch input.Action {
		case "create_group", "add_items", "reset_item":
			if issued && input.Action == "reset_item" {
				return domain.IndentGroupsInvalid("", "Issued items cannot be reset; append new items instead")
			}
			if issued && source.State != "completed" {
				return domain.IndentGroupsInvalid("", "Complete the updated BO before adding items")
			}
			if input.BOVersion != source.Version {
				return domain.IndentGroupsStale
			}
			additions := []string{}
			if input.Action == "create_group" {
				if len(input.Groups) != 1 || len(input.Groups[0].VendorIDs) > 0 {
					return domain.IndentGroupsInvalid("groups", "Save one item group before choosing its vendors")
				}
				if len(record.Groups) >= 100 {
					return domain.IndentGroupsInvalid("groups", "At most 100 item groups are supported")
				}
				group := s.groups(input.Groups)[0]
				allowedLines := append(append([]string{}, lines...), domain.IndentGroupsLineIDs(record.SourceLines)...)
				allowedVendors := append(append([]string{}, vendors...), domain.IndentGroupsGroupVendors(record.Groups)...)
				if e := domain.IndentGroupsValidateGroups(append(append([]domain.IndentGroupsGroup{}, record.Groups...), group), allowedLines, allowedVendors, false); e != nil {
					return e
				}
				record.Groups = append(record.Groups, group)
				additions = group.LineIDs
			} else if input.Action == "add_items" {
				group := find(input.GroupID)
				if group == nil {
					return domain.IndentGroupsMissing
				}
				additions = input.LineIDs
				if len(additions) == 0 || !domain.IndentGroupsDistinct(additions) || !domain.IndentGroupsSubset(additions, lines) {
					return domain.IndentGroupsInvalid("line_ids", "Select distinct items from the current BO list")
				}
				assigned := domain.IndentGroupsGroupLines(record.Groups)
				for _, id := range additions {
					if domain.IndentGroupsContains(assigned, id) {
						return domain.IndentGroupsInvalid("line_ids", "Selected items already belong to a group")
					}
				}
				if len(group.LineIDs)+len(additions) > 100 {
					return domain.IndentGroupsInvalid("line_ids", "A group supports at most 100 items")
				}
				if issued {
					group.ScopeRevision++
				}
				group.LineIDs = append(group.LineIDs, additions...)
			} else {
				if input.LineID == nil {
					return domain.IndentGroupsInvalid("line_id", "Select a grouped item")
				}
				found := false
				groups := []domain.IndentGroupsGroup{}
				for _, g := range record.Groups {
					retained := []string{}
					for _, id := range g.LineIDs {
						if id == *input.LineID {
							found = true
						} else {
							retained = append(retained, id)
						}
					}
					g.LineIDs = retained
					if len(retained) > 0 {
						groups = append(groups, g)
					}
				}
				if !found {
					return domain.IndentGroupsMissing
				}
				record.Groups = groups
			}
			record.SelectedLineIDs = domain.IndentGroupsGroupLines(record.Groups)
			record.BOVersion = source.Version
			if issued {
				frozen := domain.IndentGroupsLineIDs(record.SourceLines)
				for _, line := range source.Lines {
					if domain.IndentGroupsContains(additions, line.ID) && !domain.IndentGroupsContains(frozen, line.ID) {
						record.SourceLines = append(record.SourceLines, line)
					}
				}
			} else {
				record.State = "draft"
				if len(record.Groups) > 0 {
					record.State = "grouped"
				}
			}
		case "select_items":
			if issued {
				return domain.IndentGroupsInvalid("", "Created item selection is immutable")
			}
			if input.BOVersion != source.Version {
				return domain.IndentGroupsStale
			}
			ids := input.SelectedLineIDs
			if len(ids) == 0 || !domain.IndentGroupsDistinct(ids) || !domain.IndentGroupsSubset(ids, lines) {
				return domain.IndentGroupsInvalid("selected_line_ids", "Select distinct items from the current BO list")
			}
			if !domain.IndentGroupsSubset(domain.IndentGroupsGroupLines(record.Groups), ids) {
				return domain.IndentGroupsInvalid("selected_line_ids", "Remove items from saved groups before deselecting them")
			}
			record.SelectedLineIDs = ids
			record.BOVersion = source.Version
			record.State = "draft"
		case "save_groups":
			if issued {
				return domain.IndentGroupsInvalid("", "Created group scope is immutable")
			}
			if input.BOVersion != source.Version {
				return domain.IndentGroupsStale
			}
			if len(record.SelectedLineIDs) == 0 {
				return domain.IndentGroupsInvalid("", "Save the purchase item selection first")
			}
			groups := s.groups(input.Groups)
			if e := domain.IndentGroupsValidateGroups(groups, record.SelectedLineIDs, vendors, false); e != nil {
				return e
			}
			if len(groups) == 0 || !domain.IndentGroupsSameSet(domain.IndentGroupsGroupLines(groups), record.SelectedLineIDs) {
				return domain.IndentGroupsInvalid("groups", "Assign every selected purchase item to a group")
			}
			for _, g := range groups {
				old := find(&g.ID)
				if len(g.VendorIDs) > 0 && (old == nil || !domain.IndentGroupsSameSet(old.LineIDs, g.LineIDs) || !domain.IndentGroupsOrderedEqual(old.VendorIDs, g.VendorIDs)) {
					return domain.IndentGroupsInvalid("vendor_ids", "Save groups first, then select vendors separately")
				}
			}
			record.Groups = groups
			record.State = "grouped"
			record.BOVersion = source.Version
		case "set_vendors", "add_vendors":
			group := find(input.GroupID)
			if group == nil {
				return domain.IndentGroupsMissing
			}
			ids := input.VendorIDs
			if !domain.IndentGroupsDistinct(ids) || !domain.IndentGroupsSubset(ids, vendors) {
				return domain.IndentGroupsInvalid("vendor_ids", "Choose distinct active vendors")
			}
			if input.Action == "set_vendors" {
				if record.State != "grouped" && !issued {
					return domain.IndentGroupsInvalid("", "Save all named groups before selecting vendors")
				}
				if input.BOVersion != source.Version {
					return domain.IndentGroupsStale
				}
				if issued && len(group.VendorIDs) > 0 {
					return domain.IndentGroupsInvalid("", "Issued vendor selections are retained; add vendors instead")
				}
				if len(ids) < 3 {
					return domain.IndentGroupsInvalid("vendor_ids", "Select at least three distinct active vendors")
				}
				group.VendorIDs = append([]string{}, ids...)
			} else {
				if !issued {
					return domain.IndentGroupsInvalid("", "Create grouped indents before adding later vendors")
				}
				if len(ids) == 0 {
					return domain.IndentGroupsInvalid("vendor_ids", "Choose vendors to add")
				}
				for _, id := range ids {
					if domain.IndentGroupsContains(group.VendorIDs, id) {
						return domain.IndentGroupsInvalid("vendor_ids", "Vendor already belongs to this group")
					}
				}
				if len(group.VendorIDs)+len(ids) > 50 {
					return domain.IndentGroupsInvalid("vendor_ids", "A group supports at most 50 vendors")
				}
				group.VendorIDs = append(group.VendorIDs, ids...)
			}
			if issued {
				saved := domain.IndentGroupsVendorIDs(record.SourceVendors)
				for _, v := range candidates {
					if domain.IndentGroupsContains(ids, v.ID) && !domain.IndentGroupsContains(saved, v.ID) {
						record.SourceVendors = append(record.SourceVendors, v)
					}
				}
			}
		case "invite":
			if !issued {
				return domain.IndentGroupsInvalid("", "Create grouped indents before recording an invitation")
			}
			group := find(input.GroupID)
			if group == nil || input.VendorID == nil || !domain.IndentGroupsContains(group.VendorIDs, *input.VendorID) {
				return domain.IndentGroupsInvalid("vendor_id", "Select a vendor belonging to this group")
			}
			if !domain.IndentGroupsContains(vendors, *input.VendorID) {
				return domain.IndentGroupsInvalid("vendor_id", "Only active vendors can be invited")
			}
			revision := group.ScopeRevision
			if input.InvitationScopeRevision != nil {
				revision = *input.InvitationScopeRevision
			}
			coverage := group.LineIDs
			if input.InvitationLineIDs != nil {
				coverage = input.InvitationLineIDs
			}
			if revision < 1 || revision > group.ScopeRevision || len(coverage) == 0 || !domain.IndentGroupsSubset(coverage, group.LineIDs) {
				return domain.IndentGroupsInvalid("", "Invalid invitation scope")
			}
			for _, i := range group.Invitations {
				if i.VendorID == *input.VendorID && i.ScopeRevision == revision {
					return domain.IndentGroupsInvalid("vendor_id", "Invitation is already recorded for this group and vendor")
				}
			}
			if input.SentDate == nil || *input.SentDate > domain.IndentGroupsToday(s.v.Now()) || input.Channel == "" || input.DispatchReference == "" {
				return domain.IndentGroupsInvalid("sent_date", "Record a dispatch date, channel and reference; future dates are not allowed")
			}
			group.Invitations = append(group.Invitations, domain.IndentGroupsInvitation{VendorID: *input.VendorID, ScopeRevision: revision, LineIDs: append([]string{}, coverage...), SentDate: *input.SentDate, Channel: input.Channel, DispatchReference: input.DispatchReference, RecordedByID: a.ID, RecordedAt: s.v.Now()})
		case "save", "create":
			if issued {
				return domain.IndentGroupsInvalid("", "Created group scope is immutable; add vendors instead")
			}
			if input.BOVersion != source.Version {
				return domain.IndentGroupsStale
			}
			groups := s.groups(input.Groups)
			selection := record.SelectedLineIDs
			if len(selection) == 0 {
				selection = lines
			}
			if e := domain.IndentGroupsValidateGroups(groups, selection, vendors, input.Action == "create"); e != nil {
				return e
			}
			if input.Action == "create" && source.State != "completed" {
				return domain.IndentGroupsInvalid("", "Complete the BO list before creating grouped indents")
			}
			record.Groups = groups
			record.BOVersion = source.Version
			if len(record.SelectedLineIDs) == 0 {
				record.SelectedLineIDs = append([]string{}, lines...)
			}
			record.State = "draft"
			if input.Action == "create" {
				record.State = "created"
				record.SourceLines = []domain.IndentGroupsLine{}
				for _, line := range source.Lines {
					if domain.IndentGroupsContains(record.SelectedLineIDs, line.ID) {
						record.SourceLines = append(record.SourceLines, line)
					}
				}
				record.Number = domain.IndentGroupsNumber(bo)
				selected := domain.IndentGroupsGroupVendors(groups)
				record.SourceVendors = []domain.IndentGroupsVendor{}
				for _, v := range candidates {
					if domain.IndentGroupsContains(selected, v.ID) {
						record.SourceVendors = append(record.SourceVendors, v)
					}
				}
			}
		}
		record.Version++
		if before != nil {
			if guard, ok := r.(ports.IndentGroupsFreezeGuard); ok {
				locked, e := guard.PendingGroups(ctx, project, bo)
				if e != nil {
					return e
				}
				for _, id := range locked {
					var old, current *domain.IndentGroupsGroup
					for i := range before.Groups {
						if before.Groups[i].ID == id {
							old = &before.Groups[i]
						}
					}
					for i := range record.Groups {
						if record.Groups[i].ID == id {
							current = &record.Groups[i]
						}
					}
					if old == nil || current == nil || old.Name != current.Name || old.ScopeRevision != current.ScopeRevision || !domain.IndentGroupsOrderedEqual(old.LineIDs, current.LineIDs) || !domain.IndentGroupsOrderedEqual(old.VendorIDs, current.VendorIDs) {
						return domain.IndentGroupsInvalid("groups", "Waiting for Finance approval; the submitted group's items and vendors are frozen")
					}
				}
			}
		}
		saved, e := r.Save(ctx, *record, a.ID)
		if e != nil {
			return e
		}
		if e = r.Event(ctx, domain.IndentGroupsEvent{ID: s.v.ID(), BOID: bo, Action: input.Action, Reason: input.Reason, Before: before, After: saved, ActorID: a.ID, CreatedAt: saved.UpdatedAt}); e != nil {
			return e
		}
		out, e = s.workspace(ctx, r, c, a, project, bo, write)
		return e
	})
	return
}
