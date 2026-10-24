package outbound

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/work_calendar/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/work_calendar/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/work_calendar/ports"
	"time"
)

const LockKey = 70623124

func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic("random unavailable")
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func uuid(s string) pgtype.UUID { var v pgtype.UUID; _ = v.Scan(s); return v }

type Repository struct {
	Conn   database.Connection
	Events events.Writer
}

func (r Repository) List(ctx context.Context, year int) ([]domain.Holiday, error) {
	rows, e := db.New(r.Conn).ListHolidays(ctx, int32(year))
	out := []domain.Holiday{}
	for _, v := range rows {
		out = append(out, holiday(v))
	}
	return out, e
}
func holiday(v db.WorkCalendarHoliday) domain.Holiday {
	return domain.Holiday{ID: v.ID.String(), Name: v.Name, Date: v.HolidayDate.Time.Format("2006-01-02"), Active: v.IsActive, Version: v.Version}
}
func (r Repository) Dates(ctx context.Context) (map[string]bool, error) {
	if _, e := r.Conn.Exec(ctx, "SELECT pg_advisory_xact_lock_shared($1)", int64(LockKey)); e != nil {
		return nil, e
	}
	rows, e := db.New(r.Conn).HolidayDates(ctx)
	out := map[string]bool{}
	for _, d := range rows {
		out[d.Time.Format("2006-01-02")] = true
	}
	return out, e
}
func (r Repository) Save(ctx context.Context, c domain.Change, actor string) (domain.Holiday, error) {
	if _, e := r.Conn.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", int64(LockKey)); e != nil {
		return domain.Holiday{}, e
	}
	q := db.New(r.Conn)
	d, _ := domain.ValidDate(c.Date)
	var row db.WorkCalendarHoliday
	var e error
	if c.ID == "" {
		c.ID = ID()
		row, e = q.CreateHoliday(ctx, db.CreateHolidayParams{ID: uuid(c.ID), Name: c.Name, HolidayDate: pgtype.Date{Time: d, Valid: true}, IsActive: c.Active, Actor: uuid(actor)})
	} else {
		if !uuid(c.ID).Valid || c.Version < 1 {
			return domain.Holiday{}, domain.Invalid
		}
		row, e = q.UpdateHoliday(ctx, db.UpdateHolidayParams{ID: uuid(c.ID), Name: c.Name, HolidayDate: pgtype.Date{Time: d, Valid: true}, IsActive: c.Active, Actor: uuid(actor), ExpectedVersion: c.Version})
	}
	var pg *pgconn.PgError
	if errors.Is(e, pgx.ErrNoRows) || (errors.As(e, &pg) && pg.Code == "23505") {
		return domain.Holiday{}, domain.Conflict
	}
	if e != nil {
		return domain.Holiday{}, e
	}
	v := holiday(row)
	snapshot, _ := json.Marshal(v)
	eventID := ID()
	if e = q.HolidayHistory(ctx, db.HolidayHistoryParams{ID: uuid(eventID), HolidayID: uuid(v.ID), Version: v.Version, Reason: c.Reason, Snapshot: snapshot, Actor: uuid(actor)}); e != nil {
		return v, e
	}
	e = r.Events.Record(ctx, events.Event{ID: eventID, Module: "work_calendar", EntityType: "holiday", EntityID: v.ID, EntityVersion: int64(v.Version), Action: "saved", ActorID: actor, ActorKind: "user", CorrelationID: eventID, ScopeKind: "platform", Visibility: "restricted", OccurredAt: time.Now().UTC()}, nil)
	return v, e
}

type Unit struct {
	Manager *database.Manager
	Events  func(database.Connection) events.Writer
	Refresh func(database.Connection) func(context.Context, string) (int, error)
}

func (u Unit) Within(ctx context.Context, f func(context.Context, ports.Repository, func(context.Context, string) (int, error)) error) error {
	return u.Manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return f(ctx, Repository{c, u.Events(c)}, u.Refresh(c))
	})
}

type Auth struct{ Resolver auth.Resolver }

func (a Auth) Resolve(ctx context.Context, t string) (ports.Principal, error) {
	p, e := a.Resolver.Resolve(ctx, t)
	if e != nil || !p.Active || p.ID == "" {
		return ports.Principal{}, domain.Unauthorized
	}
	allows := func(code string) bool {
		if !p.Active {
			return false
		}
		if p.Superuser {
			return true
		}
		for _, v := range p.Permissions {
			if v == code {
				return true
			}
		}
		return false
	}
	return ports.Principal{ID: p.ID, CanRead: allows("settings.holidays.read"), CanWrite: allows("settings.holidays.write")}, nil
}
