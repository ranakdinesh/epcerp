package domain

import (
	"strings"
)

// Progress reports verified evidence; it grants no downstream dispatch authority.
type QualityProgress struct {
	ID                           string
	Version, Revision            int32
	State, Details               string
	Current                      bool
	VerifiedCount, RequiredCount int
	Snapshot                     *QualityRecord `json:"-"`
}

func QualityProgressFact(r *QualityRecord, expected QualitySourceFacts, upstreamCurrent bool, today string) QualityProgress {
	out := QualityProgress{State: "not_started", RequiredCount: len(qualityDefinitions)}
	if r == nil || !r.Active {
		return out
	}
	out.ID, out.Version, out.Revision = r.ID, r.Version, r.Revision
	source := QualitySource{QualitySourceFacts: expected, Available: true}
	if !upstreamCurrent || source.Guard() != nil || r.ProjectID != expected.ProjectID || r.PackageID != expected.PackageID || r.Source.QualitySourceFacts != expected {
		out.State, out.Details = "blocked", "Quality evidence does not match the current issued IR / IRN"
		return out
	}
	out.Current = true
	ready := QualityReadiness(r, source, today)
	out.VerifiedCount, out.RequiredCount = ready.VerifiedCount, ready.RequiredCount
	if ready.Ready {
		snapshot := QualityCopy(*r)
		out.State, out.Details, out.Snapshot = "completed", "All required certificates are verified and valid", &snapshot
		return out
	}
	out.Details = strings.Join(ready.Blockers, "; ")
	if !QualityDate(today) || r.Version < 1 || r.Revision < 1 || QualityID(r.ID) != nil || len(r.Certificates) != len(qualityDefinitions) {
		out.State = "blocked"
		return out
	}
	flags, seen := 0, map[string]bool{}
	for _, cert := range r.Certificates {
		if !qualityCode(cert.Code) || seen[cert.Code] {
			out.State = "blocked"
			return out
		}
		seen[cert.Code] = true
		if cert.Verified {
			flags++
		}
	}
	if flags > ready.VerifiedCount {
		out.State = "blocked"
	} else if ready.VerifiedCount > 0 {
		out.State = "in_progress"
	}
	return out
}
