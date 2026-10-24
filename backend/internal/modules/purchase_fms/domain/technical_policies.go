package domain

import (
	"math/big"
	"regexp"
	"slices"
)

var technicalQuantityPattern = regexp.MustCompile(`^[0-9]{1,15}(\.[0-9]{1,4})?$`)
var technicalAmountPattern = regexp.MustCompile(`^[0-9]{1,14}(\.[0-9]{1,2})?$`)
var technicalCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

func (s TechnicalSource) Guard() error {
	for _, v := range []string{s.CaseID, s.ProjectID, s.PackageID, s.SupplierID} {
		if e := TechnicalID(v); e != nil {
			return e
		}
	}
	if s.Version < 1 || s.LOIReference == "" || s.FinancialReference == "" || !TechnicalDate(s.ReleasedDate) {
		return TechnicalInvalid("loi", "An actual released LOI is required")
	}
	quantity, quantityOK := new(big.Rat).SetString(s.Quantity)
	amount, amountOK := new(big.Rat).SetString(s.Total)
	if !technicalQuantityPattern.MatchString(s.Quantity) || !quantityOK || quantity.Sign() <= 0 || !technicalAmountPattern.MatchString(s.Total) || !amountOK || amount.Sign() < 0 || !technicalCurrencyPattern.MatchString(s.Currency) {
		return TechnicalInvalid("loi", "Released LOI quantity, amount and currency must be complete and valid")
	}
	if !s.Available || len(s.Blockers) > 0 {
		return TechnicalInvalid("source", "Current project/source controls block technical execution")
	}
	return nil
}
func technicalSameSource(a, b TechnicalSource) bool {
	return a.CaseID == b.CaseID && a.Version == b.Version && a.ProjectID == b.ProjectID && a.PackageID == b.PackageID && a.SupplierID == b.SupplierID && a.Quantity == b.Quantity && a.Currency == b.Currency && a.Total == b.Total && a.FinancialReference == b.FinancialReference && a.LOIReference == b.LOIReference && a.ReleasedDate == b.ReleasedDate && a.Description == b.Description
}
func TechnicalAccessGuard(a Principal, caps TechnicalAccess, action string) error {
	if !a.Active || TechnicalID(a.ID) != nil {
		return TechnicalUnauthorized
	}
	if !caps.Read {
		return TechnicalForbidden
	}
	if action == "team_review" {
		if !caps.Approve || !caps.Scoped {
			return TechnicalForbidden
		}
		return nil
	}
	n := 6
	if action == "submit" {
		n = 7
	} else if action == "client_approval" {
		n = 8
	}
	if !caps.Write || !caps.Work[n] {
		return TechnicalForbidden
	}
	return nil
}
func TechnicalFileGuard(files []TechnicalFile, id, caseID, purpose string) error {
	if TechnicalID(id) != nil {
		return TechnicalInvalid("file_id", "Upload actual evidence first")
	}
	for _, f := range files {
		if f.ID == id && f.CaseID == caseID && f.Active && f.Purpose == purpose && f.Size > 0 && slices.Contains([]string{"application/pdf", "image/png", "image/jpeg"}, f.MediaType) {
			return nil
		}
	}
	return TechnicalInvalid("file_id", "Evidence must be an active file owned by this technical package and purpose")
}
func TechnicalReady(c TechnicalCase) bool {
	if !c.Active || c.Submission == nil || c.TeamReview == nil || c.Approval == nil || c.Submission.Revision != c.Revision || c.TeamReview.Revision != c.Revision || c.Approval.Revision != c.Revision || !c.TeamReview.Approved || !c.Approval.QAPApproved {
		return false
	}
	if c.State != "approved" && c.State != "approved_with_comments" {
		return false
	}
	if c.Approval.Status != c.State {
		return false
	}
	if c.Submission.SubmittedBy == c.TeamReview.ReviewerID || c.Submission.SubmittedBy == "" || c.TeamReview.ReviewerID == "" {
		return false
	}
	if !slices.Equal(c.Documents, c.Submission.Documents) {
		return false
	}
	return c.State == "approved" || c.Approval.ConditionsClosed && c.Approval.ClosureFileID != ""
}
func TechnicalApply(base *TechnicalCase, a Principal, caps TechnicalAccess, q TechnicalCommand, source TechnicalSource, files []TechnicalFile, now TechnicalTimestamp, id string, documentIDs []string) (TechnicalCase, error) {
	empty := TechnicalCase{}
	if e := TechnicalAccessGuard(a, caps, q.Action); e != nil {
		return empty, e
	}
	if e := source.Guard(); e != nil {
		return empty, e
	}
	var e error
	q.Reason, e = TechnicalText("reason", q.Reason, 2000, true)
	if e != nil {
		return empty, e
	}
	version := int32(0)
	if base != nil {
		version = base.Version
	}
	if q.Version != version {
		return empty, TechnicalStale
	}
	if !slices.Contains([]string{"configure", "document", "submit", "team_review", "client_approval", "revise"}, q.Action) {
		return empty, TechnicalInvalid("action", "Choose a technical action")
	}
	var c TechnicalCase
	if base == nil {
		if q.Action != "configure" {
			return empty, TechnicalMissing
		}
		if e := TechnicalID(id); e != nil {
			return empty, e
		}
		docs, e := TechnicalChecklist(documentIDs)
		if e != nil {
			return empty, e
		}
		c = TechnicalCase{ID: id, ProjectID: source.ProjectID, PackageID: source.PackageID, Source: source, Version: 0, Revision: 1, State: "draft", Documents: docs, Active: true, CreatedAt: now, CreatedBy: a.ID}
	} else {
		c = TechnicalCopy(*base)
		if !c.Active {
			return empty, TechnicalMissing
		}
		if !technicalSameSource(c.Source, source) {
			return empty, TechnicalInvalid("loi", "Released LOI authority changed; review the source")
		}
		if q.Action == "configure" {
			return empty, TechnicalInvalid("state", "Technical checklist is already configured")
		}
	}
	switch q.Action {
	case "document":
		if c.State != "draft" && c.State != "revision_required" {
			return empty, TechnicalInvalid("state", "Start a new technical revision before changing submitted evidence")
		}
		if q.Document == nil {
			return empty, TechnicalInvalid("document", "Supply a document")
		}
		d, e := TechnicalValidateDocument(*q.Document, c, files, TechnicalToday(now))
		if e != nil {
			return empty, e
		}
		index := -1
		for i, v := range c.Documents {
			if v.ID == d.ID {
				index = i
				break
			}
		}
		if index < 0 {
			return empty, TechnicalInvalid("document", "Only an owned checklist document can be edited")
		}
		c.Documents[index] = d
		c.Revision++
		c.State = "draft"
		c.Submission = nil
		c.TeamReview = nil
		c.Approval = nil
	case "revise":
		if c.State == "draft" {
			return empty, TechnicalInvalid("state", "Technical package is already draft")
		}
		c.Revision++
		c.State = "draft"
		c.Submission = nil
		c.TeamReview = nil
		c.Approval = nil
	case "submit":
		if c.State != "draft" {
			return empty, TechnicalInvalid("state", "Only a draft technical package can be submitted")
		}
		if q.Submission == nil {
			return empty, TechnicalInvalid("submission", "Supply transmittal details")
		}
		for _, d := range c.Documents {
			if d.Active && d.Required && (d.Status != "received" || TechnicalFileGuard(files, d.FileID, c.ID, "document") != nil) {
				return empty, TechnicalInvalid("documents", "Receive actual evidence for every required document before submission")
			}
		}
		s := *q.Submission
		for _, v := range []struct {
			field, value string
			max          int
		}{{"reference", s.Reference, 200}, {"authority", s.Authority, 300}} {
			if _, e := TechnicalText(v.field, v.value, v.max, true); e != nil {
				return empty, e
			}
		}
		if !TechnicalDate(s.Date) || !TechnicalDate(s.TargetDate) || s.Date < c.Source.ReleasedDate || s.Date > TechnicalToday(now) || s.TargetDate < s.Date {
			return empty, TechnicalInvalid("date", "Use submission after LOI release, not future, and target on/after submission")
		}
		if e := TechnicalFileGuard(files, s.FileID, c.ID, "transmittal"); e != nil {
			return empty, e
		}
		c.Submission = &TechnicalSubmission{Revision: c.Revision, Reference: s.Reference, Date: s.Date, TargetDate: s.TargetDate, Authority: s.Authority, FileID: s.FileID, Documents: append([]TechnicalDocument{}, c.Documents...), SubmittedBy: a.ID, SubmittedAt: now}
		c.State = "submitted"
	case "team_review":
		if c.State != "submitted" || c.Submission == nil || c.Submission.Revision != c.Revision {
			return empty, TechnicalInvalid("state", "Review the current submitted technical revision")
		}
		if c.Submission.SubmittedBy == a.ID {
			return empty, TechnicalInvalid("reviewer", "Project-team review must be independent of submission")
		}
		if q.Approve == nil {
			return empty, TechnicalInvalid("approve", "Record explicit approval or rejection")
		}
		remarks, e := TechnicalText("remarks", q.Remarks, 4000, true)
		if e != nil {
			return empty, e
		}
		c.TeamReview = &TechnicalTeamReview{Revision: c.Revision, Approved: *q.Approve, Remarks: remarks, ReviewerID: a.ID, ReviewedAt: now}
		c.State = "revision_required"
		if *q.Approve {
			c.State = "under_review"
		}
	case "client_approval":
		if !slices.Contains([]string{"under_review", "approved", "approved_with_comments"}, c.State) || c.Submission == nil || c.TeamReview == nil || !c.TeamReview.Approved || c.Submission.Revision != c.Revision || c.TeamReview.Revision != c.Revision {
			return empty, TechnicalInvalid("state", "Independent project review is required before recording client approval")
		}
		if q.Approval == nil || q.QAPApproved == nil || q.ConditionsClosed == nil {
			return empty, TechnicalInvalid("approval", "Supply client decision and explicit QAP/conditions attestations")
		}
		v := *q.Approval
		if !slices.Contains([]string{"approved", "approved_with_comments", "revision_required"}, v.Status) {
			return empty, TechnicalInvalid("status", "Choose the actual client decision")
		}
		for _, f := range []struct {
			field, value string
			max          int
		}{{"reference", v.Reference, 200}, {"approver_name", v.Name, 200}, {"approver_designation", v.Designation, 200}, {"remarks", v.Remarks, 4000}} {
			if _, e := TechnicalText(f.field, f.value, f.max, true); e != nil {
				return empty, e
			}
		}
		if !TechnicalDate(v.Date) || v.Date < c.Submission.Date || v.Date > TechnicalToday(now) {
			return empty, TechnicalInvalid("date", "Client decision must be between submission and today")
		}
		if e := TechnicalFileGuard(files, v.FileID, c.ID, "client_approval"); e != nil {
			return empty, e
		}
		if v.Status == "approved_with_comments" && !*q.ConditionsClosed && v.ClosureFileID != "" {
			return empty, TechnicalInvalid("closure_file_id", "Record closure evidence only with an explicit completed-conditions attestation")
		}
		if v.Status == "approved_with_comments" && *q.ConditionsClosed {
			if e := TechnicalFileGuard(files, v.ClosureFileID, c.ID, "conditions"); e != nil {
				return empty, e
			}
		}
		if v.Status != "approved_with_comments" && (v.ClosureFileID != "" || *q.ConditionsClosed) {
			return empty, TechnicalInvalid("conditions_closed", "Condition closure applies only to approved-with-comments decisions")
		}
		c.Approval = &TechnicalApproval{Revision: c.Revision, Status: v.Status, Reference: v.Reference, Date: v.Date, Name: v.Name, Designation: v.Designation, FileID: v.FileID, QAPApproved: *q.QAPApproved, ConditionsClosed: *q.ConditionsClosed, ClosureFileID: v.ClosureFileID, Remarks: v.Remarks, RecordedBy: a.ID, RecordedAt: now}
		c.State = v.Status
	}
	c.Version++
	c.UpdatedAt = now
	c.UpdatedBy = a.ID
	return c, nil
}
func TechnicalValidateDocument(v TechnicalDocumentInput, c TechnicalCase, files []TechnicalFile, today string) (TechnicalDocument, error) {
	d := TechnicalDocument{ID: v.ID, Kind: v.Kind, Title: v.Title, Category: v.Category, Standard: v.Standard, Status: v.Status, Revision: v.Revision, ReceivedDate: v.ReceivedDate, FileID: v.FileID, Notes: v.Notes, NAReason: v.NAReason}
	if e := TechnicalID(d.ID); e != nil {
		return d, e
	}
	if v.Required == nil || v.Active == nil {
		return d, TechnicalInvalid("document", "Record explicit required/active flags")
	}
	d.Required = *v.Required
	d.Active = *v.Active
	for _, f := range []struct {
		field    string
		value    *string
		max      int
		required bool
	}{{"title", &d.Title, 200, true}, {"category", &d.Category, 120, true}, {"standard", &d.Standard, 1000, true}, {"revision", &d.Revision, 80, true}, {"notes", &d.Notes, 4000, false}, {"na_reason", &d.NAReason, 2000, false}} {
		var e error
		*f.value, e = TechnicalText(f.field, *f.value, f.max, f.required)
		if e != nil {
			return d, e
		}
	}
	var previous *TechnicalDocument
	for _, old := range c.Documents {
		if old.ID == d.ID {
			v := old
			previous = &v
		}
	}
	if previous == nil || previous.Kind != d.Kind {
		return d, TechnicalInvalid("document", "Owned document kind cannot change")
	}
	if (d.Kind == "qap" || d.Kind == "tds") && (!d.Required || !d.Active) {
		return d, TechnicalInvalid("required", "TDS and QAP must remain active and required")
	}
	if (!d.Required || !d.Active) && (previous.Required || previous.Active && !d.Active) && d.NAReason == "" {
		return d, TechnicalInvalid("na_reason", "Explain why required evidence is not applicable")
	}
	if !slices.Contains([]string{"pending", "received", "revision_required"}, d.Status) {
		return d, TechnicalInvalid("status", "Receive evidence; approval comes from the submitted package review")
	}
	if d.Status == "received" {
		if !TechnicalDate(d.ReceivedDate) || d.ReceivedDate < c.Source.ReleasedDate || d.ReceivedDate > today {
			return d, TechnicalInvalid("received_date", "Receipt must be between LOI release and today")
		}
		if e := TechnicalFileGuard(files, d.FileID, c.ID, "document"); e != nil {
			return d, e
		}
	}
	if d.Status == "pending" && (d.FileID != "" || d.ReceivedDate != "") {
		return d, TechnicalInvalid("file_id", "Pending evidence cannot claim receipt")
	}
	return d, nil
}

