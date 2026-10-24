package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type QuotationApprovalsService struct {
	printer ports.QuotationApprovalsPrinter
	u       ports.QuotationApprovalsUnitOfWork
	a       ports.QuotationApprovalsAuthorizer
	v       ports.QuotationApprovalsValues
}

func NewQuotationApprovals(u ports.QuotationApprovalsUnitOfWork, a ports.QuotationApprovalsAuthorizer, v ports.QuotationApprovalsValues) *QuotationApprovalsService {
	return &QuotationApprovalsService{u: u, a: a, v: v}
}
func (s *QuotationApprovalsService) actor(ctx context.Context, token, p, b string) (domain.Principal, error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil {
		return a, e
	}
	if !a.Active || a.ID == "" {
		return a, domain.QuotationApprovalsUnauthorized
	}
	if _, e = domain.QuotationApprovalsID(p); e != nil {
		return a, e
	}
	if _, e = domain.QuotationApprovalsID(b); e != nil {
		return a, e
	}
	return a, nil
}
func quotationApprovalsWorkspace(ctx context.Context, r ports.QuotationApprovalsRepository, c ports.QuotationApprovalsContext, a domain.Principal, p, b string) (domain.QuotationApprovalsWorkspace, error) {
	caps, e := c.Actions(ctx, a, p)
	if e != nil {
		return domain.QuotationApprovalsWorkspace{}, e
	}
	if !caps.Read {
		return domain.QuotationApprovalsWorkspace{}, domain.QuotationApprovalsForbidden
	}
	source, e := c.Source(ctx, p, b)
	if e != nil {
		return domain.QuotationApprovalsWorkspace{}, e
	}
	record, e := r.Get(ctx, p, b)
	if e != nil {
		return domain.QuotationApprovalsWorkspace{}, e
	}
	out := domain.QuotationApprovalsWorkspace{Attempts: []domain.QuotationApprovalsAttempt{}, VendorStatuses: source.Verifications, Currency: source.Currency, CanSubmit: caps.Submit && source.Available, CanReviewRead: caps.ReviewRead, CanReview: caps.Review, CanVerify: caps.Verify, CanReadBudget: caps.BudgetRead, CanUpdateBudget: caps.BudgetWrite && source.Available, History: []domain.QuotationApprovalsEvent{}}
	if labels, ok := c.(ports.QuotationApprovalsQueueContext); ok {
		out.ProjectName, out.ProjectNumber, e = labels.ProjectLabel(ctx, p)
		if e != nil {
			return out, e
		}
	}
	if record != nil {
		if !record.Active {
			return out, domain.QuotationApprovalsMissing
		}
		out.Version = record.Version
		out.Attempts, _, out.Ready = domain.QuotationApprovalsEvaluate(record.Attempts, source)
	}
	if drafts, ok := r.(ports.QuotationShortlistDraftRepository); ok {
		out.Drafts, e = drafts.Drafts(ctx, p, b)
		if e != nil {
			return out, e
		}
	}
	out.History, e = r.History(ctx, p, b)
	if caps.ReviewRead {
		for i := range out.Attempts {
			if out.Attempts[i].State == "pending_accounts" {
				out.Attempts[i] = domain.QuotationApprovalsPendingFinanceBudget(out.Attempts[i], source)
			}
		}
	}
	if !caps.ReviewRead {
		out.Attempts = domain.QuotationApprovalsRedact(out.Attempts)
		for i := range out.History {
			h := &out.History[i]
			h.After = domain.QuotationApprovalsCopyRecord(h.After)
			h.After.Attempts = domain.QuotationApprovalsRedact(h.After.Attempts)
			if h.Before != nil {
				before := domain.QuotationApprovalsCopyRecord(*h.Before)
				before.Attempts = domain.QuotationApprovalsRedact(before.Attempts)
				h.Before = &before
			}
		}
	}
	return out, e
}
func (s *QuotationApprovalsService) Workspace(ctx context.Context, token, p, b string) (out domain.QuotationApprovalsWorkspace, e error) {
	a, e := s.actor(ctx, token, p, b)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.QuotationApprovalsRepository, c ports.QuotationApprovalsContext, f ports.QuotationApprovalsFinance) error {
		out, e = quotationApprovalsWorkspace(ctx, r, c, a, p, b)
		return e
	})
	if e != nil {
		out = domain.QuotationApprovalsWorkspace{}
	}
	return
}
func (s *QuotationApprovalsService) Execute(ctx context.Context, token, p, b, action string, input domain.QuotationApprovalsCommand) (out domain.QuotationApprovalsWorkspace, e error) {
	a, e := s.actor(ctx, token, p, b)
	if e != nil {
		return out, e
	}
	input, e = input.Validate(action)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.QuotationApprovalsRepository, c ports.QuotationApprovalsContext, f ports.QuotationApprovalsFinance) error {
		caps, e := c.Actions(ctx, a, p)
		if e != nil {
			return e
		}
		if !caps.Read || (action == "submit" && !caps.Submit) || (action == "review" && (!caps.ReviewRead || !caps.Review)) {
			return domain.QuotationApprovalsForbidden
		}
		source, e := c.Source(ctx, p, b)
		if e != nil {
			return e
		}
		if !source.Available {
			return domain.QuotationApprovalsInvalid("project", "Project purchasing is inactive, held or closed")
		}
		record, e := r.Get(ctx, p, b)
		if e != nil {
			return e
		}
		var before *domain.QuotationApprovalsRecord
		if record != nil {
			if !record.Active {
				return domain.QuotationApprovalsMissing
			}
			copy := domain.QuotationApprovalsCopyRecord(*record)
			before = &copy
		} else {
			record = &domain.QuotationApprovalsRecord{ProjectID: p, BOID: b, Active: true, Attempts: []domain.QuotationApprovalsAttempt{}}
		}
		if input.Version != record.Version {
			return domain.QuotationApprovalsStale
		}
		_, latest, _ := domain.QuotationApprovalsEvaluate(record.Attempts, source)
		var changed domain.QuotationApprovalsAttempt
		recipients := []string{}
		if action == "submit" {
			var chosen *domain.QuotationApprovalsQuote
			for i := range source.Quotes {
				if source.Quotes[i].Key == input.QuoteKey || (len(input.Proposals) > 0 && source.Quotes[i].Key == input.Proposals[0].QuoteKey) {
					chosen = &source.Quotes[i]
					break
				}
			}
			if chosen == nil {
				return domain.QuotationApprovalsMissing
			}
			prior, ok := latest[chosen.GroupID]
			if ok && (prior.EffectiveState == "pending_accounts" || prior.EffectiveState == "approved") {
				return domain.QuotationApprovalsInvalid("quote_key", "This group already has a pending or approved quote; Accounts must decide first")
			}
			if len(record.Attempts) >= 1000 {
				return domain.QuotationApprovalsInvalid("quote_key", "This BO has reached the approval history limit; contact an administrator")
			}
			attempt, e := domain.QuotationApprovalsSnapshot(*chosen, source)
			if len(input.Proposals) > 0 {
				attempt, e = domain.QuotationApprovalsShortlist(input.Proposals, source)
			}
			if e != nil {
				return e
			}
			attempt.ID = s.v.ID()
			attempt.State = "pending_accounts"
			attempt.SubmittedBy = a.ID
			attempt.SubmittedAt = s.v.Now()
			attempt.Reason = input.Reason
			if len(attempt.Proposals) > 0 {
				directory, ok := c.(ports.QuotationApprovalsWorkContext)
				if !ok {
					return domain.QuotationApprovalsInvalid("finance", "Finance reviewer assignment unavailable")
				}
				reviewers, e := directory.Reviewers(ctx, p)
				if e != nil {
					return e
				}
				for _, reviewer := range reviewers {
					if reviewer.ID != a.ID {
						recipients = append(recipients, reviewer.ID)
					}
				}
				if len(recipients) == 0 {
					return domain.QuotationApprovalsInvalid("finance", "Assign another authorized Finance reviewer before submitting")
				}
			}
			if drafts, ok := r.(ports.QuotationShortlistDraftRepository); ok {
				if e = drafts.CompleteDraft(ctx, p, b, attempt.GroupID, a.ID); e != nil {
					return e
				}
			}
			changed = attempt
			record.Attempts = append(record.Attempts, attempt)
		} else {
			index := -1
			for i := range record.Attempts {
				if record.Attempts[i].ID == input.AttemptID {
					index = i
					break
				}
			}
			if index < 0 {
				return domain.QuotationApprovalsMissing
			}
			attempt := record.Attempts[index]
			if latest[attempt.GroupID].ID != attempt.ID {
				return domain.QuotationApprovalsMissing
			}
			reason := domain.QuotationApprovalsProposalStaleReason(attempt, source)
			if reason != "" {
				return domain.QuotationApprovalsInvalid("attempt_id", reason)
			}
			if attempt.State != "pending_accounts" {
				return domain.QuotationApprovalsInvalid("attempt_id", "This quotation has already been decided")
			}
			if attempt.SubmittedBy == a.ID {
				return domain.QuotationApprovalsInvalidReviewer()
			}
			selected, e := domain.QuotationApprovalsSelectProposal(attempt, input.SelectedQuoteKey)
			if e != nil {
				return e
			}
			updated, e := domain.QuotationApprovalsDecide(selected, input.Decisions)
			if e != nil {
				return e
			}
			if len(attempt.Proposals) > 0 {
				if updated.State == "rejected" && input.ReturnAction == "" {
					return domain.QuotationApprovalsInvalid("return_action", "Choose negotiate or change vendor for a rejected group")
				}
				if updated.State == "approved" && input.ReturnAction != "" {
					return domain.QuotationApprovalsInvalid("return_action", "Approved groups cannot have a return instruction")
				}
			}
			if len(attempt.Proposals) > 0 {
				for _, d := range input.Decisions {
					if d.Decision != input.Decisions[0].Decision {
						return domain.QuotationApprovalsInvalid("decisions", "Approve one complete proposal or reject the whole group")
					}
				}
			}
			if updated.State == "approved" {
				updated = domain.QuotationApprovalsFinanceBudget(updated, source)
				if updated.BudgetIssue != "" {
					return domain.QuotationApprovalsInvalid("budget", updated.BudgetIssue)
				}
			}
			updated.ReturnAction = input.ReturnAction
			updated.ReviewComment = input.Reason
			if f == nil {
				return domain.QuotationApprovalsInvalid("approval", "Finance approval is unavailable")
			}
			decision, e := f.Decide(ctx, a, *record, updated, input.Reason)
			if e != nil {
				return e
			}
			if _, e = domain.QuotationApprovalsID(decision); e != nil {
				return e
			}
			now := s.v.Now()
			updated.ReviewedAt = &now
			updated.ReviewedBy = &a.ID
			updated.DecisionID = &decision
			record.Attempts[index] = updated
			changed = updated
			if len(updated.Proposals) > 0 {
				recipients = append(recipients, updated.SubmittedBy)
			}
		}
		record.Version++
		saved, e := r.Save(ctx, *record, a.ID)
		if e != nil {
			return e
		}
		record = &saved
		eventAction := "submit"
		if action == "review" {
			eventAction = "accounts_decision"
		}
		event := domain.QuotationApprovalsEvent{ID: s.v.ID(), ProjectID: p, BOID: b, Action: eventAction, Reason: input.Reason, Before: before, After: *record, ActorID: a.ID, CreatedAt: s.v.Now()}
		if labels, ok := c.(ports.QuotationApprovalsQueueContext); ok {
			name, number, err := labels.ProjectLabel(ctx, p)
			if err != nil {
				return err
			}
			event.ProjectLabel = number + " · " + name
		}
		if e = r.Event(ctx, event); e != nil {
			return e
		}
		if len(changed.Proposals) > 0 {
			notifier, ok := r.(ports.QuotationApprovalsWorkRepository)
			if !ok {
				return domain.QuotationApprovalsInvalid("notifications", "Quotation notice delivery unavailable")
			}
			plan := domain.QuotationApprovalWork(event, changed, recipients)
			for i := range plan.Create {
				plan.Create[i].ID = s.v.ID()
			}
			if e = notifier.Notify(ctx, plan); e != nil {
				return e
			}
		}
		out, e = quotationApprovalsWorkspace(ctx, r, c, a, p, b)
		return e
	})
	if e != nil {
		out = domain.QuotationApprovalsWorkspace{}
	}
	return
}
func (s *QuotationApprovalsService) Preview(ctx context.Context, token, p, b, key string) (out domain.QuotationApprovalsPreview, e error) {
	a, e := s.actor(ctx, token, p, b)
	if e != nil {
		return out, e
	}
	check := domain.QuotationApprovalsCommand{QuoteKey: key, Reason: "Preview"}
	if _, e = check.Validate("submit"); e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.QuotationApprovalsRepository, c ports.QuotationApprovalsContext, f ports.QuotationApprovalsFinance) error {
		caps, e := c.Actions(ctx, a, p)
		if e != nil {
			return e
		}
		if !caps.Read || !caps.ReviewRead {
			return domain.QuotationApprovalsForbidden
		}
		source, e := c.Source(ctx, p, b)
		if e != nil {
			return e
		}
		for _, q := range source.Quotes {
			if q.Key == key {
				if !q.Eligible {
					return domain.QuotationApprovalsInvalid("quote_key", "Quotation is not eligible")
				}
				out, e = domain.QuotationApprovalsBudgetPreview(q, source)
				return e
			}
		}
		return domain.QuotationApprovalsMissing
	})
	if e != nil {
		out = domain.QuotationApprovalsPreview{}
	}
	return
}

