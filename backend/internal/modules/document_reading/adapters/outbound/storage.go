package outbound

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/ports"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

type Values struct{}

func (Values) ID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic("random source unavailable")
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
func uuid(s string) pgtype.UUID { var v pgtype.UUID; _ = v.Scan(s); return v }
func identity(v pgtype.UUID) string {
	if !v.Valid {
		return ""
	}
	return v.String()
}

type repository struct {
	q      *db.Queries
	events events.Writer
}

func NewRepository(c database.Connection, e events.Writer) ports.Repository {
	return &repository{db.New(c), e}
}
func record(v db.DocumentReading) domain.Document {
	warnings := []string{}
	_ = json.Unmarshal(v.Warnings, &warnings)
	return domain.Document{ID: v.ID.String(), Module: v.Module, EntityID: v.EntityID.String(), Filename: v.Filename, Format: v.Format, Size: v.SizeBytes, SHA256: v.Sha256, Original: v.Original, Status: v.Status, Version: v.Version, Attempts: v.Attempts, ClaimID: identity(v.ClaimID), Error: v.ErrorMessage, SegmentCount: v.SegmentCount, Warnings: warnings, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, ActorID: identity(v.CreatedByID)}
}
func (r *repository) Submit(ctx context.Context, in domain.Input, sealed []byte, id string) (domain.Document, error) {
	v, e := r.q.SubmitReading(ctx, db.SubmitReadingParams{ID: uuid(id), Module: in.Module, EntityID: uuid(in.EntityID), Filename: in.Filename, Format: in.Format, SizeBytes: sourceSize(in), Sha256: in.SHA256, Original: sealed, ActorID: uuid(in.ActorID)})
	if errors.Is(e, pgx.ErrNoRows) {
		v, e = r.q.FindReading(ctx, db.FindReadingParams{Module: in.Module, EntityID: uuid(in.EntityID), Sha256: in.SHA256})
		return record(v), e
	}
	if e != nil {
		return domain.Document{}, e
	}
	d := record(v)
	return d, r.Event(ctx, d, "queued", in.ActorID)
}
func (r *repository) Get(ctx context.Context, module, entity, id string) (domain.Document, error) {
	v, e := r.q.GetReading(ctx, db.GetReadingParams{ID: uuid(id), Module: module, EntityID: uuid(entity)})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Document{}, domain.Missing
	}
	return record(v), e
}
func (r *repository) List(ctx context.Context, module, entity string) ([]domain.Document, error) {
	rows, e := r.q.ListReadings(ctx, db.ListReadingsParams{Module: module, EntityID: uuid(entity)})
	out := []domain.Document{}
	for _, v := range rows {
		d := record(db.DocumentReading{ID: v.ID, Module: v.Module, EntityID: v.EntityID, Filename: v.Filename, Format: v.Format, SizeBytes: v.SizeBytes, Sha256: v.Sha256, Status: v.Status, Version: v.Version, Attempts: v.Attempts, ErrorMessage: v.ErrorMessage, SegmentCount: v.SegmentCount, Warnings: v.Warnings, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, CreatedByID: v.CreatedByID})
		out = append(out, d)
	}
	return out, e
}
func (r *repository) Result(ctx context.Context, id string) ([]byte, string, error) {
	v, e := r.q.GetReadingResult(ctx, uuid(id))
	return v.Result, v.ResultSha256, e
}
func (r *repository) Retry(ctx context.Context, id string, version int32, actor string) (domain.Document, error) {
	v, e := r.q.RetryReading(ctx, db.RetryReadingParams{ID: uuid(id), ExpectedVersion: version, ActorID: uuid(actor)})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Document{}, domain.Conflict
	}
	return record(v), e
}
func (r *repository) Claim(ctx context.Context, claim string) (*domain.Document, error) {
	v, e := r.q.ClaimReading(ctx, uuid(claim))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	d := record(v)
	return &d, nil
}
func (r *repository) Finish(ctx context.Context, job domain.Document, cipher []byte, hash string, result domain.Result, failure string) (domain.Document, error) {
	status := "ready"
	warnings := result.Warnings
	count := int32(len(result.Segments))
	if failure != "" {
		status = "failed"
		warnings = []string{}
		count = 0
	}
	if warnings == nil {
		warnings = []string{}
	}
	raw, e := json.Marshal(warnings)
	if e != nil {
		return domain.Document{}, e
	}
	v, e := r.q.FinishReading(ctx, db.FinishReadingParams{ID: uuid(job.ID), ClaimID: uuid(job.ClaimID), Status: status, ErrorMessage: failure, SegmentCount: count, Warnings: raw})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Document{}, domain.Conflict
	}
	if e != nil {
		return domain.Document{}, e
	}
	if failure == "" {
		e = r.q.InsertReadingResult(ctx, db.InsertReadingResultParams{DocumentID: uuid(job.ID), Result: cipher, ResultSha256: hash, ReaderVersion: result.ReaderVersion})
	}
	return record(v), e
}
func (r *repository) Event(ctx context.Context, d domain.Document, action, actor string) error {
	id := Values{}.ID()
	if e := r.q.InsertReadingEvent(ctx, db.InsertReadingEventParams{ID: uuid(id), DocumentID: uuid(d.ID), Action: action, Version: d.Version, ActorID: uuid(actor)}); e != nil {
		return e
	}
	kind := "user"
	if actor == "" {
		kind = "system"
	}
	return r.events.Record(ctx, events.Event{ID: id, Module: "document_reading", EntityType: "document_reading", EntityID: d.ID, EntityVersion: int64(d.Version), Action: action, ActorID: actor, ActorKind: kind, CorrelationID: id, ScopeKind: "platform", Visibility: "restricted", OccurredAt: d.UpdatedAt}, nil)
}
func (r *repository) History(ctx context.Context, id string) ([]domain.Event, error) {
	rows, e := r.q.ReadingHistory(ctx, uuid(id))
	out := []domain.Event{}
	for _, v := range rows {
		out = append(out, domain.Event{ID: v.ID.String(), DocumentID: v.DocumentID.String(), Action: v.Action, Version: v.Version, ActorID: identity(v.ActorID), CreatedAt: v.CreatedAt.Time})
	}
	return out, e
}

type UnitOfWork struct {
	m *database.Manager
	e func(database.Connection) events.Writer
}

func NewUnitOfWork(b database.Beginner, e func(database.Connection) events.Writer) *UnitOfWork {
	return &UnitOfWork{database.NewManager(b), e}
}
func (u *UnitOfWork) Within(ctx context.Context, w func(context.Context, ports.Repository) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error { return w(ctx, NewRepository(c, u.e(c))) })
}

func sourceSize(in domain.Input) int64 {
	if in.SizeBytes > 0 {
		return in.SizeBytes
	}
	return int64(len(in.Original))
}

func (u *UnitOfWork) KeepClaim(ctx context.Context, job domain.Document) (context.Context, func()) {
	return infrastructure.KeepClaim(ctx, func(ctx context.Context) error {
		return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
			n, e := db.New(c).RenewReadingClaim(ctx, db.RenewReadingClaimParams{ID: uuid(job.ID), ClaimID: uuid(job.ClaimID)})
			if e != nil {
				return e
			}
			if n != 1 {
				return domain.Conflict
			}
			return nil
		})
	})
}
