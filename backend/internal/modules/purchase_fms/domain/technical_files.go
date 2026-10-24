package domain

import (
	"slices"
)

const TechnicalMaxFileBytes = 10 << 20

type TechnicalEvidence struct {
	Bucket, ObjectKey string
	File              TechnicalFile
	Ciphertext        []byte
	CreatedBy         string
	CreatedAt         TechnicalTimestamp
}
type TechnicalFileEvent struct {
	ID, CaseID, FileID, Action, ActorID, Reason string
	At                                          TechnicalTimestamp
}
type TechnicalUpload struct {
	Version                          int32
	Purpose, Name, MediaType, Reason string
	Data                             []byte
}

func TechnicalUploadGuard(c TechnicalCase, a Principal, caps TechnicalAccess, source TechnicalSource, q TechnicalUpload) error {
	action := "document"
	if q.Purpose == "transmittal" {
		action = "submit"
	}
	if q.Purpose == "client_approval" || q.Purpose == "conditions" {
		action = "client_approval"
	}
	if !slices.Contains([]string{"document", "transmittal", "client_approval", "conditions"}, q.Purpose) {
		return TechnicalInvalid("purpose", "Select document, transmittal, client approval or condition closure evidence")
	}
	if e := TechnicalAccessGuard(a, caps, action); e != nil {
		return e
	}
	if !c.Active {
		return TechnicalMissing
	}
	if c.Version != q.Version {
		return TechnicalStale
	}
	if e := source.Guard(); e != nil {
		return e
	}
	if !technicalSameSource(c.Source, source) {
		return TechnicalStale
	}
	if action == "document" || action == "submit" {
		if c.State != "draft" && !(action == "document" && c.State == "revision_required") {
			return TechnicalInvalid("state", "Revise the kit before uploading new technical documents")
		}
	} else if c.State != "under_review" && c.State != "approved" && c.State != "approved_with_comments" {
		return TechnicalInvalid("state", "Complete independent team review before uploading client decision evidence")
	}
	return nil
}
