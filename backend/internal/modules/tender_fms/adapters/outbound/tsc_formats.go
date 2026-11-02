package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
	"time"
)

type TSCFormatUnit struct {
	m       *database.Manager
	users   func(database.Connection) auth.AssignmentDirectory
	journal func(database.Connection) events.Writer
}

func NewTSCFormatUnit(b database.Beginner, u func(database.Connection) auth.AssignmentDirectory, j func(database.Connection) events.Writer) *TSCFormatUnit {
	return &TSCFormatUnit{database.NewManager(b), u, j}
}
func (u *TSCFormatUnit) Within(ctx context.Context, f func(context.Context, ports.TSCFormatRepository) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return f(ctx, &tscFormatRepository{db.New(c), u.users(c), u.journal(c)})
	})
}

type tscFormatRepository struct {
	q       *db.Queries
	users   auth.AssignmentDirectory
	journal events.Writer
}

func (r *tscFormatRepository) Authorize(ctx context.Context, actor string, write bool) error {
	u, e := r.users.Get(ctx, actor)
	if e != nil {
		return e
	}
	if u == nil || !u.Active || !u.Ready {
		return domain.Forbidden
	}
	p := domain.Principal{ID: u.ID, Active: u.Active, Superuser: u.Superuser, Permissions: u.Permissions}
	if !p.Allows("settings.tsc_formats.read") || write && !p.Allows("settings.tsc_formats.write") {
		return domain.Forbidden
	}
	return nil
}
func (r *tscFormatRepository) List(ctx context.Context, active bool) ([]domain.TSCFormat, error) {
	rows, e := r.q.ListTSCFormats(ctx, active)
	out := []domain.TSCFormat{}
	for _, v := range rows {
		out = append(out, domain.TSCFormat{ID: v.ID.String(), Name: v.Name, Description: v.Description, Active: v.IsActive, Version: v.Version, QuestionCount: v.QuestionCount, UpdatedAt: v.UpdatedAt.Time})
	}
	return out, e
}
func (r *tscFormatRepository) Get(ctx context.Context, id string) (domain.TSCFormat, error) {
	v, e := r.q.GetTSCFormat(ctx, uuid(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.TSCFormat{}, domain.CaptureFailure(404, "format_not_found", "TSC format not found")
	}
	if e != nil {
		return domain.TSCFormat{}, e
	}
	out := domain.TSCFormat{ID: v.ID.String(), Name: v.Name, Description: v.Description, Active: v.IsActive, Version: v.Version, UpdatedAt: v.UpdatedAt.Time, Questions: []domain.TSCQuestion{}}
	qs, e := r.q.ListTSCFormatQuestions(ctx, v.ID)
	if e != nil {
		return out, e
	}
	for _, q := range qs {
		out.Questions = append(out.Questions, domain.TSCQuestion{ID: q.ID, Section: q.Section, Number: q.PrintedNumber, Text: q.Question, ManualReview: q.ManualReview, Row: int(q.LegacyRow)})
	}
	out.QuestionCount = int64(len(out.Questions))
	return out, nil
}
func formatDBError(e error) error {
	var pg *pgconn.PgError
	if errors.As(e, &pg) && pg.Code == "23505" {
		return domain.CaptureFailure(409, "duplicate_format_question", "A format with this name or a duplicate question already exists")
	}
	return e
}
func (r *tscFormatRepository) Save(ctx context.Context, id, actor string, in domain.TSCFormatInput) (domain.TSCFormat, error) {
	var before []byte
	if in.Version == 0 {
		if e := r.q.CreateTSCFormat(ctx, db.CreateTSCFormatParams{ID: uuid(id), Name: in.Name, Description: in.Description, IsActive: in.Active, CreatedByID: uuid(actor)}); e != nil {
			return domain.TSCFormat{}, formatDBError(e)
		}
	} else {
		old, e := r.Get(ctx, id)
		if e != nil {
			return old, e
		}
		before, e = json.Marshal(old)
		if e != nil {
			return old, e
		}
		n, e := r.q.UpdateTSCFormat(ctx, db.UpdateTSCFormatParams{ID: uuid(id), Name: in.Name, Description: in.Description, IsActive: in.Active, UpdatedByID: uuid(actor), Version: in.Version})
		if e != nil {
			return old, formatDBError(e)
		}
		if n != 1 {
			return old, domain.CaptureFailure(409, "stale_format", "This format changed. Reload it before saving")
		}
		if e = r.q.ClearTSCFormatQuestions(ctx, uuid(id)); e != nil {
			return old, e
		}
	}
	for i, q := range in.Questions {
		if e := r.q.InsertTSCFormatQuestion(ctx, db.InsertTSCFormatQuestionParams{FormatID: uuid(id), ID: q.ID, Position: int32(i + 1), Section: q.Section, PrintedNumber: q.Number, Question: q.Text, ManualReview: q.ManualReview, LegacyRow: int32(q.Row)}); e != nil {
			return domain.TSCFormat{}, formatDBError(e)
		}
	}
	out, e := r.Get(ctx, id)
	if e != nil {
		return out, e
	}
	after, e := json.Marshal(out)
	if e != nil {
		return out, e
	}
	eventID := Values{}.ID()
	if e = r.q.InsertTSCFormatEvent(ctx, db.InsertTSCFormatEventParams{ID: uuid(eventID), FormatID: uuid(id), Version: out.Version, ActorID: uuid(actor), BeforeSnapshot: before, AfterSnapshot: after}); e != nil {
		return out, e
	}
	e = r.journal.Record(ctx, events.Event{ID: eventID, CorrelationID: eventID, Module: "tender_fms", EntityType: "tsc_format", EntityID: id, EntityVersion: out.Version, Action: "tsc_format_saved", ActorID: actor, ActorKind: "user", ScopeKind: "platform", Visibility: "restricted", OccurredAt: time.Now().UTC()}, nil)
	return out, e
}

func (r *tscRepository) Formats(ctx context.Context) ([]domain.TSCFormat, error) {
	return (&tscFormatRepository{q: r.q}).List(ctx, true)
}
func (r *tscRepository) SelectFormat(ctx context.Context, selection domain.TSCFormatSelection, in *domain.TSCInput) error {
	id := selection.ID
	if id == "" {
		id = domain.DefaultTSCFormatID
	}
	f, e := (&tscFormatRepository{q: r.q}).Get(ctx, id)
	if e != nil {
		return e
	}
	if !f.Active {
		return domain.Invalid("This TSC format is inactive. Select an active format")
	}
	if selection.Version > 0 && selection.Version != f.Version {
		return domain.CaptureFailure(409, "stale_format", "The selected format changed. Refresh available formats before generating")
	}
	in.FormatID = f.ID
	in.FormatName = f.Name
	in.FormatVersion = f.Version
	in.Questions = f.Questions
	return nil
}
