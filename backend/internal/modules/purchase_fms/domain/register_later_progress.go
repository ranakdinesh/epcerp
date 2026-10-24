package domain

import (
	"math/big"
	"strings"
)

// Full demand coverage is required before a later project stage earns credit.
func LaterRegisterCompletion(lines []ProgressLine, packages []ProgressPackage) []int {
	if len(lines) == 0 || len(packages) == 0 {
		return []int{}
	}
	expected := map[string]*big.Int{}
	actual := map[string]*big.Int{}
	for _, l := range lines {
		n, e := laterRegisterQuantity(l.Quantity)
		if e != nil || l.DemandID == "" || expected[l.DemandID] != nil {
			return []int{}
		}
		expected[l.DemandID] = n
	}
	seen := map[string]bool{}
	for _, p := range packages {
		if seen[p.ID] || len(p.SourceBlockers) > 0 || expected[p.DemandID] == nil {
			return []int{}
		}
		seen[p.ID] = true
		n, e := laterRegisterQuantity(p.Quantity)
		if e != nil {
			return []int{}
		}
		if actual[p.DemandID] == nil {
			actual[p.DemandID] = new(big.Int)
		}
		actual[p.DemandID].Add(actual[p.DemandID], n)
	}
	for id, n := range expected {
		if actual[id] == nil || actual[id].Cmp(n) != 0 {
			return []int{}
		}
	}
	out := []int{}
	for stage := 5; stage <= 21; stage++ {
		complete := true
		for _, p := range packages {
			done := false
			for _, f := range p.Evidence {
				if f.Number == stage {
					done = progressDone(f.State)
				}
			}
			complete = complete && done
		}
		if complete {
			out = append(out, stage)
		}
	}
	return out
}

func laterRegisterQuantity(s string) (*big.Int, error) {
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return progressQuantity(s)
}