// ApprovedSelections is an internal transaction-bound downstream publication;
// the consuming sourcing/PO capability must authorize its own scoped use case.
func QuotationApprovalsApprovedSelections(ctx context.Context, r ports.QuotationApprovalsRepository, c ports.QuotationApprovalsContext, p, b string) ([]domain.QuotationApprovalsAttempt, error) {
	source, e := c.Source(ctx, p, b)
	if e != nil {
		return nil, e
	}
	return quotationApprovalsApprovedForSource(ctx, r, source, p, b)
}
func quotationApprovalsApprovedForSource(ctx context.Context, r ports.QuotationApprovalsRepository, source domain.QuotationApprovalsSource, p, b string) ([]domain.QuotationApprovalsAttempt, error) {
	record, e := r.Get(ctx, p, b)
	if e != nil {
		return nil, e
	}
	if record == nil || !record.Active {
		return nil, domain.QuotationApprovalsForbidden
	}
	_, latest, ready := domain.QuotationApprovalsEvaluate(record.Attempts, source)
	if !ready {
		return nil, domain.QuotationApprovalsInvalid("approval", "Accounts must approve every current group's quotation before the next step")
	}
	out := []domain.QuotationApprovalsAttempt{}
	for _, g := range source.Groups {
		out = append(out, latest[g.ID])
	}
	return domain.QuotationApprovalsRedact(out), nil
}

