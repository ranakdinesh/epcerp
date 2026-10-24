package repository

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func registerIndentComplete(ctx context.Context, c database.Connection, b domain.BORecord) (bool, error) {
	if b.State != "completed" {
		return false, nil
	}
	groups := NewIndentGroupsReader(c)
	g, err := groups.Get(ctx, b.ProjectID, b.ID)
	if err != nil || g == nil || !g.Active {
		return false, err
	}
	if g.ProjectID != b.ProjectID || g.BOID != b.ID {
		return false, domain.ProgressInvalid("source", "Indent grouping scope changed")
	}
	docs, err := NewIndentDocumentsReader(c, groups).List(ctx, b.ProjectID, b.ID)
	if err != nil {
		return false, err
	}
	s := domain.ProgressGroupedSource{BOState: b.State, GroupState: g.State}
	for _, l := range b.Lines {
		i := l.Snapshot
		s.Lines = append(s.Lines, domain.ProgressGroupedLine{ID: l.ID, Quantity: progressValue(l.Quantity), RequiredDate: progressValue(l.RequiredDate), Snapshot: progressItemSnapshot(i.ID, i.SKU, i.Description, i.EngineeringKey, i.UOM, i.Manufacturer)})
	}
	for _, x := range g.Groups {
		s.Groups = append(s.Groups, domain.ProgressGroupedGroup{ID: x.ID, Revision: x.ScopeRevision, LineIDs: x.LineIDs, VendorIDs: x.VendorIDs})
	}
	for _, d := range docs {
		if d.ProjectID != b.ProjectID || d.BOID != b.ID {
			return false, domain.ProgressInvalid("source", "Indent document scope changed")
		}
		x := domain.ProgressGroupedDocument{ID: d.ID, GroupID: d.GroupID, VendorID: d.VendorID, Version: d.Version, Revision: d.Data.ScopeRevision, Active: d.Active, SentDate: progressValue(d.Data.SentDate)}
		for _, l := range d.Data.Lines {
			i := l.Snapshot
			x.Lines = append(x.Lines, domain.ProgressGroupedLine{ID: l.ID, Quantity: progressValue(l.Quantity), RequiredDate: progressValue(l.RequiredDate), Snapshot: progressItemSnapshot(i.ID, i.SKU, i.Description, i.EngineeringKey, i.UOM, i.Manufacturer)})
		}
		s.Documents = append(s.Documents, x)
	}
	facts, err := domain.ProgressGroupedFacts(s)
	if err != nil {
		return false, err
	}
	for _, f := range facts {
		if f.Number == 2 {
			return f.State == "completed", nil
		}
	}
	return false, nil
}
