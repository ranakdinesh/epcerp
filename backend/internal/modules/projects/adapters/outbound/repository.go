package outbound

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/ports"
	purchaseContracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	calendarContracts "github.com/ranakdinesh/epcERP/internal/modules/work_calendar/contracts"
	"strconv"
	"strings"
	"time"
)

type UnitOfWork struct {
	assignments func(database.Connection) access.ReadableProjects
	manager     *database.Manager
	clients     func(database.Connection) masterContracts.ClientDirectory
	users       func(database.Connection) auth.AssignmentDirectory
	events      func(database.Connection) events.Writer
	purchasing  func(database.Connection) purchaseContracts.WorkspaceOwnership
	calendar    func(database.Connection) calendarContracts.Reader
}

func NewUnitOfWork(b database.Beginner, c func(database.Connection) masterContracts.ClientDirectory, u func(database.Connection) auth.AssignmentDirectory, e func(database.Connection) events.Writer, calendars ...func(database.Connection) calendarContracts.Reader) *UnitOfWork {
	unit := &UnitOfWork{manager: database.NewManager(b), clients: c, users: u, events: e}
	if len(calendars) > 0 {
		unit.calendar = calendars[0]
	}
	return unit
}
func (u *UnitOfWork) Within(ctx context.Context, work func(context.Context, ports.Repository, ports.Clients, ports.Users, ports.Calendar, ports.Purchasing) error) error {
	return u.manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		var repo ports.Repository = &repository{db.New(c), u.events(c)}
		if u.assignments != nil {
			repo = &scopedReadRepository{repo.(*repository), u.assignments(c)}
		}
		return work(ctx, repo, &clientBridge{u.clients(c)}, &userBridge{u.users(c)}, calendarBridge{factory: u.calendar, conn: c}, purchaseBridge{factory: u.purchasing, conn: c})
	})
}

type clientBridge struct {
	reader masterContracts.ClientDirectory
}

func (b *clientBridge) Get(ctx context.Context, id string) (*domain.Client, error) {
	v, e := b.reader.Get(ctx, id)
	if e != nil || v == nil {
		return nil, e
	}
	return &domain.Client{ID: v.ID, Name: v.Name, Code: v.Code, Active: v.Active}, nil
}
func (b *clientBridge) List(ctx context.Context) ([]domain.Client, error) {
	rows, e := b.reader.List(ctx)
	if e != nil {
		return nil, e
	}
	result := []domain.Client{}
	for _, v := range rows {
		result = append(result, domain.Client{ID: v.ID, Name: v.Name, Code: v.Code, Active: v.Active})
	}
	return result, nil
}

type userBridge struct{ reader auth.AssignmentDirectory }

func (b *userBridge) Get(ctx context.Context, id string) (*domain.Candidate, error) {
	v, e := b.reader.Get(ctx, id)
	if e != nil || v == nil {
		return nil, e
	}
	return &domain.Candidate{ID: v.ID, Name: v.Name, Active: v.Active, Superuser: v.Superuser, Permissions: v.Permissions}, nil
}
func (b *userBridge) List(ctx context.Context) ([]domain.Candidate, error) {
	rows, e := b.reader.List(ctx)
	if e != nil {
		return nil, e
	}
	result := []domain.Candidate{}
	for _, v := range rows {
		result = append(result, domain.Candidate{ID: v.ID, Name: v.Name, Active: v.Active, Superuser: v.Superuser, Permissions: v.Permissions})
	}
	return result, nil
}

type AuthBridge struct{ resolver auth.Resolver }

func NewAuthBridge(r auth.Resolver) *AuthBridge { return &AuthBridge{r} }
func (b *AuthBridge) Resolve(ctx context.Context, t string) (domain.Principal, error) {
	v, e := b.resolver.Resolve(ctx, t)
	return domain.Principal{ID: v.ID, Active: v.Active, Superuser: v.Superuser, Permissions: v.Permissions}, e
}

type SystemValues struct{}

func (SystemValues) ID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic("random source unavailable")
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

type repository struct {
	q      *db.Queries
	events events.Writer
}