// CurrentCommercialSelections publishes actual current priced lines only after
// the same complete-group Finance gate; it grants no caller authority.
func QuotationApprovalsCurrentCommercialSelections(ctx context.Context, r ports.QuotationApprovalsRepository, c ports.QuotationApprovalsContext, p, b string) ([]domain.QuotationApprovalsCommercialLine, error) {
	source, e := c.Source(ctx, p, b)
	if e != nil {
		return nil, e
	}
	approved, e := quotationApprovalsApprovedForSource(ctx, r, source, p, b)
	if e != nil {
		return nil, e
	}
	out := []domain.QuotationApprovalsCommercialLine{}
	for _, a := range approved {
		for _, q := range source.Quotes {
			if q.Key != a.QuoteKey || q.VendorID != a.VendorID {
				continue
			}
			for _, l := range q.Lines {
				for _, accepted := range a.Lines {
					if l.LineID == accepted.LineID && l.Item.ID == accepted.Item.ID {
						out = append(out, domain.QuotationApprovalsCommercialLine{AttemptID: a.ID, LineID: l.LineID, ItemID: l.Item.ID, VendorID: a.VendorID, QuoteKey: a.QuoteKey, Quantity: l.Quantity, Total: l.Quoted, Price: l.Price})
					}
				}
			}
		}
	}
	return out, nil
}

