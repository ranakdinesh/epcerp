package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type workflowTaskReader struct {
	c database.Connection
	p WorkflowProviders
}

func NewWorkflowTaskReader(c database.Connection, p WorkflowProviders) contracts.WorkflowTaskReader {
	return &workflowTaskReader{c, p}
}
func (r *workflowTaskReader) Assigned(ctx context.Context, a domain.Principal, q domain.WorkflowTaskQuery) (out domain.WorkflowTaskPage, e error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return out, errors.New("Assigned tasks require the consuming transaction")
	}
	if e = domain.WorkflowID(a.ID); e != nil {
		return out, e
	}
	u, e := r.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active || !a.Active || u.ID != a.ID {
		return out, domain.WorkflowUnauthorized
	}
	live := func(code string) bool { return a.Can(code) && u.Allows(code) }
	execute, resolve := live("projects.purchase_workflow.execute"), live("projects.purchase_workflow.resolve")
	out.Rows = []domain.WorkflowAssignedTask{}
	if !execute && !resolve {
		return out, nil
	}
	if q.Page < 1 || q.Page > 100000 || q.Size < 1 || q.Size > 100 || len(q.Search) > 480 || q.Role != "all" && q.Role != "doer" && q.Role != "resolver" || q.State != "all" && q.State != "not_started" && q.State != "preparing" && q.State != "blocked" {
		return out, domain.WorkflowInvalid("query", "Use bounded task query")
	}
	pattern := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(q.Search) + "%"
	queries := NewStore(r.c)
	out.Total, e = queries.WorkflowCountAssignedTasks(ctx, sqlc.WorkflowCountAssignedTasksParams{ActorID: a.ID, CanExecute: execute, CanResolve: resolve, Role: q.Role, State: q.State, Search: pattern})
	if e != nil {
		return domain.WorkflowTaskPage{}, e
	}
	rows, e := queries.WorkflowPageAssignedTasks(ctx, sqlc.WorkflowPageAssignedTasksParams{ActorID: a.ID, CanExecute: execute, CanResolve: resolve, Role: q.Role, State: q.State, Search: pattern, PageSize: q.Size, RecordOffset: (q.Page - 1) * q.Size})
	if e != nil {
		return domain.WorkflowTaskPage{}, e
	}
	cache := map[string]domain.WorkflowSource{}
	for _, v := range rows {
		var step domain.WorkflowStep
		if e = json.Unmarshal(v.Step, &step); e != nil {
			return domain.WorkflowTaskPage{}, e
		}
		p, id := v.ProjectID.String(), v.ID.String()
		source, ok := cache[id]
		if !ok {
			_, source, e = NewWorkflowSourceReader(r.c, r.p).Inspect(ctx, p, id)
			if e != nil {
				return domain.WorkflowTaskPage{}, e
			}
			cache[id] = source
		}
		relation := "doer"
		if !(execute && step.DoerID != nil && *step.DoerID == a.ID && q.Role != "resolver") {
			relation = "resolver"
		}
		date, due := "", ""
		if step.PlannedDate != nil {
			date = *step.PlannedDate
		}
		if step.ResolutionDue != nil {
			due = *step.ResolutionDue
		}
		out.Rows = append(out.Rows, domain.WorkflowAssignedTask{ID: id + ":" + strconv.Itoa(step.Number), ProjectID: p, ProjectName: source.ProjectName, PackageID: id, Code: v.Code, Version: v.Version, Number: step.Number, Title: step.Title, Phase: step.Phase, Relation: relation, State: step.State, PlannedDate: date, ResolutionDue: due, BlockReason: step.BlockReason, Available: source.Available && source.Eligible && len(source.Blockers) == 0, Blockers: append([]string{}, source.Blockers...)})
	}
	return out, nil
}
