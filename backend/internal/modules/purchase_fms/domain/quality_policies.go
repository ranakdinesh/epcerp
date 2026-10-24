package domain

import (
	"fmt"
	"math/big"
	"regexp"
)

var qualityDecimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$`)

func (s QualitySource) Guard() error {
	if !s.Available || len(s.Blockers) > 0 {
		return QualityInvalid("source", "Resolve project, package and current issued-IRN blockers first")
	}
	for _, id := range []string{s.ProjectID, s.PackageID, s.ReportID, s.OrderID, s.InspectionID} {
		if e := QualityID(id); e != nil {
			return e
		}
	}
	qty, ok := new(big.Rat).SetString(s.Quantity)
	if s.ReportVersion < 1 || s.ReportRevision < 1 || s.OrderVersion < 2 || s.InspectionVersion < 1 || !QualityDate(s.IssueDate) || s.IRNumber == "" || s.IRNNumber == "" || s.UOM == "" || s.Number == "" || s.SupplierName == "" || s.Description == "" || !qualityDecimal.MatchString(s.Quantity) || !ok || qty.Sign() <= 0 {
		return QualityInvalid("source", "Use the actual issued report and cleared material quantity")
	}
	return nil
}
func QualityAccessGuard(a Principal, c QualityAccess, write bool) error {
	if !a.Active || a.ID == "" {
		return QualityUnauthorized
	}
	if !c.Read || write && (!c.Work || !c.Write) {
		return QualityForbidden
	}
	return nil
}
func QualityActionAccess(a Principal, c QualityAccess, action string) error {
	if e := QualityAccessGuard(a, c, false); e != nil {
		return e
	}
	if !c.Work {
		return QualityForbidden
	}
	switch action {
	case "record", "revise":
		if !c.Write {
			return QualityForbidden
		}
	case "verify", "revoke":
		if !c.Verify {
			return QualityForbidden
		}
	default:
		return QualityInvalid("action", "Choose record, verify, revoke or revise")
	}
	return nil
}
func QualityValidate(q QualityCommand) (QualityCommand, error) {
	if q.Version < 0 {
		return q, QualityInvalid("version", "Use the current nonnegative quality version")
	}
	var e error
	q.Reason, e = QualityText("reason", q.Reason, 2000, true)
	if e != nil {
		return q, e
	}
	switch q.Action {
	case "record", "verify", "revoke":
		if !qualityCode(q.Code) {
			return q, QualityInvalid("certificate_code", "Choose a required certificate type")
		}
		if q.Action == "record" && q.Details == nil {
			return q, QualityInvalid("details", "Supply certificate details")
		}
		if q.Action != "record" && q.Details != nil {
			return q, QualityInvalid("details", "Save details before recording a verification decision")
		}
	case "revise":
		if q.Code != "" || q.Details != nil {
			return q, QualityInvalid("revision", "Revision carries only version and reason")
		}
	default:
		return q, QualityInvalid("action", "Choose record, verify, revoke or revise")
	}
	return q, nil
}
func qualityDetails(d QualityDetails) (QualityDetails, error) {
	for _, v := range []struct {
		f string
		p *string
		n int
	}{{"cert_number", &d.Number, 255}, {"issued_by", &d.Issuer, 255}, {"file_name", &d.DocumentReference, 500}} {
		var e error
		*v.p, e = QualityText(v.f, *v.p, v.n, false)
		if e != nil {
			return d, e
		}
	}
	if d.ValidUntil != "" && !QualityDate(d.ValidUntil) {
		return d, QualityInvalid("valid_until", "Use a real certificate validity date")
	}
	return d, nil
}
func qualityVerification(c QualityCertificate, today string) error {
	if !c.Active || c.Number == "" || c.Issuer == "" {
		return QualityInvalid("certificate", "Save the actual certificate number and issuer before verification")
	}
	if c.ValidUntil != "" && (!QualityDate(c.ValidUntil) || c.ValidUntil < today) {
		return QualityInvalid("valid_until", "Expired certificates cannot satisfy current quality readiness")
	}
	return nil
}
func QualityCan(r *QualityRecord, a Principal, c QualityAccess, q QualityCommand, s QualitySource, today string) error {
	if e := QualityActionAccess(a, c, q.Action); e != nil {
		return e
	}
	if e := s.Guard(); e != nil {
		return e
	}
	if r == nil {
		if q.Action != "record" {
			return QualityInvalid("certificate", "Record certificate details first")
		}
		return nil
	}
	if !r.Active || r.ProjectID != s.ProjectID || r.PackageID != s.PackageID {
		return QualityMissing
	}
	if q.Action == "revise" {
		if r.Source.QualitySourceFacts == s.QualitySourceFacts {
			return QualityInvalid("revision", "Current source is unchanged; edit or revoke a certificate instead")
		}
		return nil
	}
	if r.Source.QualitySourceFacts != s.QualitySourceFacts {
		return QualityStale
	}
	if q.Action == "record" {
		return nil
	}
	for _, cert := range r.Certificates {
		if cert.Code == q.Code {
			if q.Action == "verify" {
				if cert.Verified {
					return QualityInvalid("certificate", "This certificate is already verified")
				}
				return qualityVerification(cert, today)
			}
			if !cert.Verified {
				return QualityInvalid("certificate", "This certificate is not verified")
			}
			return nil
		}
	}
	return QualityMissing
}
func QualityApply(base *QualityRecord, a Principal, c QualityAccess, q QualityCommand, s QualitySource, now QualityTimestamp, id string) (QualityRecord, error) {
	empty := QualityRecord{}
	q, e := QualityValidate(q)
	if e != nil {
		return empty, e
	}
	if e = QualityCan(base, a, c, q, s, QualityToday(now)); e != nil {
		return empty, e
	}
	out := QualityRecord{ID: id, ProjectID: s.ProjectID, PackageID: s.PackageID, Revision: 1, Source: s, Certificates: QualityDefaults(a, now), Active: true, CreatedAt: now, CreatedBy: a.ID}
	if base != nil {
		if q.Version != base.Version {
			return empty, QualityStale
		}
		out = QualityCopy(*base)
	} else if q.Version != 0 {
		return empty, QualityStale
	}
	for _, id := range []string{out.ID, a.ID} {
		if e = QualityID(id); e != nil {
			return empty, e
		}
	}
	if q.Action == "revise" {
		out.Revision++
		out.Source = s
		for i := range out.Certificates {
			cert := &out.Certificates[i]
			cert.Verified = false
			cert.VerifiedAt = nil
			cert.VerifiedBy = ""
			cert.UpdatedAt = now
			cert.UpdatedBy = a.ID
		}
	} else {
		found := false
		for i := range out.Certificates {
			cert := &out.Certificates[i]
			if cert.Code != q.Code {
				continue
			}
			found = true
			switch q.Action {
			case "record":
				cert.QualityDetails, e = qualityDetails(*q.Details)
				if e != nil {
					return empty, e
				}
				cert.Verified = false
				cert.VerifiedAt = nil
				cert.VerifiedBy = ""
			case "verify":
				at := now
				cert.Verified = true
				cert.VerifiedAt = &at
				cert.VerifiedBy = a.ID
			case "revoke":
				cert.Verified = false
				cert.VerifiedAt = nil
				cert.VerifiedBy = ""
			}
			cert.UpdatedAt = now
			cert.UpdatedBy = a.ID
		}
		if !found {
			return empty, QualityMissing
		}
	}
	out.Version = q.Version + 1
	out.UpdatedAt = now
	out.UpdatedBy = a.ID
	return out, nil
}
func QualityReadiness(r *QualityRecord, s QualitySource, today string) QualityEligibility {
	out := QualityEligibility{RequiredCount: len(qualityDefinitions), Blockers: []string{}}
	if !QualityDate(today) {
		out.Blockers = append(out.Blockers, "Use a valid current quality-check date")
	}
	if e := s.Guard(); e != nil {
		out.Blockers = append(out.Blockers, e.Error())
	}
	if r == nil || !r.Active {
		out.Blockers = append(out.Blockers, "Record the five required quality certificates")
		return out
	}
	if r.Version < 1 || r.Revision < 1 || QualityID(r.ID) != nil {
		out.Blockers = append(out.Blockers, "Use a persisted quality revision")
	}
	if r.ProjectID != s.ProjectID || r.PackageID != s.PackageID || r.Source.QualitySourceFacts != s.QualitySourceFacts {
		out.Blockers = append(out.Blockers, "Quality verification belongs to an obsolete report; start a revision")
	}
	for _, d := range qualityDefinitions {
		found := false
		for _, cert := range r.Certificates {
			if cert.Code != d.code {
				continue
			}
			if found {
				out.Blockers = append(out.Blockers, "Duplicate required certificate type")
			}
			found = true
			if cert.Title != d.title {
				out.Blockers = append(out.Blockers, d.code+": required certificate identity changed")
			}
			if !cert.Verified || cert.VerifiedAt == nil || cert.VerifiedBy == "" {
				out.Blockers = append(out.Blockers, fmt.Sprintf("%s: verification is pending", d.code))
				continue
			}
			if QualityID(cert.VerifiedBy) != nil || QualityToday(*cert.VerifiedAt) > today || cert.VerifiedAt.Before(cert.CreatedAt) {
				out.Blockers = append(out.Blockers, d.code+": verification attribution/date is invalid")
				continue
			}
			if e := qualityVerification(cert, today); e != nil {
				out.Blockers = append(out.Blockers, fmt.Sprintf("%s: %s", d.code, e.Error()))
				continue
			}
			out.VerifiedCount++
		}
		if !found {
			out.Blockers = append(out.Blockers, d.code+": certificate is missing")
		}
	}
	if len(r.Certificates) != len(qualityDefinitions) {
		out.Blockers = append(out.Blockers, "Retain the five required certificate records")
	}
	out.Ready = len(out.Blockers) == 0 && out.VerifiedCount == out.RequiredCount
	return out
}
func QualityVerifiedFact(r *QualityRecord, s QualitySource, today string) (*QualityRecord, error) {
	ready := QualityReadiness(r, s, today)
	if !ready.Ready {
		return nil, QualityInvalid("quality", "Current required certificates must all be verified and valid")
	}
	out := QualityCopy(*r)
	return &out, nil
}
