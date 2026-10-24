package outbound

import (
	"context"
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
)

type advanceReviewer struct{ budgetApprover }

func NewAdvanceReviewer(c database.Connection, u auth.AssignmentDirectory, s access.Gate, j events.Writer, n inbox.Writer, v ports.Values) contracts.AdvanceReviewer {
	return &advanceReviewer{budgetApprover{c, u, s, j, n, v}}
}
func (r *advanceReviewer) ReviewAdvance(ctx context.Context, a domain.Actor, q domain.AdvanceRequest) (string, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return "", errors.New("Finance advance decision requires the consuming transaction")
	}
	return application.ReviewAdvance(ctx, r, advanceAuthority{r}, r, r.v, a, q)
}

type advanceAuthority struct{ r *advanceReviewer }

func (a advanceAuthority) ReviewAdvance(ctx context.Context, actor domain.Actor, p string) error {
	u, e := a.r.users.Get(ctx, actor.ID)
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
	for _, code := range []string{"accounts.purchase_advances.view", "accounts.purchase_advances.read", "accounts.purchase_advances.write"} {
		if !live.Can(code) {
			return domain.Forbidden
		}
		v, e := a.r.scope.Check(ctx, live, p, code, code == "accounts.purchase_advances.write")
		if e != nil {
			var d *access.Error
			if errors.As(e, &d) {
				return &domain.Error{Status: d.Status, Field: d.Field, Message: d.Message}
			}
			return e
		}
		if code == "accounts.purchase_advances.write" && !v.CanWrite {
			return domain.Forbidden
		}
	}
	return nil
}
func (r *advanceReviewer) RecordAdvance(ctx context.Context, d domain.AdvanceDecision) error {
	var amount pgtype.Numeric
	if e := amount.Scan(d.Amount); e != nil {
		return e
	}
	at := pgtype.Timestamptz{Time: d.CreatedAt, Valid: true}
	e := db.New(r.c).RecordAdvanceDecision(ctx, db.RecordAdvanceDecisionParams{ID: uuid(d.ID), ProjectID: uuid(d.ProjectID), PackageID: uuid(d.PackageID), RequestID: uuid(d.RequestID), OrderID: uuid(d.OrderID), OrderVersion: d.OrderVersion, Kind: d.Kind, SourceVersion: d.Version, SubmittedByID: uuid(d.SubmittedBy), Outcome: d.Outcome, Amount: amount, Currency: d.Currency, SourceFingerprint: d.Fingerprint, Reason: d.Reason, CreatedAt: at, ActorID: uuid(d.ActorID)})
	if e != nil {
		var p *pgconn.PgError
		if errors.As(e, &p) && p.Code == "23505" {
			return domain.Conflict
		}
		return e
	}
	return r.j.Record(ctx, events.Event{ID: d.ID, Module: "finance_approvals", EntityType: "financial_decision", EntityID: d.ID, EntityVersion: 1, Action: d.Outcome, ActorID: d.ActorID, ActorKind: "user", CorrelationID: d.ID, ScopeKind: "project", ScopeID: d.ProjectID, Visibility: "restricted", OccurredAt: d.CreatedAt}, nil)
}
