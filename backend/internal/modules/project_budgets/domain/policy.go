package domain

import (
	"strings"
	"unicode/utf8"
)

func kind(s string) bool {
	switch s {
	case "purchase", "labour", "subcontract", "equipment", "engineering", "logistics", "overheads", "testing", "contingency", "other":
		return true
	}
	return false
}
func Validate(d Data, scope Scope, previous *Data) (Data, error) {
	if d.PricingMode != "" && d.PricingMode != "unit_rates" {
		return d, Invalid("pricing_mode", "Choose the supported unit-rate budget mode")
	}
	var e error
	d.OwnerID, e = ID(d.OwnerID)
	if e != nil {
		return d, e
	}
	total, e := Money(d.Total)
	if e != nil {
		return d, e
	}
	d.Total = Format(total)
	if d.Currency != scope.Project.Currency {
		return d, Invalid("currency", "Use the project's currency")
	}
	owners := map[string]bool{}
	for _, o := range scope.Owners {
		owners[o.ID] = true
	}
	if !owners[d.OwnerID] {
		return d, Invalid("owner_id", "Choose an active project budget owner")
	}
	if utf8.RuneCountInString(d.Notes) > 2000 || len(d.Heads) > 100 || len(d.Allocations) > 1000 {
		return d, Invalid("data", "Budget notes or record count exceeds permitted limits")
	}
	d = CopyData(d)
	heads := map[string]Head{}
	allHeads := map[string]bool{}
	allAllocations := map[string]bool{}
	names := map[string]bool{}
	for i, h := range d.Heads {
		h.ID, e = ID(h.ID)
		if e != nil {
			return d, e
		}
		if allHeads[h.ID] {
			return d, Invalid("heads", "Budget record identifiers must be distinct")
		}
		allHeads[h.ID] = true
		h.OwnerID, e = ID(h.OwnerID)
		if e != nil {
			return d, e
		}
		if h.ParentID != nil {
			v, e := ID(*h.ParentID)
			if e != nil {
				return d, e
			}
			h.ParentID = &v
		}
		h.Name = strings.TrimSpace(h.Name)
		if h.Name == "" || utf8.RuneCountInString(h.Name) > 128 || !kind(h.Kind) {
			return d, Invalid("heads", "Choose a valid cost category and a head name of at most 128 characters")
		}
		n, e := Money(h.Amount)
		if e != nil {
			return d, e
		}
		h.Amount = Format(n)
		if h.Active {
			parent := ""
			if h.ParentID != nil {
				parent = *h.ParentID
			}
			key := parent + ":" + strings.ToLower(h.Name)
			if names[key] {
				return d, Invalid("heads", "Use distinct head names within each parent")
			}
			names[key] = true
			if !owners[h.OwnerID] {
				return d, Invalid("owner_id", "Choose an active head owner")
			}
			heads[h.ID] = h
		}
		d.Heads[i] = h
	}
	sources := map[string]Item{}
	for _, s := range scope.Items {
		s.Snapshot = ItemSnapshot{SKU: s.SKU, Description: s.Description, UOM: s.UOM, Manufacturer: s.Manufacturer}
		sources[s.Key()] = s
	}
	assigned := map[string]bool{}
	for i, a := range d.Allocations {
		for _, field := range []*string{&a.ID, &a.BOID, &a.LineID, &a.HeadID, &a.OwnerID} {
			if *field, e = ID(*field); e != nil {
				return d, e
			}
		}
		if allAllocations[a.ID] {
			return d, Invalid("allocations", "Budget record identifiers must be distinct")
		}
		allAllocations[a.ID] = true
		n, e := Money(a.Amount)
		if e != nil {
			return d, e
		}
		a.Amount = Format(n)
		if a.UnitPrice != nil {
			n, e := Money(*a.UnitPrice)
			if e != nil {
				return d, e
			}
			v := Format(n)
			a.UnitPrice = &v
		}
		if a.PricingQuantity != nil {
			if _, e := Qty(*a.PricingQuantity); e != nil {
				return d, e
			}
		}
		if a.Active {
			source, found := sources[a.Key()]
			if assigned[a.Key()] || !found {
				return d, Invalid("allocations", "Choose distinct items from this project's BO lists")
			}
			assigned[a.Key()] = true
			head, found := heads[a.HeadID]
			if !found || head.Kind != "purchase" || !owners[a.OwnerID] {
				return d, Invalid("allocations", "Item allocations need an active purchase head and responsible owner")
			}
			if a.UnitPrice != nil {
				q, e := Qty(source.Quantity)
				if e != nil {
					return d, e
				}
				if a.PricingQuantity == nil {
					return d, Invalid("pricing_quantity", "Purchase quantities changed; reopen Item purchase and save prices")
				}
				old, e := Qty(*a.PricingQuantity)
				if e != nil || old.Cmp(q) != 0 {
					return d, Invalid("pricing_quantity", "Purchase quantities changed; reopen Item purchase and save prices")
				}
				expected, e := LineBudget(source.Quantity, *a.UnitPrice)
				if e != nil {
					return d, e
				}
				if expected != a.Amount {
					return d, Invalid("amount", "Item budget must equal current quantity multiplied by unit price")
				}
			}
			snapshot := source
			a.Source = &snapshot
		}
		d.Allocations[i] = a
	}
	if previous != nil {
		oldAllocations := map[string]Allocation{}
		for _, old := range previous.Allocations {
			oldAllocations[old.ID] = old
		}
		for n, a := range d.Allocations {
			if old, exists := oldAllocations[a.ID]; exists {
				if old.BOID != a.BOID || old.LineID != a.LineID {
					return d, Invalid("allocations", "Retain each allocation's original BO and item identity")
				}
				if !a.Active {
					d.Allocations[n].Source = old.Source
				}
			}
		}
		for _, h := range previous.Heads {
			if !allHeads[h.ID] {
				return d, Invalid("heads", "Deactivate existing budget records instead of deleting them")
			}
		}
		for _, a := range previous.Allocations {
			if !allAllocations[a.ID] {
				return d, Invalid("allocations", "Deactivate existing budget records instead of deleting them")
			}
		}
	}
	var roots int64
	for _, h := range heads {
		path := map[string]bool{h.ID: true}
		parent := h.ParentID
		for parent != nil {
			p, found := heads[*parent]
			if !found {
				return d, Invalid("parent_id", "Subheads require an active parent")
			}
			if path[p.ID] || len(path) >= 6 {
				return d, Invalid("parent_id", "Budget hierarchy must be acyclic and at most six levels")
			}
			path[p.ID] = true
			if p.Kind != h.Kind {
				return d, Invalid("kind", "Subheads must use their parent's cost category")
			}
			parent = p.ParentID
		}
		amount, _ := Money(h.Amount)
		if h.ParentID == nil {
			roots, e = addBound(roots, amount)
			if e != nil {
				return d, e
			}
		}
		var committed int64
		for _, child := range heads {
			if child.ParentID != nil && *child.ParentID == h.ID {
				n, _ := Money(child.Amount)
				committed, e = addBound(committed, n)
				if e != nil {
					return d, e
				}
			}
		}
		for _, a := range d.Allocations {
			if a.Active && a.HeadID == h.ID {
				n, _ := Money(a.Amount)
				committed, e = addBound(committed, n)
				if e != nil {
					return d, e
				}
			}
		}
		if committed > amount {
			return d, Invalid("amount", "Subheads and item allocations exceed "+h.Name)
		}
	}
	if roots > total {
		return d, Invalid("total", "Cost heads exceed the project budget")
	}
	return d, nil
}
func BuildReport(d Data) (Report, error) {
	out := Report{Heads: []HeadReport{}}
	total, e := Money(d.Total)
	if e != nil {
		return out, e
	}
	var roots, purchase, items int64
	for _, a := range d.Allocations {
		if a.Active {
			v, e := Money(a.Amount)
			if e != nil {
				return out, e
			}
			items, e = addBound(items, v)
			if e != nil {
				return out, e
			}
		}
	}
	for _, h := range d.Heads {
		if !h.Active {
			continue
		}
		v, e := Money(h.Amount)
		if e != nil {
			return out, e
		}
		if h.ParentID == nil {
			roots, e = addBound(roots, v)
			if e != nil {
				return out, e
			}
			if h.Kind == "purchase" {
				purchase, e = addBound(purchase, v)
				if e != nil {
					return out, e
				}
			}
		}
		var assigned int64
		for _, child := range d.Heads {
			if child.Active && child.ParentID != nil && *child.ParentID == h.ID {
				v, e := Money(child.Amount)
				if e != nil {
					return out, e
				}
				assigned, e = addBound(assigned, v)
				if e != nil {
					return out, e
				}
			}
		}
		for _, a := range d.Allocations {
			if a.Active && a.HeadID == h.ID {
				v, e := Money(a.Amount)
				if e != nil {
					return out, e
				}
				assigned, e = addBound(assigned, v)
				if e != nil {
					return out, e
				}
			}
		}
		if assigned > v {
			return out, Invalid("amount", "Subheads and item allocations exceed "+h.Name)
		}
		out.Heads = append(out.Heads, HeadReport{Head: h, Unallocated: Format(v - assigned)})
	}
	if roots > total {
		return out, Invalid("total", "Cost heads exceed the project budget")
	}
	out.Total = Format(total)
	out.AllocatedHeads = Format(roots)
	out.PurchaseTotal = Format(purchase)
	out.Unallocated = Format(total - roots)
	out.ItemAllocations = Format(items)
	return out, nil
}

