package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type WorkspaceUnitOfWork struct {
	later func(database.Connection) *LaterProgressReader
	a     func(database.Connection) QuotationApprovalsProviders
	m     *database.Manager
	p     func(database.Connection) projects.ContextReader
	u     func(database.Connection) auth.AssignmentDirectory
	e     func(database.Connection) events.Writer
}

func NewWorkspaceUnitOfWork(b database.Beginner, p func(database.Connection) projects.ContextReader, u func(database.Connection) auth.AssignmentDirectory, e func(database.Connection) events.Writer) *WorkspaceUnitOfWork {
	return &WorkspaceUnitOfWork{m: database.NewManager(b), p: p, u: u, e: e}
}
func (u *WorkspaceUnitOfWork) Within(ctx context.Context, f func(context.Context, ports.WorkspaceRepository, ports.WorkspaceProjects, ports.WorkspaceUsers) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return f(ctx, &workspaceRepository{q: NewStore(c), e: u.e(c), c: c, later: func() *LaterProgressReader {
			if u.later == nil {
				return nil
			}
			return u.later(c)
		}(), a: func() *QuotationApprovalsProviders {
			if u.a == nil {
				return nil
			}
			v := u.a(c)
			return &v
		}()}, &workspaceProjectBridge{u.p(c)}, &workspaceUserBridge{u.u(c)})
	})
}

type workspaceProjectBridge struct{ r projects.ContextReader }

func workspaceProject(p projects.Context) domain.WorkspaceProject {
	return domain.WorkspaceProject{ID: p.ID, Name: p.Name, Number: p.Number, ManagerID: p.ManagerID, Active: p.Active, Lifecycle: p.Lifecycle}
}
func (b *workspaceProjectBridge) Get(ctx context.Context, id string, lock bool) (*domain.WorkspaceProject, error) {
	p, e := b.r.Get(ctx, id, lock)
	if e != nil || p == nil {
		return nil, e
	}
	v := workspaceProject(*p)
	return &v, nil
}
func (b *workspaceProjectBridge) Page(ctx context.Context, f domain.WorkspaceFilter, actor string, ids []string, portfolio bool) ([]domain.WorkspaceProject, int64, error) {
	p, e := b.r.Page(ctx, projects.Filter{Page: f.Page, PageSize: f.Size, Search: f.Search, Sort: f.Sort, Direction: f.Direction}, actor, ids, portfolio)
	out := []domain.WorkspaceProject{}
	for _, v := range p.Items {
		out = append(out, workspaceProject(v))
	}
	return out, p.Total, e
}

type workspaceUserBridge struct{ r auth.AssignmentDirectory }

func workspaceCandidate(v auth.AssignmentCandidate) domain.WorkspaceCandidate {
	return domain.WorkspaceCandidate{ID: v.ID, Name: v.Name, Active: v.Active, Superuser: v.Superuser, Permissions: v.Permissions}
}
func (b *workspaceUserBridge) Get(ctx context.Context, id string) (*domain.WorkspaceCandidate, error) {
	v, e := b.r.Get(ctx, id)
	if e != nil || v == nil {
		return nil, e
	}
	c := workspaceCandidate(*v)
	return &c, nil
}
func (b *workspaceUserBridge) List(ctx context.Context) ([]domain.WorkspaceCandidate, error) {
	vs, e := b.r.List(ctx)
	out := []domain.WorkspaceCandidate{}
	for _, v := range vs {
		out = append(out, workspaceCandidate(v))
	}
	return out, e
}

type workspaceRepository struct {
	later *LaterProgressReader
	a     *QuotationApprovalsProviders
	q     *sqlc.Queries
	e     events.Writer
	c     database.Connection
}

func (r *workspaceRepository) Get(ctx context.Context, id string) (domain.Workspace, error) {
	v, e := r.q.WorkspaceGetWorkspace(ctx, dtos.UUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Workspace{ProjectID: id}, nil
	}
	return dtos.WorkspaceWorkspace(v), e
}
func (r *workspaceRepository) Assigned(ctx context.Context, id string) ([]string, error) {
	vs, e := r.q.WorkspaceAssignedProjects(ctx, dtos.UUID(id))
	out := []string{}
	for _, v := range vs {
		out = append(out, v.String())
	}
	return out, e
}
func (r *workspaceRepository) Save(ctx context.Context, id string, c domain.WorkspaceChange, actor string) (domain.Workspace, error) {
	var v sqlc.PurchaseFmsWorkspace
	var e error
	if c.Version == 0 {
		v, e = r.q.WorkspaceInsertWorkspace(ctx, sqlc.WorkspaceInsertWorkspaceParams{ProjectID: dtos.UUID(id), ManagerID: dtos.UUID(c.ManagerID), ActorID: dtos.UUID(actor)})
	} else {
		v, e = r.q.WorkspaceTransferOwner(ctx, sqlc.WorkspaceTransferOwnerParams{ProjectID: dtos.UUID(id), ManagerID: dtos.UUID(c.ManagerID), ActorID: dtos.UUID(actor), ExpectedVersion: c.Version})
	}
	if errors.Is(e, pgx.ErrNoRows) {
		e = domain.WorkspaceStale
	}
	return dtos.WorkspaceWorkspace(v), e
}

