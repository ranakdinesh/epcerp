package outbound

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	inbox "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/application"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/ports"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	"time"
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
func (Values) Now() time.Time   { return time.Now().UTC() }
func uuid(s string) pgtype.UUID { var n pgtype.UUID; _ = n.Scan(s); return n }

type budgetApprover struct {
	c       database.Connection
	users   auth.AssignmentDirectory
	scope   access.Gate
	j       events.Writer
	notices inbox.Writer
	v       ports.Values
}

func NewBudgetApprover(c database.Connection, u auth.AssignmentDirectory, s access.Gate, j events.Writer, n inbox.Writer, v ports.Values) contracts.BudgetApprover {
	return &budgetApprover{c, u, s, j, n, v}
}
func (b *budgetApprover) ApproveBudget(ctx context.Context, a domain.Actor, q domain.BudgetRequest) (string, error) {
	if !database.IsTransactionConnection(ctx, b.c) {
		return "", errors.New("Finance approval requires the consuming budget transaction")
	}
	return application.ApproveBudget(ctx, b, budgetAuthority{b}, b, b.v, a, q)
}
func (b *budgetApprover) Director(ctx context.Context, event string) error {
	_, e := b.notices.DirectorDecision(ctx, event)
	var d *inbox.Error
	if errors.As(e, &d) {
		return &domain.Error{Status: d.Status, Field: d.Field, Message: d.Message}
	}
	return e
}

// authority wrapper keeps the command and authorization interfaces distinct.
type budgetAuthority struct{ b *budgetApprover }

func (a budgetAuthority) ApproveBudget(ctx context.Context, actor domain.Actor, p string) error {
	u, e := a.b.users.Get(ctx, actor.ID)
	if e != nil {
		return e
	}
	if u == nil || !u.Active || u.ID != actor.ID {
		return domain.Unauthorized
	}
	codes := []string{}
	for _, code := range actor.Permissions {
		if u.Allows(code) {
			codes = append(codes, code)
		}
	}
	live := access.Actor{ID: actor.ID, Active: true, Superuser: actor.Superuser && u.Superuser, Permissions: codes}
	if !live.Can("projects.budgets.read") || !live.Can("projects.budgets.approve") {
		return domain.Forbidden
	}
	for _, check := range []struct {
		code  string
		write bool
	}{{"projects.budgets.read", false}, {"projects.budgets.approve", true}} {
		v, e := a.b.scope.Check(ctx, live, p, check.code, check.write)
		if e != nil {
			var d *access.Error
			if errors.As(e, &d) {
				return &domain.Error{Status: d.Status, Field: d.Field, Message: d.Message}
			}
			return e
		}
		if check.write && !v.CanWrite {
			return domain.Forbidden
		}
	}
	return nil
}
func (b *budgetApprover) Record(ctx context.Context, d domain.Decision) error {
	var amount pgtype.Numeric
	if e := amount.Scan(d.Amount); e != nil {
		return e
	}
	at := pgtype.Timestamptz{Time: d.CreatedAt, Valid: true}
	if e := db.New(b.c).RecordBudgetDecision(ctx, db.RecordBudgetDecisionParams{ID: uuid(d.ID), ProjectID: uuid(d.ProjectID), SourceVersion: d.SourceVersion, SourceRevision: d.SourceRevision, Amount: amount, Currency: d.Currency, SourceFingerprint: d.Fingerprint, Reason: d.Reason, CreatedAt: at, ActorID: uuid(d.ActorID)}); e != nil {
		var p *pgconn.PgError
		if errors.As(e, &p) && p.Code == "23505" {
			return domain.Conflict
		}
		return e
	}
	return b.j.Record(ctx, events.Event{ID: d.ID, Module: "finance_approvals", EntityType: "financial_decision", EntityID: d.ID, EntityVersion: 1, Action: "approved", ActorID: d.ActorID, ActorKind: "user", CorrelationID: d.ID, ScopeKind: "project", ScopeID: d.ProjectID, Visibility: "restricted", OccurredAt: d.CreatedAt}, nil)
}
