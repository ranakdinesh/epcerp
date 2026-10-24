package domain

import (
	"fmt"
	"math/big"
)

// LOITotals reconciles exact approved components, including allocated freight and
// line-total pricing. It never recomputes rounded GST from a displayed unit rate.
func LOITotals(d LOIContent) (map[string]string, error) {
	sums := map[string]*big.Int{}
	for _, key := range []string{"base", "discount", "tax", "charges", "total"} {
		sums[key] = new(big.Int)
	}
	if len(d.Items) == 0 {
		return nil, WorkflowInvalid("items", "LOI requires approved items")
	}
	for _, item := range d.Items {
		values := map[string]string{"base": item.Price.Base, "discount": item.Price.Discount, "tax": item.Price.Tax, "charges": item.Price.Charges, "total": item.Total}
		for key, value := range values {
			n, e := quotationApprovalsMoney(value)
			if e != nil {
				return nil, e
			}
			sums[key].Add(sums[key], n)
		}
		b, e := quotationApprovalsMoney(item.Price.Base)
		if e != nil {
			return nil, e
		}
		for _, entry := range []struct {
			v        string
			subtract bool
		}{{item.Price.Discount, true}, {item.Price.Tax, false}, {item.Price.Charges, false}} {
			n, e := quotationApprovalsMoney(entry.v)
			if e != nil {
				return nil, e
			}
			if entry.subtract {
				b.Sub(b, n)
			} else {
				b.Add(b, n)
			}
		}
		t, e := quotationApprovalsMoney(item.Total)
		if e != nil || b.Cmp(t) != 0 {
			return nil, WorkflowInvalid("price", "Approved LOI item components do not reconcile")
		}
	}
	t, e := quotationApprovalsMoney(d.Total)
	if e != nil || t.Cmp(sums["total"]) != 0 {
		return nil, WorkflowInvalid("total", "Approved LOI total does not reconcile")
	}
	out := map[string]string{}
	for key, n := range sums {
		q, r := new(big.Int), new(big.Int)
		q.QuoRem(n, big.NewInt(100), r)
		out[key] = fmt.Sprintf("%s.%02d", q.String(), r.Int64())
	}
	return out, nil
}
