package domain

type QuotationWorkTask struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	BOID        string `json:"bo_id"`
	GroupID     string `json:"group_id"`
	Kind        string `json:"kind"`
	State       string `json:"state"`
	Title       string `json:"title"`
	Link        string `json:"link"`
}

type QuotationProjectGroup struct {
	BOID         string `json:"bo_id"`
	GroupID      string `json:"group_id"`
	Name         string `json:"name"`
	State        string `json:"state"`
	Message      string `json:"message"`
	ReturnAction string `json:"return_action"`
}

type QuotationWorkNotice struct{ Recipient, Title, Link string }
type QuotationWorkEntry struct{ ID, Recipient, Kind, State, Title, Link string }
type QuotationWorkPlan struct {
	EventID, ProjectID, BOID, GroupID, AttemptID, ActorID string
	Notices                                               []QuotationWorkNotice
	Create                                                []QuotationWorkEntry
	ResolveGroup, ResolveAttempt                          bool
	Return                                                *QuotationWorkEntry
}

func QuotationApprovalWork(e QuotationApprovalsEvent, a QuotationApprovalsAttempt, recipients []string) QuotationWorkPlan {
	plan := QuotationWorkPlan{EventID: e.ID, ProjectID: e.ProjectID, BOID: e.BOID, GroupID: a.GroupID, AttemptID: a.ID, ActorID: e.ActorID}
	link := "/dashboard?screen=quotation-approvals&project=" + e.ProjectID + "&bo=" + e.BOID + "&group=" + a.GroupID + "&attempt=" + a.ID
	executiveLink := "/dashboard?screen=purchase-fms&project=" + e.ProjectID + "&step=PUR-04"
	title := "Quotation shortlist submitted: " + e.ProjectLabel + " · " + a.GroupName
	if e.Action != "submit" {
		link = executiveLink
		title = "Quotation " + a.State + ": " + e.ProjectLabel + " · " + a.GroupName + " · " + a.ReturnAction
	}
	if text := []rune(title); len(text) > 300 {
		title = string(text[:300])
	}
	for _, id := range recipients {
		plan.Notices = append(plan.Notices, QuotationWorkNotice{id, title, link})
	}
	if e.Action == "submit" {
		plan.ResolveGroup = true
		for _, id := range recipients {
			plan.Create = append(plan.Create, QuotationWorkEntry{Recipient: id, Kind: "finance", State: "pending", Title: "Review quotation shortlist · " + a.GroupName, Link: link})
		}
		plan.Create = append(plan.Create, QuotationWorkEntry{Recipient: a.SubmittedBy, Kind: "executive", State: "waiting", Title: "Waiting for Finance · " + a.GroupName, Link: executiveLink})
	} else {
		plan.ResolveAttempt = true
		if a.State == "rejected" {
			step := "PUR-03"
			if a.ReturnAction == "change_vendor" {
				step = "PUR-02"
			}
			plan.Return = &QuotationWorkEntry{Recipient: a.SubmittedBy, Kind: "executive", State: a.ReturnAction, Title: "Returned quotation · " + a.GroupName, Link: "/dashboard?screen=purchase-fms&project=" + e.ProjectID + "&step=" + step + "&bo=" + e.BOID + "&group=" + a.GroupID + func() string {
				if a.ReturnAction == "change_vendor" {
					return "&indentTab=vendors"
				}
				return "&quote=" + a.QuoteKey
			}()}
		}
	}
	return plan
}
