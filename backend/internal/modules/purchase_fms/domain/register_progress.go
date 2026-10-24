package domain

import (
	"fmt"
	"sort"
)

// Register credit is a project stage: all active preparations must be finalised.
// A partial, reopened or empty preparation cannot complete the project register.
type RegisterState struct {
	LaterCompleted                   []int
	State                            string
	HasItems                         bool
	IndentComplete                   bool
	QuotesComplete, ApprovalComplete bool
	Pending, Returned, Approved      int
}
type RegisterProgress struct {
	Pending, Returned, Approved int
	Percent                     *float64
	Completed                   []int
	Next                        string
}

func RegisterCompletion(records []RegisterState) RegisterProgress {
	out := RegisterProgress{Completed: []int{}, Next: "PUR-01"}
	if len(records) == 0 {
		return out
	}
	for _, r := range records {
		out.Pending += r.Pending
		out.Returned += r.Returned
		out.Approved += r.Approved
	}
	zero := 0.0
	out.Percent = &zero
	for _, r := range records {
		if r.State != "completed" || !r.HasItems {
			return out
		}
	}
	units := progressWeights[0]
	allIndents := true
	for _, r := range records {
		allIndents = allIndents && r.IndentComplete
	}
	if allIndents {
		units += progressWeights[1]
		out.Completed = append(out.Completed, 2)
	}
	allQuotes, allApprovals := allIndents, true
	for _, r := range records {
		allQuotes = allQuotes && r.QuotesComplete
		allApprovals = allApprovals && r.ApprovalComplete
	}
	if allQuotes {
		units += progressWeights[2]
		out.Completed = append(out.Completed, 3)
	}
	if allApprovals {
		units += progressWeights[3]
		out.Completed = append(out.Completed, 4)
	}
	for step := 5; step <= 20; step++ {
		done := true
		for _, r := range records {
			found := false
			for _, n := range r.LaterCompleted {
				found = found || n == step
			}
			done = done && found
		}
		if done {
			units += progressWeights[step-1]
			out.Completed = append(out.Completed, step)
		}
	}
	clientPOComplete := true
	for _, r := range records {
		found := false
		for _, n := range r.LaterCompleted {
			found = found || n == 21
		}
		clientPOComplete = clientPOComplete && found
	}
	if clientPOComplete {
		out.Completed = append(out.Completed, 21)
	}
	earned := progressPercent(units)
	out.Percent = &earned
	out.Completed = append([]int{1}, out.Completed...)
	out.Next = "PUR-02"
	if allIndents {
		out.Next = "PUR-03"
	}
	if allQuotes {
		out.Next = "PUR-04"
	}
	if allApprovals {
		out.Next = "PUR-05"
	}
	if allApprovals {
		for step := 5; step <= 20; step++ {
			found := false
			for _, n := range out.Completed {
				found = found || n == step
			}
			if !found {
				out.Next = fmt.Sprintf("PUR-%02d", step)
				break
			}
			if step == 20 {
				out.Next = "PUR-20"
			}
		}
	}
	if out.Next == "PUR-09" && !clientPOComplete {
		out.Next = "PUR-PO-CLIENT"
	}
	return out
}

// Lock order is independent of the visible dashboard sort, preventing cross-page
// reporting reads from acquiring the same project locks in opposite orders.
func WorkspaceSummaryOrder(rows []WorkspaceRow) []int {
	indices := make([]int, len(rows))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool { return rows[indices[i]].Project.ID < rows[indices[j]].Project.ID })
	return indices
}
