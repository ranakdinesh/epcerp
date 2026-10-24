package domain

import (
	"strings"
	"unicode/utf8"
)

func AddressedQuotation(id string, s Source, recipient, title, link string) (Notice, error) {
	for _, v := range []string{id, s.EventID, s.EntityID, s.ProjectID, s.ActorID, recipient} {
		if _, e := ID(v); e != nil {
			return Notice{}, e
		}
	}
	if s.Module != "quotation_approvals" || s.EntityType != "quotation_approval_set" || s.Visibility != "restricted" || s.ScopeKind != "project" || (s.Action != "submit" && s.Action != "accounts_decision") || s.Version < 1 || s.At.IsZero() || utf8.RuneCountInString(title) > 300 || title == "" || len(link) > 400 || !strings.HasPrefix(link, "/dashboard?") {
		return Notice{}, SourceConflict
	}
	actor := s.ActorID
	return Notice{ID: id, EventID: s.EventID, RecipientID: recipient, EntityID: s.EntityID, ProjectID: s.ProjectID, SourceVersion: s.Version, Action: s.Action, Kind: "quotation", Title: title, Link: link, Version: 1, Active: true, CreatedAt: s.At, UpdatedAt: s.At, CreatedBy: &actor, UpdatedBy: &actor}, nil
}

func AddressedSupplierReview(id string, s Source, recipient, title, link string) (Notice, error) {
	for _, value := range []string{id, s.EventID, s.EntityID, s.ActorID, recipient} {
		if _, err := ID(value); err != nil {
			return Notice{}, err
		}
	}
	if s.Module != "supplier_review_assignments" || s.EntityType != "supplier_review_assignment" || s.ScopeKind != "platform" || s.ProjectID != "" || s.Visibility != "restricted" || s.Action != "assigned" || s.Version < 1 || s.At.IsZero() || title == "" || utf8.RuneCountInString(title) > 300 || len(link) > 400 || !strings.HasPrefix(link, "/dashboard?screen=suppliers&supplier_id=") {
		return Notice{}, SourceConflict
	}
	actor := s.ActorID
	return Notice{ID: id, EventID: s.EventID, RecipientID: recipient, EntityID: s.EntityID, SourceVersion: s.Version, Action: s.Action, Kind: "supplier_review", Title: title, Link: link, Version: 1, Active: true, CreatedAt: s.At, UpdatedAt: s.At, CreatedBy: &actor, UpdatedBy: &actor}, nil
}

type TenderAction struct {
	ID, AssignmentID, RecipientID, EventID, TenderID, ActorID string
	Version                                                   int64
	At                                                        Timestamp
}

func AddressedTender(id string, s Source, a TenderAction) (Notice, error) {
	for _, value := range []string{id, a.ID, a.AssignmentID, a.RecipientID, a.EventID, a.TenderID, a.ActorID} {
		if _, e := ID(value); e != nil {
			return Notice{}, e
		}
	}
	if s.Module != "tender_fms" || s.EntityType != "tender_assignment" || s.EntityID != a.AssignmentID || s.EventID != a.EventID || s.Action != "manager_assigned" || s.ScopeKind != "platform" || s.ProjectID != "" || s.Visibility != "restricted" || s.Version != a.Version || s.ActorID != a.ActorID || !s.At.Equal(a.At) {
		return Notice{}, SourceConflict
	}
	actor := a.ActorID
	return Notice{ID: id, SourceActionID: a.ID, Kind: "tender_manager", Title: "Tender assigned for evaluation", Link: "/dashboard?screen=tender-fms&tender_id=" + a.TenderID, EventID: a.EventID, RecipientID: a.RecipientID, EntityID: a.AssignmentID, SourceVersion: a.Version, Action: "assigned", Version: 1, Active: true, CreatedAt: s.At, UpdatedAt: s.At, CreatedBy: &actor, UpdatedBy: &actor}, nil
}
