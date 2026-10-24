package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type workflowSourceReader struct {
	c database.Connection
	p WorkflowProviders
}

func NewWorkflowSourceReader(c database.Connection, p WorkflowProviders) contracts.WorkflowSourceReader {
	return &workflowSourceReader{c, p}
}
func (s *workflowSourceReader) Inspect(ctx context.Context, p, id string) (domain.WorkflowPackage, domain.WorkflowSource, error) {
	if !database.IsTransactionConnection(ctx, s.c) {
		return domain.WorkflowPackage{}, domain.WorkflowSource{}, errors.New("Package source requires the consuming transaction")
	}
	if e := domain.WorkflowID(p); e != nil {
		return domain.WorkflowPackage{}, domain.WorkflowSource{}, e
	}
	if e := domain.WorkflowID(id); e != nil {
		return domain.WorkflowPackage{}, domain.WorkflowSource{}, e
	}
	r, e := (&workflowRepository{q: NewStore(s.c)}).Get(ctx, p, id)
	if e != nil {
		return domain.WorkflowPackage{}, domain.WorkflowSource{}, e
	}
	if r == nil || !r.Active {
		return domain.WorkflowPackage{}, domain.WorkflowSource{}, domain.WorkflowMissing
	}
	source, e := (&workflowSourceContext{s.p}).Source(ctx, p, r.IndentID, r.DemandID)
	return *r, source, e
}
func (s *workflowSourceReader) CanWork(ctx context.Context, a domain.Principal, p, id string, n int) (bool, error) {
	record, _, e := s.Source(ctx, p, id)
	if e != nil {
		return false, e
	}
	if n < 3 || n > 20 {
		return false, domain.WorkflowInvalid("step", "Choose preparation step3–20")
	}
	caps, e := (&workflowSourceContext{s.p}).Actions(ctx, a, p)
	if e != nil {
		return false, e
	}
	return domain.WorkflowWorkGuard(a, caps, record.Steps[n-1], "start") == nil, nil
}
func (s *workflowSourceReader) CanConfigure(ctx context.Context, a domain.Principal, p string) (bool, error) {
	if !database.IsTransactionConnection(ctx, s.c) {
		return false, errors.New("Package assignment scope requires the consuming transaction")
	}
	caps, e := (&workflowSourceContext{s.p}).Actions(ctx, a, p)
	return caps.Write, e
}

func (s *workflowSourceReader) Source(ctx context.Context, p, id string) (domain.WorkflowPackage, domain.WorkflowSource, error) {
	r, source, e := s.Inspect(ctx, p, id)
	if e == nil {
		e = source.Guard()
	}
	return r, source, e
}
func (s *workflowSourceReader) CanReadTask(ctx context.Context, a domain.Principal, p, id string, n int) (bool, error) {
	r, _, e := s.Inspect(ctx, p, id)
	if e != nil {
		return false, e
	}
	if n < 3 || n > 20 || len(r.Steps) != 20 {
		return false, domain.WorkflowInvalid("step", "Choose preparation step3–20")
	}
	caps, e := (&workflowSourceContext{s.p}).Actions(ctx, a, p)
	if e != nil {
		return false, e
	}
	step := r.Steps[n-1]
	return caps.Execute && domain.WorkflowAssigned(step, a, "start") || caps.Resolve && domain.WorkflowAssigned(step, a, "resolve"), nil
}

// Cancellation spends no capacity and remains possible when execution is held.
func (s *workflowSourceReader) CanCancel(ctx context.Context, a domain.Principal, p, id string, n int) (bool, error) {
	r, _, e := s.Inspect(ctx, p, id)
	if e != nil {
		return false, e
	}
	if n < 3 || n > 20 || len(r.Steps) != 20 {
		return false, domain.WorkflowInvalid("step", "Choose preparation step3–20")
	}
	caps, e := (&workflowSourceContext{s.p}).Actions(ctx, a, p)
	if e != nil {
		return false, e
	}
	return caps.Execute && (caps.Recovery || domain.WorkflowAssigned(r.Steps[n-1], a, "start")), nil
}
