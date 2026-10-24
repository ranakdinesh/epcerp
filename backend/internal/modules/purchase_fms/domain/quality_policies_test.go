package domain

import (
	"strings"
	"testing"
	"time"
)

const qualityProject = "11111111-1111-4111-8111-111111111111"
const qualityPkg = "22222222-2222-4222-8222-222222222222"
const qualityActorID = "33333333-3333-4333-8333-333333333333"
const qualityIdentity = "44444444-4444-4444-8444-444444444444"

func qualitySource() QualitySource {
	return QualitySource{Available: true, Blockers: []string{}, QualitySourceFacts: QualitySourceFacts{ProjectID: qualityProject, PackageID: qualityPkg, ReportID: qualityIdentity, ReportVersion: 6, ReportRevision: 3, IRNumber: "Actual IR", IRNNumber: "Actual IRN", IssueDate: "2026-10-10", Quantity: "1.000001", UOM: "metre", OrderID: qualityIdentity, OrderVersion: 2, Number: "Actual PO", SupplierName: "Actual supplier", Description: "Inspected material", InspectionID: qualityIdentity, InspectionVersion: 8}}
}
func qualityOwner() Principal { return Principal{ID: qualityActorID, Active: true} }
func qualityCaps() QualityAccess {
	return QualityAccess{Read: true, Write: true, Verify: true, Scoped: true, Work: true}
}
func qualityNow() QualityTimestamp { return time.Date(2026, 10, 11, 5, 0, 0, 0, time.UTC) }
func qualityApply(t *testing.T, r *QualityRecord, q QualityCommand) QualityRecord {
	t.Helper()
	if r != nil {
		q.Version = r.Version
	}
	q.Reason = "Actual certificate work"
	next, e := QualityApply(r, qualityOwner(), qualityCaps(), q, qualitySource(), qualityNow(), qualityIdentity)
	if e != nil {
		t.Fatal(e)
	}
	return next
}
func qualityAll(t *testing.T) QualityRecord {
	t.Helper()
	var r *QualityRecord
	for _, d := range qualityDefinitions {
		next := qualityApply(t, r, QualityCommand{Action: "record", Code: d.code, Details: &QualityDetails{Number: "Actual " + d.code, Issuer: "Actual issuing laboratory", ValidUntil: "2026-10-11", DocumentReference: "Actual signed reference"}})
		next = qualityApply(t, &next, QualityCommand{Action: "verify", Code: d.code})
		r = &next
	}
	return *r
}
func TestQualityQualityFiveRequiredRecordsAndMetadataVerificationSeparation(t *testing.T) {
	r := qualityApply(t, nil, QualityCommand{Action: "record", Code: "TC", Details: &QualityDetails{Number: "Actual TC", Issuer: "Actual lab"}})
	if len(r.Certificates) != 5 || r.Certificates[0].Verified || r.Certificates[1].Number != "" {
		t.Fatal("invented evidence")
	}
	for _, c := range r.Certificates {
		if !c.Active || c.CreatedBy != qualityActorID || c.CreatedAt.IsZero() {
			t.Fatal("missing child audit")
		}
	}
	ready := QualityReadiness(&r, qualitySource(), QualityToday(qualityNow()))
	if ready.Ready || ready.RequiredCount != 5 || ready.VerifiedCount != 0 {
		t.Fatal(ready)
	}
	r = qualityApply(t, &r, QualityCommand{Action: "verify", Code: "TC"})
	if !r.Certificates[0].Verified || r.Certificates[0].VerifiedBy != qualityActorID || r.Certificates[0].VerifiedAt == nil {
		t.Fatal("no verifier attribution")
	}
	r = qualityApply(t, &r, QualityCommand{Action: "record", Code: "TC", Details: &QualityDetails{Number: "Corrected actual TC", Issuer: "Actual lab"}})
	if r.Certificates[0].Verified || r.Certificates[0].VerifiedAt != nil || r.Certificates[0].VerifiedBy != "" {
		t.Fatal("changed details retained attestation")
	}
	r = qualityApply(t, &r, QualityCommand{Action: "verify", Code: "TC"})
	r = qualityApply(t, &r, QualityCommand{Action: "revoke", Code: "TC"})
	if r.Certificates[0].Verified || r.Certificates[0].VerifiedBy != "" {
		t.Fatal("revocation retained attestation")
	}
}
func TestQualityQualityCurrentEligibilityExpiryRevisionAndDefensiveCopies(t *testing.T) {
	r := qualityAll(t)
	f, e := QualityVerifiedFact(&r, qualitySource(), "2026-10-11")
	if e != nil || f == nil || !QualityReadiness(&r, qualitySource(), "2026-10-11").Ready {
		t.Fatal("current quality not ready", e)
	}
	*f.Certificates[0].VerifiedAt = f.Certificates[0].VerifiedAt.AddDate(0, 0, 1)
	if QualityToday(*r.Certificates[0].VerifiedAt) != "2026-10-11" {
		t.Fatal("fact timestamp alias")
	}
	if _, e = QualityVerifiedFact(&r, qualitySource(), "2026-10-12"); e == nil {
		t.Fatal("expired verification reused")
	}
	changed := qualitySource()
	changed.ReportVersion++
	if _, e = QualityVerifiedFact(&r, changed, "2026-10-11"); e == nil {
		t.Fatal("obsolete report reused")
	}
	next, e := QualityApply(&r, qualityOwner(), qualityCaps(), QualityCommand{Version: r.Version, Action: "revise", Reason: "Actual changed IRN"}, changed, qualityNow(), qualityIdentity)
	if e != nil || next.Revision != 2 || next.Source.ReportVersion != changed.ReportVersion {
		t.Fatal(e)
	}
	for i, c := range next.Certificates {
		if c.Verified || c.VerifiedAt != nil || c.VerifiedBy != "" || c.Number != r.Certificates[i].Number {
			t.Fatal("revision lost metadata or retained attestation")
		}
	}
	if !r.Certificates[0].Verified {
		t.Fatal("revision changed old evidence")
	}
	for _, mutate := range []func(*QualityRecord){func(r *QualityRecord) { r.Version = 0 }, func(r *QualityRecord) { r.Certificates = r.Certificates[:4] }, func(r *QualityRecord) { r.Certificates[0].VerifiedBy = "forged" }, func(r *QualityRecord) { r.Certificates[0].Title = "Fake optional certificate" }} {
		bad := QualityCopy(r)
		mutate(&bad)
		if _, e = QualityVerifiedFact(&bad, qualitySource(), "2026-10-11"); e == nil {
			t.Fatal("invalid quality fact allowed")
		}
	}
}
func TestQualityQualityCommandsPermissionAndValidation(t *testing.T) {
	q := QualityCommand{Action: "record", Code: "TC", Reason: "Actual certificate", Details: &QualityDetails{Number: "Actual TC", Issuer: "Actual lab"}}
	for _, c := range []QualityAccess{{Write: true, Work: true}, {Read: true, Write: true}, {Read: true, Verify: true, Work: true}} {
		if _, e := QualityApply(nil, qualityOwner(), c, q, qualitySource(), qualityNow(), qualityIdentity); e != QualityForbidden {
			t.Fatal("write permission bypass", e)
		}
	}
	r := qualityApply(t, nil, q)
	writeOnly := qualityCaps()
	writeOnly.Verify = false
	if _, e := QualityApply(&r, qualityOwner(), writeOnly, QualityCommand{Version: 1, Action: "verify", Code: "TC", Reason: "r"}, qualitySource(), qualityNow(), qualityIdentity); e != QualityForbidden {
		t.Fatal("write grant verified quality", e)
	}
	for _, q := range []QualityCommand{{Action: "verify", Code: "MTC", Reason: "r"}, {Action: "record", Code: "NA", Reason: "r", Details: &QualityDetails{}}, {Action: "record", Code: "TC", Reason: "r", Details: &QualityDetails{ValidUntil: "2026-02-30"}}, {Action: "record", Code: "TC", Reason: "r", Details: &QualityDetails{Issuer: strings.Repeat("x", 256)}}, {Action: "verify", Code: "TC", Reason: "r", Details: &QualityDetails{}}, {Action: "revise", Code: "TC", Reason: "r"}, {Action: "revoke", Code: "TC", Reason: "r"}} {
		q.Version = r.Version
		if _, e := QualityApply(&r, qualityOwner(), qualityCaps(), q, qualitySource(), qualityNow(), qualityIdentity); e == nil {
			t.Fatal("invalid quality command allowed", q.Action)
		}
	}
	expired := qualityApply(t, &r, QualityCommand{Action: "record", Code: "TC", Details: &QualityDetails{Number: "Actual expired TC", Issuer: "Actual lab", ValidUntil: "2026-10-10"}})
	if _, e := QualityApply(&expired, qualityOwner(), qualityCaps(), QualityCommand{Version: expired.Version, Action: "verify", Code: "TC", Reason: "r"}, qualitySource(), qualityNow(), qualityIdentity); e == nil {
		t.Fatal("expired certificate verified")
	}
	held := qualitySource()
	held.Available = false
	if _, e := QualityApply(&r, qualityOwner(), qualityCaps(), q, held, qualityNow(), qualityIdentity); e == nil {
		t.Fatal("held quality write allowed")
	}
	q.Version = 0
	if _, e := QualityApply(&r, qualityOwner(), qualityCaps(), q, qualitySource(), qualityNow(), qualityIdentity); e != QualityStale {
		t.Fatal("stale quality version accepted", e)
	}
}
