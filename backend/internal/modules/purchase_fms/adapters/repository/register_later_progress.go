package repository

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type LaterProgressProviders struct {
	Workflow              WorkflowProviders
	Sourcing              ports.SourcingContext
	Technical             ports.TechnicalContext
	Acceptance            ports.AcceptanceContext
	Advances              ports.AdvancesContext
	Inspections           ports.InspectionsContext
	InspectionReports     ports.InspectionReportsContext
	Quality               ports.QualityContext
	ClientClearance       ports.ClientClearanceContext
	Dispatch              ports.DispatchContext
	SiteReceipts          ports.SiteReceiptsContext
	GRNs                  ports.GRNsContext
	EngineerConfirmations ports.EngineerConfirmationsContext
	Accounts              func(context.Context, string, string) (domain.ProgressEvidence, error)
}
type LaterProgressReader struct {
	c database.Connection
	p LaterProgressProviders
}

func NewLaterProgressReader(c database.Connection, p LaterProgressProviders) *LaterProgressReader {
	return &LaterProgressReader{c, p}
}
func (r *LaterProgressReader) Read(ctx context.Context, project string, b domain.BORecord) ([]int, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Later progress requires the owning transaction")
	}
	demands := []string{}
	for _, l := range b.Lines {
		if l.DemandID != nil {
			demands = append(demands, *l.DemandID)
		}
	}
	if len(demands) != len(b.Lines) {
		return []int{}, nil
	}
	packages := []domain.ProgressPackage{}
	for page := int32(1); ; page++ {
		part, e := NewWorkflowProgressReader(r.c, r.p.Workflow).Page(ctx, project, demands, page, 100)
		if e != nil {
			return nil, e
		}
		if part.Total > 1000 {
			return nil, domain.ProgressInvalid("packages", "Project reporting package limit exceeded")
		}
		for _, row := range part.Rows {
			v := row.Package
			facts, e := r.packageFacts(ctx, project, v.ID)
			if e != nil {
				return nil, e
			}
			p := domain.ProgressPackage{ID: v.ID, DemandID: v.DemandID, Quantity: v.Source.Quantity, Evidence: facts}
			if !row.Source.Eligible || row.Source.DemandID != v.DemandID || row.Source.Quantity != v.Source.Quantity || row.Source.Description != v.Source.Description || row.Source.UOM != v.Source.UOM {
				p.SourceBlockers = []string{"Work package differs from current demand"}
			}
			packages = append(packages, p)
		}
		if int64(page*100) >= part.Total {
			break
		}
	}
	lines := []domain.ProgressLine{}
	for _, l := range b.Lines {
		if l.DemandID == nil || l.Quantity == nil {
			return []int{}, nil
		}
		lines = append(lines, domain.ProgressLine{ID: l.ID, DemandID: *l.DemandID, Quantity: *l.Quantity})
	}
	return domain.LaterRegisterCompletion(lines, packages), nil
}
func (r *LaterProgressReader) packageFacts(ctx context.Context, p, pkg string) ([]domain.ProgressEvidence, error) {
	out := []domain.ProgressEvidence{}
	q := NewStore(r.c)
	sourcing, e := (&sourcingRepository{q: q}).Get(ctx, p, pkg)
	if e != nil {
		return nil, e
	}
	loiCurrent := false
	if sourcing != nil && sourcing.Active {
		source, e := r.p.Sourcing.Source(ctx, p, pkg)
		if e != nil {
			return nil, e
		}
		fact := domain.SourcingProgressFacts(sourcing, p, pkg, source, true)
		for _, step := range fact.Steps {
			if step.Number == 5 {
				out = append(out, domain.ProgressEvidence{Number: 5, State: step.State, Details: step.Details})
				loiCurrent = step.State == "completed"
			}
		}
	}
	technical, e := NewTechnicalProgressReader(r.c, r.p.Technical).Progress(ctx, p, pkg, loiCurrent)
	if e != nil {
		return nil, e
	}
	technicalCurrent := false
	for _, s := range technical.Steps {
		if s.Number == 8 && s.State == "completed" {
			technicalCurrent = true
		}
		out = append(out, domain.ProgressEvidence{Number: s.Number, State: s.State, Details: s.Details})
	}
	order, e := NewOrdersProgressReader(r.c).Progress(ctx, p, pkg)
	if e != nil {
		return nil, e
	}
	out = append(out, domain.ProgressEvidence{Number: 9, State: order.State, Details: order.Details})
	prepared, err := (&ordersRepository{q: q}).Get(ctx, p, pkg)
	if err != nil {
		return nil, err
	}
	clientState := "not_started"
	if prepared != nil {
		clientState = "in_progress"
		if prepared.State == "released" || domain.OrdersClientCleared(*prepared) && loiCurrent && technicalCurrent {
			clientState = "completed"
		}
	}
	out = append(out, domain.ProgressEvidence{Number: 21, State: clientState, Details: "Client stamp/sign on the exact draft PO"})
	acceptance, e := NewAcceptanceProgressReader(r.c, r.p.Acceptance).Progress(ctx, p, pkg)
	if e != nil {
		return nil, e
	}
	out = append(out, domain.ProgressEvidence{Number: 10, State: acceptance.State, Details: acceptance.Details})
	{
		record, e := (&advancesRepository{q: q}).Get(ctx, p, pkg)
		if e != nil {
			return nil, e
		}
		if record != nil && record.Active {
			source, e := r.p.Advances.Source(ctx, p, pkg)
			if e != nil {
				return nil, e
			}
			fact := domain.AdvancesProgressFact(record, source.AdvancesSourceFacts, true, (Values{}).Now())
			out = append(out, domain.ProgressEvidence{Number: 11, State: fact.State, Details: fact.Details})
		}
	}
	{
		record, e := (&inspectionsRepository{q: q}).Get(ctx, p, pkg)
		if e != nil {
			return nil, e
		}
		if record != nil && record.Active {
			source, e := r.p.Inspections.Source(ctx, p, pkg)
			if e != nil {
				return nil, e
			}
			fact := domain.InspectionsProgressFact(record, source.InspectionsSourceFacts, true)
			out = append(out, domain.ProgressEvidence{Number: 12, State: fact.State, Details: fact.Details})
		}
	}
	{
		record, e := (&inspectionReportsRepository{q: q}).Get(ctx, p, pkg)
		if e != nil {
			return nil, e
		}
		if record != nil && record.Active {
			source, e := r.p.InspectionReports.Source(ctx, p, pkg)
			if e != nil {
				return nil, e
			}
			fact := domain.InspectionReportsProgressFact(record, source.InspectionReportsSourceFacts, true)
			out = append(out, domain.ProgressEvidence{Number: 13, State: fact.State, Details: fact.Details})
		}
	}
	{
		record, e := (&qualityRepository{q: q}).Get(ctx, p, pkg)
		if e != nil {
			return nil, e
		}
		if record != nil && record.Active {
			source, e := r.p.Quality.Source(ctx, p, pkg)
			if e != nil {
				return nil, e
			}
			fact := domain.QualityProgressFact(record, source.QualitySourceFacts, true, domain.QualityToday((Values{}).Now()))
			out = append(out, domain.ProgressEvidence{Number: 14, State: fact.State, Details: fact.Details})
		}
	}
	{
		record, e := (&clientClearanceRepository{q: q}).Get(ctx, p, pkg)
		if e != nil {
			return nil, e
		}
		if record != nil && record.Active {
			source, e := r.p.ClientClearance.Source(ctx, p, pkg)
			if e != nil {
				return nil, e
			}
			fact := domain.ClientClearanceProgressFact(record, source.ClientClearanceSourceFacts, true)
			out = append(out, domain.ProgressEvidence{Number: 15, State: fact.State, Details: fact.Details})
		}
	}
	{
		record, e := (&dispatchRepository{q: q}).Get(ctx, p, pkg)
		if e != nil {
			return nil, e
		}
		if record != nil && record.Active {
			source, e := r.p.Dispatch.Source(ctx, p, pkg)
			if e != nil {
				return nil, e
			}
			fact := domain.DispatchProgressFact(record, source.DispatchSourceFacts, true)
			out = append(out, domain.ProgressEvidence{Number: 16, State: fact.State, Details: fact.Details})
		}
	}
	{
		record, e := (&siteReceiptsRepository{q: q}).Get(ctx, p, pkg)
		if e != nil {
			return nil, e
		}
		if record != nil && record.Active {
			source, e := r.p.SiteReceipts.Source(ctx, p, pkg)
			if e != nil {
				return nil, e
			}
			fact := domain.SiteReceiptsProgressFact(record, source.SiteReceiptsSourceFacts, true)
			out = append(out, domain.ProgressEvidence{Number: 17, State: fact.State, Details: fact.Details})
		}
	}
	{
		record, e := (&gRNsRepository{q: q}).Get(ctx, p, pkg)
		if e != nil {
			return nil, e
		}
		if record != nil && record.Active {
			source, e := r.p.GRNs.Source(ctx, p, pkg)
			if e != nil {
				return nil, e
			}
			fact := domain.GRNsProgressFact(record, source.GRNsSourceFacts, true)
			out = append(out, domain.ProgressEvidence{Number: 18, State: fact.State, Details: fact.Details})
		}
	}
	{
		record, e := (&engineerConfirmationsRepository{q: q}).Get(ctx, p, pkg)
		if e != nil {
			return nil, e
		}
		if record != nil && record.Active {
			source, e := r.p.EngineerConfirmations.Source(ctx, p, pkg)
			if e != nil {
				return nil, e
			}
			fact := domain.EngineerConfirmationsProgressFact(record, source.EngineerConfirmationsSourceFacts, true)
			out = append(out, domain.ProgressEvidence{Number: 19, State: fact.State, Details: fact.Details})
		}
	}
	if r.p.Accounts != nil {
		fact, e := r.p.Accounts(ctx, p, pkg)
		if e != nil {
			return nil, e
		}
		out = append(out, fact)
	}
	return out, nil
}
