package domain

import (
	"fmt"
	"math/big"
	"strings"
)

// GroupedSource is normalized owner evidence supplied by internal adapters only.
// Snapshot is a canonical complete item snapshot, not a user-supplied fingerprint.
type ProgressGroupedLine struct{ ID, Quantity, RequiredDate, Snapshot string }
type ProgressGroupedGroup struct {
	ID                 string
	Revision           int32
	LineIDs, VendorIDs []string
}
type ProgressGroupedDocument struct {
	ID, GroupID, VendorID, SentDate string
	Version, Revision               int32
	Active                          bool
	Lines                           []ProgressGroupedLine
}
type ProgressGroupedQuote struct {
	DocumentID      string
	DocumentVersion int32
	Active          bool
	PricedLineIDs   []string
}
type ProgressGroupedSource struct {
	BOState, GroupState string
	Lines               []ProgressGroupedLine
	Groups              []ProgressGroupedGroup
	Documents           []ProgressGroupedDocument
	Quotes              []ProgressGroupedQuote
	ApprovedGroupIDs    []string
	ApprovalStarted     bool
}

func progressExactIDs(values []string) (map[string]bool, bool) {
	out := map[string]bool{}
	for _, v := range values {
		if v == "" || out[v] {
			return out, false
		}
		out[v] = true
	}
	return out, true
}
func progressGroupedQuantity(v string) (*big.Int, error) {
	parts := strings.Split(v, ".")
	if len(parts) != 2 || parts[1] == "" {
		return progressQuantity(v)
	}
	fraction := strings.TrimRight(parts[1], "0")
	normalized := parts[0]
	if fraction != "" {
		normalized += "." + fraction
	}
	return progressQuantity(normalized)
}

// GroupedLineMatches compares exact quantities and the complete published item/date scope.
func ProgressGroupedLineMatches(a, b ProgressGroupedLine) bool {
	x, err := progressGroupedQuantity(a.Quantity)
	if err != nil {
		return false
	}
	y, err := progressGroupedQuantity(b.Quantity)
	if err != nil {
		return false
	}
	if _, err := progressDate(a.RequiredDate); err != nil {
		return false
	}
	return a.ID == b.ID && x.Cmp(y) == 0 && a.Snapshot != "" && a.Snapshot == b.Snapshot && a.RequiredDate == b.RequiredDate
}
func ProgressGroupedFacts(s ProgressGroupedSource) ([]ProgressEvidence, error) {
	if s.Groups == nil {
		return []ProgressEvidence{}, nil
	}
	lines := map[string]ProgressGroupedLine{}
	for _, line := range s.Lines {
		if line.ID == "" || lines[line.ID].ID != "" {
			return nil, ProgressInvalid("groups", "Use distinct current BO lines")
		}
		lines[line.ID] = line
	}
	groups := map[string]ProgressGroupedGroup{}
	assigned := map[string]bool{}
	scopeComplete, vendorsReady := len(lines) > 0, true
	type pair struct{ group, vendor string }
	expected := map[pair]ProgressGroupedGroup{}
	for _, g := range s.Groups {
		if g.ID == "" || groups[g.ID].ID != "" || g.Revision < 1 {
			return nil, ProgressInvalid("groups", "Use distinct current group revisions")
		}
		groups[g.ID] = g
		lineIDs, unique := progressExactIDs(g.LineIDs)
		if !unique || len(lineIDs) == 0 {
			scopeComplete = false
		}
		for id := range lineIDs {
			if assigned[id] || lines[id].ID == "" {
				scopeComplete = false
			}
			assigned[id] = true
		}
		vendors, unique := progressExactIDs(g.VendorIDs)
		if !unique || len(vendors) < 3 {
			vendorsReady = false
		}
		for vendor := range vendors {
			expected[pair{g.ID, vendor}] = g
		}
	}
	for id := range lines {
		if !assigned[id] {
			scopeComplete = false
		}
	}
	issued := s.BOState == "completed" && s.GroupState == "created" && scopeComplete && vendorsReady && len(expected) > 0
	current := map[pair]ProgressGroupedDocument{}
	documentIDs := map[string]bool{}
	for _, doc := range s.Documents {
		if !doc.Active {
			continue
		}
		key := pair{doc.GroupID, doc.VendorID}
		g, exists := expected[key]
		if !exists || doc.Revision != g.Revision || doc.ID == "" || doc.Version < 1 {
			continue
		}
		needed, valid := progressExactIDs(g.LineIDs)
		if !valid || len(doc.Lines) != len(needed) {
			continue
		}
		seen := map[string]bool{}
		for _, line := range doc.Lines {
			if seen[line.ID] || !needed[line.ID] || !ProgressGroupedLineMatches(line, lines[line.ID]) {
				valid = false
			}
			seen[line.ID] = true
		}
		if !valid {
			continue
		}
		if _, duplicate := current[key]; duplicate || documentIDs[doc.ID] {
			return nil, ProgressInvalid("documents", "Multiple current vendor documents cannot supply one scope")
		}
		current[key] = doc
		documentIDs[doc.ID] = true
	}
	quotes := map[string]ProgressGroupedQuote{}
	for _, quote := range s.Quotes {
		if !quote.Active {
			continue
		}
		if _, exists := quotes[quote.DocumentID]; exists {
			return nil, ProgressInvalid("quotations", "Use one current quotation per document")
		}
		quotes[quote.DocumentID] = quote
	}
	sent, received := 0, 0
	for _, doc := range current {
		if _, err := progressDate(doc.SentDate); err != nil {
			continue
		}
		sent++
		quote, exists := quotes[doc.ID]
		if !exists || quote.DocumentVersion != doc.Version {
			continue
		}
		priced, valid := progressExactIDs(quote.PricedLineIDs)
		if !valid || len(priced) != len(doc.Lines) {
			continue
		}
		for _, line := range doc.Lines {
			if !priced[line.ID] {
				valid = false
			}
		}
		if valid {
			received++
		}
	}
	approved, unique := progressExactIDs(s.ApprovedGroupIDs)
	if !unique {
		return nil, ProgressInvalid("approvals", "Use distinct current approved groups")
	}
	for id := range approved {
		if groups[id].ID == "" {
			return nil, ProgressInvalid("approvals", "Approval cannot refer to another group scope")
		}
	}
	assignedCount := 0
	for id := range lines {
		if assigned[id] {
			assignedCount++
		}
	}
	out := []ProgressEvidence{{Number: 2, State: "in_progress", Details: fmt.Sprintf("%d/%d items grouped; %d/%d vendor indents generated", assignedCount, len(lines), len(current), len(expected))}, {Number: 3, State: "not_started", Details: fmt.Sprintf("%d/%d invitations sent; %d/%d quotations fully priced", sent, len(expected), received, len(expected))}, {Number: 4, State: "not_started", Details: fmt.Sprintf("%d/%d groups approved by Accounts", len(approved), len(groups))}}
	if issued && len(current) == len(expected) {
		out[0].State = "completed"
	}
	if issued && sent == len(expected) && received == len(expected) {
		out[1].State = "completed"
	} else if sent > 0 || received > 0 {
		out[1].State = "in_progress"
	}
	if len(groups) > 0 && len(approved) == len(groups) {
		out[2].State = "completed"
	} else if s.ApprovalStarted || len(approved) > 0 {
		out[2].State = "in_progress"
	}
	return out, nil
}
