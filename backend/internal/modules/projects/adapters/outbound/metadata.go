package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
)

func (r *repository) UpdateMetadata(ctx context.Context, p domain.Project, input domain.Correction) (domain.Project, error) {
	before, e := r.Get(ctx, p.ID, false)
	if e != nil {
		return p, e
	}
	p.Name = input.Name
	p.NameKey = domain.NameKey(input.Name)
	p.ClientID = input.ClientID
	p.Capture = input.Capture
	v, e := r.q.UpdateMetadata(ctx, db.UpdateMetadataParams{ID: uuid(p.ID), Name: p.Name, NormalizedName: p.NameKey, ClientID: uuid(p.ClientID), ActorID: uuid(p.UpdatedBy), ShortName: p.ShortName, ProjectType: p.ProjectType, SiteName: p.SiteName, SiteState: p.SiteState, SiteCity: p.SiteCity, ScopeSummary: p.ScopeSummary, OperationsManagerID: p.OperationsManagerID, ContractValue: p.ContractValue, Currency: p.Currency, LoiNumber: p.LOINumber, PoNumber: p.PONumber, ScheduleStartBasis: p.ScheduleStartBasis, PlannedStartDate: p.PlannedStartDate, PlannedCompletionDate: p.PlannedCompletionDate, LoiDate: p.LOIDate, PoDate: p.PODate, ActualStartDate: p.ActualStartDate, SiteHandoverDate: p.SiteHandoverDate, PlannedDurationDays: p.PlannedDurationDays, SafetyBufferPercent: p.SafetyBufferPercent, SafetyBufferDays: p.SafetyBufferDays, Remarks: p.Remarks, ActualEndDate: p.ActualEndDate, PurchaseEnabled: p.PurchaseEnabled, ScheduleCalculation: calculationJSON(p.Calculation), ExpectedVersion: input.ExpectedVersion})
	if errors.Is(e, pgx.ErrNoRows) {
		return p, domain.ErrStale
	}
	if e != nil {
		if errors.Is(conflict(e), domain.ErrConflict) {
			return p, &domain.Invalid{Field: "name", Message: "A project with this name already exists"}
		}
		return p, e
	}
	saved := project(v)
	id := SystemValues{}.ID()
	beforeJSON, e := json.Marshal(before)
	if e != nil {
		return p, e
	}
	afterJSON, e := json.Marshal(saved)
	if e != nil {
		return p, e
	}
	if e = r.q.InsertMetadataHistory(ctx, db.InsertMetadataHistoryParams{ID: uuid(id), ProjectID: uuid(saved.ID), Version: saved.Version, Reason: input.Reason, BeforeSnapshot: beforeJSON, AfterSnapshot: afterJSON, Actor: uuid(saved.UpdatedBy)}); e != nil {
		return p, e
	}
	e = r.events.Record(ctx, events.Event{ID: id, Module: "projects", EntityType: "project", EntityID: saved.ID, EntityVersion: int64(saved.Version), Action: "metadata_updated", ActorID: saved.UpdatedBy, ActorKind: "user", CorrelationID: id, ScopeKind: "project", ScopeID: saved.ID, Visibility: "restricted", OccurredAt: saved.UpdatedAt}, nil)
	return saved, e
}
func (r *repository) MetadataHistory(ctx context.Context, id string) ([]domain.MetadataChange, error) {
	rows, e := r.q.MetadataHistory(ctx, uuid(id))
	if e != nil {
		return nil, e
	}
	out := []domain.MetadataChange{}
	for _, v := range rows {
		var before, after domain.Project
		if e = json.Unmarshal(v.BeforeSnapshot, &before); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(v.AfterSnapshot, &after); e != nil {
			return nil, e
		}
		out = append(out, domain.MetadataChange{ID: key(v.ID), ProjectID: id, Version: v.ProjectVersion, Reason: v.Reason, ActorID: key(v.CreatedByID), Before: before, After: after, OccurredAt: v.CreatedAt.Time.UTC()})
	}
	return out, nil
}