// PriceItems needs fresh IDs supplied by the application Values port. The domain
// does not generate randomness or accept browser-calculated totals.
func PriceItems(previous *Data, prices []Price, sources []Item, owner, total, currency, rootID string, newIDs map[string]string) (Data, error) {
	mapped := map[string]Item{}
	for _, s := range sources {
		mapped[s.Key()] = s
	}
	seen := map[string]bool{}
	for _, p := range prices {
		source, found := mapped[p.Key()]
		if seen[p.Key()] || !found {
			return Data{}, Invalid("prices", "Purchase items changed; reload the complete project item list")
		}
		seen[p.Key()] = true
		q, e := Qty(p.Quantity)
		if e != nil {
			return Data{}, e
		}
		actual, e := Qty(source.Quantity)
		if e != nil {
			return Data{}, e
		}
		if q.Cmp(actual) != 0 || p.BOVersion != source.BOVersion {
			return Data{}, Stale
		}
	}
	if len(seen) != len(mapped) {
		return Data{}, Invalid("prices", "Purchase items changed; reload the complete project item list")
	}
	d := Data{OwnerID: owner, Heads: []Head{}, Allocations: []Allocation{}}
	if previous != nil {
		d = CopyData(*previous)
	}
	n, e := Money(total)
	if e != nil {
		return d, e
	}
	d.Total = Format(n)
	d.Currency = currency
	d.PricingMode = "unit_rates"
	purchase := map[string]int{}
	root := ""
	for i, h := range d.Heads {
		if h.Active && h.Kind == "purchase" {
			purchase[h.ID] = i
			if root == "" && h.ParentID == nil {
				root = h.ID
			}
		}
	}
	if root == "" {
		root = rootID
		purchase[root] = len(d.Heads)
		d.Heads = append(d.Heads, Head{ID: root, Name: "Item purchase", Kind: "purchase", Amount: "0.00", OwnerID: owner, Active: true})
	}
	current := map[string]int{}
	for i, a := range d.Allocations {
		if a.Active {
			current[a.Key()] = i
		}
		if _, found := mapped[a.Key()]; !found {
			d.Allocations[i].Active = false
		}
	}
	for _, p := range prices {
		index, exists := current[p.Key()]
		rate := p.UnitPrice
		if !p.Changed {
			rate = nil
			if exists {
				rate = d.Allocations[index].UnitPrice
			}
		}
		if rate == nil {
			if p.Changed && exists {
				d.Allocations[index].Active = false
			}
			continue
		}
		r, e := Money(*rate)
		if e != nil {
			return d, e
		}
		rateText := Format(r)
		amount, e := LineBudget(mapped[p.Key()].Quantity, rateText)
		if e != nil {
			return d, e
		}
		if !exists {
			id := newIDs[p.Key()]
			if id == "" {
				return d, Invalid("id", "Allocation identity is missing")
			}
			index = len(d.Allocations)
			d.Allocations = append(d.Allocations, Allocation{ID: id, BOID: p.BOID, LineID: p.LineID, HeadID: root, OwnerID: owner})
		}
		a := &d.Allocations[index]
		a.UnitPrice = &rateText
		qty := mapped[p.Key()].Quantity
		a.PricingQuantity = &qty
		a.Amount = amount
		a.Active = true
		if _, found := purchase[a.HeadID]; !found {
			a.HeadID = root
		}
	}
	var rollup func(string, map[string]bool) (int64, error)
	rollup = func(id string, path map[string]bool) (int64, error) {
		if path[id] {
			return 0, Invalid("heads", "Purchase budget hierarchy must be acyclic")
		}
		path[id] = true
		defer delete(path, id)
		var total int64
		for _, a := range d.Allocations {
			if a.Active && a.HeadID == id {
				v, e := Money(a.Amount)
				if e != nil {
					return 0, e
				}
				total, e = addBound(total, v)
				if e != nil {
					return 0, e
				}
			}
		}
		for child, index := range purchase {
			h := d.Heads[index]
			if h.ParentID != nil && *h.ParentID == id {
				v, e := rollup(child, path)
				if e != nil {
					return 0, e
				}
				total, e = addBound(total, v)
				if e != nil {
					return 0, e
				}
			}
		}
		d.Heads[purchase[id]].Amount = Format(total)
		return total, nil
	}
	for id, index := range purchase {
		if d.Heads[index].ParentID == nil {
			if _, e = rollup(id, map[string]bool{}); e != nil {
				return d, e
			}
		}
	}
	return d, nil
}
func Attribute(d Data, previous *Data, actor string, now Timestamp) Data {
	d = CopyData(d)
	heads := map[string]Audit{}
	allocations := map[string]Audit{}
	if previous != nil {
		for _, h := range previous.Heads {
			heads[h.ID] = h.Audit
		}
		for _, a := range previous.Allocations {
			allocations[a.ID] = a.Audit
		}
	}
	audit := func(old Audit) Audit {
		if old.CreatedAt.IsZero() {
			old.CreatedAt = now
			old.CreatedBy = &actor
		}
		old.UpdatedAt = now
		old.UpdatedBy = &actor
		return old
	}
	for i, h := range d.Heads {
		d.Heads[i].Audit = audit(heads[h.ID])
	}
	for i, a := range d.Allocations {
		d.Allocations[i].Audit = audit(allocations[a.ID])
	}
	return d
}