func uuid(s string) pgtype.UUID { var v pgtype.UUID; _ = v.Scan(s); return v }
func key(v pgtype.UUID) string {
	if !v.Valid {
		return ""
	}
	return v.String()
}
func project(v db.Project) domain.Project {
	return domain.Project{Capture: capture(v), ID: v.ID.String(), Number: v.ProjectNumber, Name: v.Name, NameKey: v.NormalizedName, ClientID: v.ClientID.String(), ManagerID: v.ProjectManagerID.String(), Version: v.Version, Active: v.IsActive, Lifecycle: v.LifecycleStatus, CreatedAt: v.CreatedAt.Time.UTC(), UpdatedAt: v.UpdatedAt.Time.UTC(), CreatedBy: key(v.CreatedByID), UpdatedBy: key(v.UpdatedByID)}
}
func conflict(e error) error {
	var p *pgconn.PgError
	if errors.As(e, &p) && (p.Code == "23505" || p.Code == "23503") {
		return domain.ErrConflict
	}
	return e
}
func (r *repository) Get(ctx context.Context, id string, lock bool) (*domain.Project, error) {
	var v db.Project
	var e error
	if lock {
		v, e = r.q.LockProject(ctx, uuid(id))
	} else {
		v, e = r.q.GetProject(ctx, uuid(id))
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	p := project(v)
	return &p, nil
}
func (r *repository) List(ctx context.Context, f domain.Filter, manager string) (domain.Page, error) {
	return r.listAssigned(ctx, f, manager, nil)
}
func (r *repository) listAssigned(ctx context.Context, f domain.Filter, manager string, ids []string) (domain.Page, error) {
	search := ""
	if f.Search != "" {
		search = "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(f.Search) + "%"
	}
	rows, e := r.q.ListProjects(ctx, db.ListProjectsParams{AssignedIds: projectReadUUIDs(ids), ManagerID: uuid(manager), Search: search, PageOffset: (f.Page - 1) * f.PageSize, PageSize: f.PageSize})
	if e != nil {
		return domain.Page{}, e
	}
	total, e := r.q.CountProjects(ctx, db.CountProjectsParams{AssignedIds: projectReadUUIDs(ids), ManagerID: uuid(manager), Search: search})
	result := domain.Page{Items: []domain.Project{}, Total: total, Page: f.Page, PageSize: f.PageSize}
	for _, v := range rows {
		result.Items = append(result.Items, project(v))
	}
	return result, e
}
func (r *repository) Create(ctx context.Context, p domain.Project) (domain.Project, error) {
	v, e := r.q.CreateProject(ctx, db.CreateProjectParams{ID: uuid(p.ID), Name: p.Name, NormalizedName: p.NameKey, ClientID: uuid(p.ClientID), ProjectManagerID: uuid(p.ManagerID), ActorID: uuid(p.CreatedBy), ShortName: p.ShortName, ProjectType: p.ProjectType, SiteName: p.SiteName, SiteState: p.SiteState, SiteCity: p.SiteCity, ScopeSummary: p.ScopeSummary, OperationsManagerID: p.OperationsManagerID, ContractValue: p.ContractValue, Currency: p.Currency, LoiNumber: p.LOINumber, PoNumber: p.PONumber, ScheduleStartBasis: p.ScheduleStartBasis, PlannedStartDate: p.PlannedStartDate, PlannedCompletionDate: p.PlannedCompletionDate, LoiDate: p.LOIDate, PoDate: p.PODate, ActualStartDate: p.ActualStartDate, SiteHandoverDate: p.SiteHandoverDate, PlannedDurationDays: p.PlannedDurationDays, SafetyBufferPercent: p.SafetyBufferPercent, SafetyBufferDays: p.SafetyBufferDays, Remarks: p.Remarks, ActualEndDate: p.ActualEndDate, PurchaseEnabled: p.PurchaseEnabled, ScheduleCalculation: calculationJSON(p.Calculation)})
	return project(v), conflict(e)
}
func (r *repository) Transfer(ctx context.Context, p domain.Project, manager string) (domain.Project, error) {
	v, e := r.q.TransferManager(ctx, db.TransferManagerParams{ID: uuid(p.ID), ManagerID: uuid(manager), ExpectedVersion: p.Version, ActorID: uuid(p.UpdatedBy)})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Project{}, domain.ErrStale
	}
	return project(v), conflict(e)
}
func (r *repository) Assignment(ctx context.Context, a domain.Assignment) error {
	if e := r.q.InsertManagerHistory(ctx, db.InsertManagerHistoryParams{ID: uuid(a.ID), ProjectID: uuid(a.ProjectID), PreviousManagerID: uuid(a.BeforeManagerID), ManagerID: uuid(a.ManagerID), Version: a.Version, Reason: a.Reason, OccurredAt: pgtype.Timestamptz{Time: a.OccurredAt, Valid: true}, ActorID: uuid(a.ActorID)}); e != nil {
		return e
	}
	action := "manager_reassigned"
	if a.BeforeManagerID == "" {
		action = "created"
	}
	return r.events.Record(ctx, events.Event{ID: a.ID, Module: "projects", EntityType: "project", EntityID: a.ProjectID, EntityVersion: int64(a.Version), Action: action, ActorID: a.ActorID, ActorKind: "user", CorrelationID: a.ID, ScopeKind: "project", ScopeID: a.ProjectID, Visibility: "restricted", OccurredAt: a.OccurredAt}, nil)
}
func (r *repository) History(ctx context.Context, id string) ([]domain.Assignment, error) {
	rows, e := r.q.ManagerHistory(ctx, uuid(id))
	if e != nil {
		return nil, e
	}
	result := []domain.Assignment{}
	for _, v := range rows {
		result = append(result, domain.Assignment{ID: v.ID.String(), ProjectID: v.ProjectID.String(), BeforeManagerID: key(v.PreviousManagerID), ManagerID: v.ProjectManagerID.String(), Reason: v.Reason, Version: v.ProjectVersion, ActorID: key(v.CreatedByID), OccurredAt: v.CreatedAt.Time.UTC()})
	}
	return result, nil
}