func QuotationApprovalsApprovedGroups(ctx context.Context, r ports.QuotationApprovalsRepository, c ports.QuotationApprovalsContext, p, b string) ([]domain.QuotationApprovalsAttempt, error) {
	source, e := c.Source(ctx, p, b)
	if e != nil {
		return nil, e
	}
	record, e := r.Get(ctx, p, b)
	if e != nil {
		return nil, e
	}
	if record == nil || !record.Active {
		return nil, domain.QuotationApprovalsForbidden
	}
	_, latest, _ := domain.QuotationApprovalsEvaluate(record.Attempts, source)
	out := []domain.QuotationApprovalsAttempt{}
	for _, g := range source.Groups {
		a, ok := latest[g.ID]
		if ok && a.EffectiveState == "approved" {
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		return nil, domain.QuotationApprovalsInvalid("approval", "Finance must approve this group's quotation before sourcing")
	}
	return domain.QuotationApprovalsRedact(out), nil
}
func QuotationApprovalsCommercialGroups(ctx context.Context, r ports.QuotationApprovalsRepository, c ports.QuotationApprovalsContext, p, b string) ([]domain.QuotationApprovalsCommercialLine, error) {
	approved, e := QuotationApprovalsApprovedGroups(ctx, r, c, p, b)
	if e != nil {
		return nil, e
	}
	source, e := c.Source(ctx, p, b)
	if e != nil {
		return nil, e
	}
	out := []domain.QuotationApprovalsCommercialLine{}
	for _, a := range approved {
		for _, q := range source.Quotes {
			if q.Key != a.QuoteKey || q.VendorID != a.VendorID {
				continue
			}
			for _, l := range q.Lines {
				out = append(out, domain.QuotationApprovalsCommercialLine{AttemptID: a.ID, LineID: l.LineID, ItemID: l.Item.ID, VendorID: a.VendorID, QuoteKey: a.QuoteKey, Quantity: l.Quantity, Total: l.Quoted, Price: l.Price})
			}
		}
	}
	return out, nil
}
