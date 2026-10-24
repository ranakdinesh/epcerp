package domain

import (
	"slices"
)

// Progress is reporting evidence, not a Clearance or permission to issue a PO.
// UpstreamCurrent is established by the report's transaction-bound LOI projection;
// current holds do not erase submitted/approved evidence with unchanged links.
type TechnicalProgressStep struct {
	Number         int
	State, Details string
}
type TechnicalProgress struct {
	CaseID            string
	Version, Revision int32
	Current           bool
	Steps             []TechnicalProgressStep
	Snapshot          *TechnicalCase `json:"-"`
}

func TechnicalProgressFacts(c *TechnicalCase, current TechnicalSource, files []TechnicalFile, upstreamCurrent bool) TechnicalProgress {
	out := TechnicalProgress{Steps: []TechnicalProgressStep{{Number: 6, State: "not_started"}, {Number: 7, State: "not_started"}, {Number: 8, State: "not_started"}}}
	if c == nil || !c.Active {
		return out
	}
	out.CaseID = c.ID
	out.Version = c.Version
	out.Revision = c.Revision
	if !upstreamCurrent || !technicalSameSource(c.Source, current) {
		for i := range out.Steps {
			out.Steps[i].State = "blocked"
			out.Steps[i].Details = "Technical evidence does not match the current upstream LOI"
		}
		return out
	}
	out.Current = true
	snapshot := TechnicalCopy(*c)
	out.Snapshot = &snapshot
	required, received, anyReceived := 0, 0, false
	for _, d := range c.Documents {
		if !d.Active {
			continue
		}
		valid := d.Status == "received" && TechnicalFileGuard(files, d.FileID, c.ID, "document") == nil
		anyReceived = anyReceived || valid
		if d.Required {
			required++
			if valid {
				received++
			}
		}
	}
	kit := required > 0 && received == required
	if kit {
		out.Steps[0].State = "completed"
		out.Steps[0].Details = "Required technical documents received with current file evidence"
	} else if anyReceived {
		out.Steps[0].State = "in_progress"
		out.Steps[0].Details = "Required technical kit remains incomplete"
	}
	submitted := kit && c.Submission != nil && c.Submission.Revision == c.Revision && slices.Equal(c.Documents, c.Submission.Documents) && TechnicalFileGuard(files, c.Submission.FileID, c.ID, "transmittal") == nil
	if submitted {
		out.Steps[1].State = "completed"
		out.Steps[1].Details = c.Submission.Reference
	}
	if c.State == "revision_required" {
		out.Steps[1].State = "blocked"
		out.Steps[1].Details = "Project team requires a revised technical submission"
		if c.TeamReview != nil && c.TeamReview.Remarks != "" {
			out.Steps[1].Details = c.TeamReview.Remarks
		}
	}
	if TechnicalReadyWithEvidence(*c, files) {
		out.Steps[2].State = "completed"
		out.Steps[2].Details = c.Approval.Reference
	} else if c.State == "rejected" {
		out.Steps[2].State = "blocked"
		out.Steps[2].Details = "Client technical approval rejected"
		if c.Approval != nil && c.Approval.Remarks != "" {
			out.Steps[2].Details = c.Approval.Remarks
		}
	} else if submitted || c.TeamReview != nil || c.Approval != nil {
		out.Steps[2].State = "in_progress"
		out.Steps[2].Details = "Client approval, signed QAP or condition closure remains pending"
	}
	return out
}