type calendarBridge struct {
	factory func(database.Connection) calendarContracts.Reader
	conn    database.Connection
}

func (b calendarBridge) Count(ctx context.Context, start, end string) (domain.WorkingCount, error) {
	if b.factory == nil {
		return domain.WorkingCount{}, errors.New("Working calendar is not configured")
	}
	v, e := b.factory(b.conn).Count(ctx, start, end)
	return domain.WorkingCount{CalendarDays: v.CalendarDays, WeekendDays: v.WeekendDays, HolidayDays: v.HolidayDays, WorkingDays: v.WorkingDays}, e
}
func calculationJSON(c domain.Calculation) []byte { v, _ := json.Marshal(c); return v }
func (r *repository) RecalculationProjects(ctx context.Context) ([]domain.Project, error) {
	rows, e := r.q.RecalculationProjects(ctx)
	out := []domain.Project{}
	for _, v := range rows {
		out = append(out, project(v))
	}
	return out, e
}
func (r *repository) SaveCalculation(ctx context.Context, p domain.Project, actor string) error {
	id := SystemValues{}.ID()
	if e := r.q.CalculationHistory(ctx, db.CalculationHistoryParams{EventID: uuid(id), ProjectID: uuid(p.ID), Actor: uuid(actor), Calculation: calculationJSON(p.Calculation)}); e != nil {
		return e
	}
	days, _ := strconv.Atoi(p.PlannedDurationDays)
	buffer, _ := strconv.Atoi(p.SafetyBufferDays)
	if e := r.q.SaveCalculation(ctx, db.SaveCalculationParams{ID: uuid(p.ID), Actor: uuid(actor), Days: pgtype.Int4{Int32: int32(days), Valid: true}, Buffer: pgtype.Int4{Int32: int32(buffer), Valid: true}, Calculation: calculationJSON(p.Calculation)}); e != nil {
		return e
	}
	return r.events.Record(ctx, events.Event{ID: id, Module: "projects", EntityType: "project", EntityID: p.ID, EntityVersion: int64(p.Version + 1), Action: "calendar_recalculated", ActorID: actor, ActorKind: "user", CorrelationID: id, ScopeKind: "project", ScopeID: p.ID, Visibility: "restricted", OccurredAt: time.Now().UTC()}, nil)
}

func (u *UnitOfWork) WithPurchasing(f func(database.Connection) purchaseContracts.WorkspaceOwnership) *UnitOfWork {
	u.purchasing = f
	return u
}

type purchaseBridge struct {
	factory func(database.Connection) purchaseContracts.WorkspaceOwnership
	conn    database.Connection
}

func (b purchaseBridge) Candidates(ctx context.Context) ([]domain.Candidate, error) {
	if b.factory == nil {
		return nil, errors.New("Purchase ownership not configured")
	}
	rows, e := b.factory(b.conn).Candidates(ctx)
	out := []domain.Candidate{}
	for _, r := range rows {
		out = append(out, domain.Candidate{ID: r.ID, Name: r.Name, Active: r.Active, Superuser: r.Superuser, Permissions: r.Permissions})
	}
	return out, e
}
func (b purchaseBridge) Owner(ctx context.Context, id string) (string, error) {
	if b.factory == nil {
		return "", errors.New("Purchase ownership not configured")
	}
	return b.factory(b.conn).Owner(ctx, id)
}
func (b purchaseBridge) Initialize(ctx context.Context, id, user, actor string) error {
	if b.factory == nil {
		return errors.New("Purchase ownership not configured")
	}
	e := b.factory(b.conn).Initialize(ctx, id, user, actor)
	if errors.Is(e, purchaseContracts.WorkspaceErrIneligible) {
		return &domain.Invalid{Field: "procurement_executive_id", Message: "Select an active Procurement Executive with purchase view/read permissions"}
	}
	if errors.Is(e, purchaseContracts.WorkspaceErrAlreadyAssigned) {
		return domain.ErrConflict
	}
	return e
}
