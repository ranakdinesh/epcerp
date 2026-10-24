package domain

import (
	"strconv"
)

type IndentsRequirement struct {
	IndentsSource
	ID                string `json:"id"`
	ReleasedQuantity  string `json:"released_quantity"`
	RemainingQuantity string `json:"remaining_quantity"`
}
type IndentsCandidate struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
type IndentsWorkspaceSource struct {
	Currency string               `json:"currency"`
	Lines    []IndentsRequirement `json:"lines"`
	Members  []IndentsCandidate   `json:"members"`
}
type IndentsWorkspace struct {
	IndentsWorkspaceSource
	Rows        []IndentsRecord `json:"rows"`
	Permissions map[string]bool `json:"permissions"`
}

func IndentsRemaining(total string, records []IndentsRecord, demand string) (string, string, error) {
	q, e := IndentsQuantity(total)
	if e != nil {
		return "", "", e
	}
	used := int64(0)
	for _, record := range records {
		for _, release := range record.Data.Released {
			if release.DemandID != demand {
				continue
			}
			n, e := IndentsQuantity(release.Quantity)
			if e != nil {
				return "", "", e
			}
			if n > int64(9223372036854775807)-used {
				return "", "", IndentsInvalid("quantity", "Released quantity exceeds the supported limit")
			}
			used += n
		}
	}
	left := q - used
	if left < 0 {
		left = 0
	}
	format := func(n int64) string {
		return strconv.FormatInt(n/1000, 10) + "." + strconv.FormatInt(1000+n%1000, 10)[1:]
	}
	return format(used), format(left), nil
}
