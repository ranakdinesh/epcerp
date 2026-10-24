package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type IndentDocumentsService struct {
	u ports.IndentDocumentsUnitOfWork
	a ports.IndentDocumentsAuthorizer
	v ports.IndentDocumentsValues
}

func NewIndentDocuments(u ports.IndentDocumentsUnitOfWork, a ports.IndentDocumentsAuthorizer, v ports.IndentDocumentsValues) *IndentDocumentsService {
	return &IndentDocumentsService{u, a, v}
}
func (s *IndentDocumentsService) actor(ctx context.Context, t string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, t)
	if e != nil {
		return a, e
	}
	if !a.Active || a.ID == "" {
		return a, domain.IndentDocumentsUnauthorized
	}
	return a, nil
}
func indentDocumentsRead(a domain.Principal) bool {
	return a.Can("projects.purchase_bo.read") && a.Can("projects.purchase_indent_groups.read") && a.Can("projects.purchase_indent_documents.read")
}
func (s *IndentDocumentsService) workspace(ctx context.Context, r ports.IndentDocumentsRepository, c ports.IndentDocumentsContext, a domain.Principal, p, b string, write bool) (domain.IndentDocumentsWorkspace, error) {
	rev, e := c.Revisions(ctx, p, b)
	if e != nil {
		return domain.IndentDocumentsWorkspace{}, e
	}
	docs, e := r.List(ctx, p, b)
	if e != nil {
		return domain.IndentDocumentsWorkspace{}, e
	}
	for i := range docs {
		docs[i].Data.IsCurrent = docs[i].Data.ScopeRevision == rev[docs[i].GroupID]
	}
	h, e := r.History(ctx, p, b)
	if e != nil {
		return domain.IndentDocumentsWorkspace{}, e
	}
	rec, e := c.Recipients(ctx, p)
	if e != nil {
		return domain.IndentDocumentsWorkspace{}, e
	}
	return domain.IndentDocumentsWorkspace{Documents: docs, History: h, ManagerAssigned: rec.ManagerAssigned, CanGenerate: write && a.Can("projects.purchase_indent_documents.generate"), CanPrint: write && a.Can("projects.purchase_indent_documents.print"), CanDispatch: write && a.Can("projects.purchase_indent_documents.dispatch")}, nil
}
func (s *IndentDocumentsService) Workspace(ctx context.Context, t, p, b string) (out domain.IndentDocumentsWorkspace, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return out, e
	}
	if !indentDocumentsRead(a) || (!a.Can("projects.purchase_indent_register.view") && !a.Can("projects.purchase_indent_documents.view")) {
		return out, domain.IndentDocumentsForbidden
	}
	if p, e = domain.IndentDocumentsID(p); e != nil {
		return
	}
	if b, e = domain.IndentDocumentsID(b); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.IndentDocumentsRepository, c ports.IndentDocumentsContext) error {
		write, e := c.Scope(ctx, a, p, "projects.purchase_indent_documents.read", false)
		if e != nil {
			return e
		}
		out, e = s.workspace(ctx, r, c, a, p, b, write)
		return e
	})
	return
}
func (s *IndentDocumentsService) Execute(ctx context.Context, t, p, b string, input domain.IndentDocumentsCommand) (out domain.IndentDocumentsWorkspace, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return out, e
	}
	if !indentDocumentsRead(a) {
		return out, domain.IndentDocumentsForbidden
	}
	input, e = input.Validate()
	if e != nil {
		return
	}
	permission := "projects.purchase_indent_documents." + input.Action
	if input.Action == "send" {
		permission = "projects.purchase_indent_documents.dispatch"
	}
	if !a.Can(permission) {
		return out, domain.IndentDocumentsForbidden
	}
	if p, e = domain.IndentDocumentsID(p); e != nil {
		return
	}
	if b, e = domain.IndentDocumentsID(b); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.IndentDocumentsRepository, c ports.IndentDocumentsContext) error {
		write, e := c.Scope(ctx, a, p, permission, true)
		if e != nil {
			return e
		}
		rev, e := c.Revisions(ctx, p, b)
		if e != nil {
			return e
		}
		docs, e := r.List(ctx, p, b)
		if e != nil {
			return e
		}
		var d domain.IndentDocumentsDocument
		var before *domain.IndentDocumentsDocument
		now := s.v.Now()
		reason := "Generate frozen vendor indent"
		if input.Action == "generate" {
			revision, ok := rev[*input.GroupID]
			if !ok {
				return domain.IndentDocumentsInvalid("group_id", "Save item groups and vendors first")
			}
			for _, old := range docs {
				if old.GroupID == *input.GroupID && old.VendorID == *input.VendorID && old.Data.ScopeRevision == revision {
					out, e = s.workspace(ctx, r, c, a, p, b, write)
					return e
				}
			}
			scope, e := c.Generate(ctx, a, p, b, input)
			if e != nil {
				return e
			}
			scope.Terms, e = domain.IndentDocumentsVendorTerms(scope.Vendor)
			if e != nil {
				return e
			}
			d = domain.IndentDocumentsDocument{ID: s.v.ID(), ProjectID: p, BOID: b, GroupID: *input.GroupID, VendorID: *input.VendorID, Version: 1, Data: scope, Active: true}
			d.Data.Number = domain.IndentDocumentsNumber(d.ID)
			d.Data.GeneratedAt = domain.IndentDocumentsLocal(now)
			d.Data.GeneratedByID = a.ID
			d.Data.PrintRequests = []domain.IndentDocumentsPrintRequest{}
			d.Data.IsCurrent = false
		} else {
			found := false
			for _, old := range docs {
				if old.ID == *input.DocumentID {
					d = old
					found = true
					break
				}
			}
			if !found {
				return domain.IndentDocumentsMissing
			}
			if d.Data.Terms == nil {
				return domain.IndentDocumentsInvalid("document_id", "This saved indent has no terms snapshot; apply the vendor terms migration before printing or dispatch")
			}
			if d.Version != input.Version {
				return domain.IndentDocumentsStale
			}
			copy := domain.IndentDocumentsCopy(d)
			before = &copy
			switch input.Action {
			case "print":
				reason = "Record vendor indent print request"
				d.Data.PrintRequests = append(d.Data.PrintRequests, domain.IndentDocumentsPrintRequest{RequestedAt: domain.IndentDocumentsLocal(now), ActorID: a.ID})
				if d.Data.SentDate == nil {
					rec, e := c.Recipients(ctx, p)
					if e != nil {
						return e
					}
					if e = r.Notify(ctx, d, rec.IDs, a.ID); e != nil {
						return e
					}
				}
			case "send":
				reason = "Record vendor indent dispatch"
				if e = domain.IndentDocumentsValidateDispatch(d.Data, *input.SentDate, now); e != nil {
					return e
				}
				if e = c.Dispatch(ctx, a, d, input); e != nil {
					return e
				}
				d.Data.SentDate = input.SentDate
				d.Data.Channel = input.Channel
				d.Data.DispatchReference = input.DispatchReference
				local := domain.IndentDocumentsLocal(now)
				d.Data.SentRecordedAt = &local
				d.Data.SentByID = &a.ID
				if e = r.Resolve(ctx, d.ID, a.ID); e != nil {
					return e
				}
			}
			d.Version++
		}
		saved, e := r.Save(ctx, d, a.ID)
		if e != nil {
			return e
		}
		if e = r.Event(ctx, domain.IndentDocumentsEvent{ID: s.v.ID(), DocumentID: d.ID, Action: input.Action, Reason: reason, Before: before, After: saved, ActorID: a.ID, CreatedAt: saved.UpdatedAt}); e != nil {
			return e
		}
		out, e = s.workspace(ctx, r, c, a, p, b, write)
		return e
	})
	return
}
func (s *IndentDocumentsService) Notifications(ctx context.Context, t string) (out []domain.IndentDocumentsNotice, e error) {
	a, e := s.actor(ctx, t)
	if e != nil {
		return
	}
	if !a.Can("projects.master.read") && !a.Can("projects.purchase_indent_documents.read") {
		return nil, domain.IndentDocumentsForbidden
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.IndentDocumentsRepository, c ports.IndentDocumentsContext) error {
		out, e = r.Notices(ctx, a.ID)
		return e
	})
	return
}