func (r *workspaceRepository) Record(ctx context.Context, h domain.WorkspaceHistory) error {
	v, e := r.q.WorkspaceInsertOwnerHistory(ctx, sqlc.WorkspaceInsertOwnerHistoryParams{ID: dtos.UUID(h.ID), ProjectID: dtos.UUID(h.ProjectID), PreviousManagerID: dtos.UUID(h.PreviousID), ManagerID: dtos.UUID(h.ManagerID), Version: h.Version, Reason: h.Reason, ActorID: dtos.UUID(h.ActorID)})
	if e != nil {
		return e
	}
	return r.e.Record(ctx, events.Event{ID: h.ID, Module: "purchase_fms", EntityType: "purchase_workspace", EntityID: h.ProjectID, EntityVersion: int64(h.Version), Action: "owner_assigned", ActorID: h.ActorID, ActorKind: "user", CorrelationID: h.ID, ScopeKind: "project", ScopeID: h.ProjectID, Visibility: "restricted", OccurredAt: v.CreatedAt.Time}, nil)
}
func (r *workspaceRepository) History(ctx context.Context, id string) ([]domain.WorkspaceHistory, error) {
	vs, e := r.q.WorkspaceOwnerHistory(ctx, dtos.UUID(id))
	out := []domain.WorkspaceHistory{}
	for _, v := range vs {
		out = append(out, dtos.WorkspaceHistory(v))
	}
	return out, e
}

func (r *workspaceRepository) RegisterStates(ctx context.Context, id string) ([]domain.RegisterState, error) {
	records, err := NewBOReader(r.c).List(ctx, id)
	if err != nil {
		return nil, err
	}
	out := []domain.RegisterState{}
	for _, b := range records {
		if !b.Active {
			continue
		}
		done, err := registerIndentComplete(ctx, r.c, b)
		if err != nil {
			return nil, err
		}
		state := domain.RegisterState{State: b.State, HasItems: len(b.Lines) > 0, IndentComplete: done}
		if r.a != nil {
			source, e := (&quotationApprovalsSourceContext{*r.a}).Source(ctx, id, b.ID)
			if e != nil {
				return nil, e
			}
			docs, e := NewIndentDocumentsReader(r.c, NewIndentGroupsReader(r.c)).List(ctx, id, b.ID)
			if e != nil {
				return nil, e
			}
			state.QuotesComplete = len(docs) > 0
			for _, d := range docs {
				if !d.Active || !d.Data.IsCurrent {
					continue
				}
				received := false
				for _, q := range source.Quotes {
					if q.DocumentID == d.ID && q.Eligible {
						received = true
					}
				}
				state.QuotesComplete = state.QuotesComplete && received
			}
			approvals, e := (&quotationApprovalsRepository{q: NewStore(r.c)}).Get(ctx, id, b.ID)
			if e != nil {
				return nil, e
			}
			if approvals != nil {
				_, latest, ready := domain.QuotationApprovalsEvaluate(approvals.Attempts, source)
				state.ApprovalComplete = ready
				for _, g := range source.Groups {
					a, exists := latest[g.ID]
					if !exists {
						continue
					}
					switch a.EffectiveState {
					case "pending_accounts":
						state.Pending++
					case "rejected", "stale":
						state.Returned++
					case "approved":
						state.Approved++
					}
				}
			}
		}
		if r.later != nil {
			state.LaterCompleted, err = r.later.Read(ctx, id, b)
			if err != nil {
				return nil, err
			}
		}
		out = append(out, state)
	}
	return out, nil
}

func (u *WorkspaceUnitOfWork) WithApprovals(a func(database.Connection) QuotationApprovalsProviders) *WorkspaceUnitOfWork {
	u.a = a
	return u
}

func (u *WorkspaceUnitOfWork) WithLaterProgress(f func(database.Connection) *LaterProgressReader) *WorkspaceUnitOfWork {
	u.later = f
	return u
}