func TechnicalReadyWithEvidence(c TechnicalCase, files []TechnicalFile) bool {
	if !TechnicalReady(c) || TechnicalFileGuard(files, c.Submission.FileID, c.ID, "transmittal") != nil || TechnicalFileGuard(files, c.Approval.FileID, c.ID, "client_approval") != nil {
		return false
	}
	if c.State == "approved_with_comments" && TechnicalFileGuard(files, c.Approval.ClosureFileID, c.ID, "conditions") != nil {
		return false
	}
	for _, d := range c.Documents {
		if d.Active && d.Required && (d.Status != "received" || TechnicalFileGuard(files, d.FileID, c.ID, "document") != nil) {
			return false
		}
	}
	return true
}

func TechnicalValidateCommand(q TechnicalCommand) (TechnicalCommand, error) {
	if q.Version < 0 {
		return q, TechnicalInvalid("version", "Use the current nonnegative version")
	}
	var e error
	q.Reason, e = TechnicalText("reason", q.Reason, 2000, true)
	if e != nil {
		return q, e
	}
	if !slices.Contains([]string{"configure", "document", "submit", "team_review", "client_approval", "revise"}, q.Action) {
		return q, TechnicalInvalid("action", "Choose a technical action")
	}
	return q, nil
}

// CurrentSourceGuard prevents an approval for an obsolete LOI snapshot from
// advertising PO readiness or new actions, even while retained history is readable.
func TechnicalCurrentSourceGuard(c TechnicalCase, current TechnicalSource) error {
	if e := current.Guard(); e != nil {
		return e
	}
	if !technicalSameSource(c.Source, current) {
		return TechnicalStale
	}
	return nil
}
func TechnicalCurrentReady(c TechnicalCase, current TechnicalSource, files []TechnicalFile) bool {
	return TechnicalCurrentSourceGuard(c, current) == nil && TechnicalReadyWithEvidence(c, files)
}
