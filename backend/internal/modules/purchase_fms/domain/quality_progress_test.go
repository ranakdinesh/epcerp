package domain

import (
	"testing"
)

func TestQualityQualityProgressRequiresValidAttributedCurrentEvidence(t *testing.T) {
	s := qualitySource()
	r := qualityAll(t)
	p := QualityProgressFact(&r, s.QualitySourceFacts, true, QualityToday(qualityNow()))
	if p.State != "completed" || !p.Current || p.VerifiedCount != 5 || p.Snapshot == nil {
		t.Fatal(p)
	}
	p.Snapshot.Certificates[0].Number = "changed"
	*p.Snapshot.Certificates[0].VerifiedAt = QualityTimestamp{}
	if r.Certificates[0].Number == "changed" || r.Certificates[0].VerifiedAt.IsZero() {
		t.Fatal("aliased certificate")
	}
	s.Available = false
	s.Blockers = []string{"Held project"}
	if QualityProgressFact(&r, s.QualitySourceFacts, true, QualityToday(qualityNow())).State != "completed" {
		t.Fatal("hold erased valid attestation")
	}
	if _, err := QualityVerifiedFact(&r, s, QualityToday(qualityNow())); err == nil {
		t.Fatal("report bypassed hold")
	}
	changed := s.QualitySourceFacts
	changed.ReportVersion++
	if p := QualityProgressFact(&r, changed, true, QualityToday(qualityNow())); p.State != "blocked" || p.Current {
		t.Fatal("obsolete IRN earned verification", p)
	}
	if p := QualityProgressFact(&r, s.QualitySourceFacts, true, "2026-10-12"); p.State != "blocked" || p.Snapshot != nil {
		t.Fatal("expired certificate earned progress", p)
	}
	r.Certificates[0].VerifiedBy = ""
	if p := QualityProgressFact(&r, s.QualitySourceFacts, true, QualityToday(qualityNow())); p.State != "blocked" || p.Snapshot != nil {
		t.Fatal("unattributed verification earned progress", p)
	}
}
func TestQualityQualityProgressShowsPartialWorkAndResetsOnRevision(t *testing.T) {
	r := qualityApply(t, nil, QualityCommand{Action: "record", Code: "TC", Details: &QualityDetails{Number: "Actual TC", Issuer: "Actual lab"}})
	if p := QualityProgressFact(&r, qualitySource().QualitySourceFacts, true, QualityToday(qualityNow())); p.State != "not_started" || p.VerifiedCount != 0 {
		t.Fatal(p)
	}
	r = qualityApply(t, &r, QualityCommand{Action: "verify", Code: "TC"})
	if p := QualityProgressFact(&r, qualitySource().QualitySourceFacts, true, QualityToday(qualityNow())); p.State != "in_progress" || p.VerifiedCount != 1 || p.Snapshot != nil {
		t.Fatal(p)
	}
	changed := qualitySource()
	changed.ReportVersion++
	var err error
	r, err = QualityApply(&r, qualityOwner(), qualityCaps(), QualityCommand{Version: r.Version, Action: "revise", Reason: "Review changed actual IRN"}, changed, qualityNow(), qualityIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if p := QualityProgressFact(&r, changed.QualitySourceFacts, true, QualityToday(qualityNow())); p.State != "not_started" || p.VerifiedCount != 0 {
		t.Fatal(p)
	}
}
