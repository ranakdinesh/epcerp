package outbound

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
)

func (r *tscRepository) CloudBudget(ctx context.Context, draft string) (int64, error) {
	return r.q.GetTSCBudget(ctx, uuid(draft))
}

func (r *tscRepository) ReserveCloud(ctx context.Context, call domain.TSCCloudCall) (domain.TSCCloudCall, bool, error) {
	raw, e := json.Marshal([]any{call.JobID, call.ActorID, call.Provider, call.Model, call.Input})
	if e != nil {
		return call, false, e
	}
	sum := sha256.Sum256(raw)
	call.Fingerprint = hex.EncodeToString(sum[:])
	// All paid operations for one tender serialize on the same source-owned row.
	if e := r.q.LockTSCBudget(ctx, uuid(call.DraftID)); e != nil {
		return call, false, e
	}
	old, e := r.q.FindTSCCloudCall(ctx, db.FindTSCCloudCallParams{DraftID: uuid(call.DraftID), RequestKey: call.Key})
	if e == nil {
		if old.Fingerprint != call.Fingerprint || old.ActorID.String() != call.ActorID {
			return call, false, domain.CaptureFailure(409, "idempotency_conflict", "This research operation belongs to different inputs")
		}
		call.ID = old.ID.String()
		call.State = old.State
		call.Error = old.ErrorCode
		if len(old.ResultCipher) > 0 {
			raw, e := r.cipher.Open(old.ResultCipher, old.ResultHash.String)
			if e != nil {
				return call, false, e
			}
			if e = json.Unmarshal(raw, &call.Answer); e != nil {
				return call, false, e
			}
		}
		return call, false, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return call, false, e
	}
	used, e := r.CloudBudget(ctx, call.DraftID)
	if e != nil {
		return call, false, e
	}
	if call.ReservedPaise <= 0 || used+call.ReservedPaise > domain.TSCCloudBudgetPaise {
		return call, false, domain.CaptureFailure(409, "cloud_budget_exhausted", "The INR 100 tender cloud allowance is exhausted; use source search and manual correction")
	}
	call.ID = Values{}.ID()
	call.State = "pending"
	_, e = r.q.ReserveTSCCloudCall(ctx, db.ReserveTSCCloudCallParams{ID: uuid(call.ID), DraftID: uuid(call.DraftID), JobID: uuid(call.JobID), ActorID: uuid(call.ActorID), RequestKey: call.Key, Fingerprint: call.Fingerprint, Provider: call.Provider, Model: call.Model, ReservedPaise: call.ReservedPaise})
	if e != nil {
		return call, false, e
	}
	j := domain.TSCJob{ID: call.JobID, Version: 1}
	return call, true, r.event(ctx, j, "cloud_reserved", call.ID+":"+call.Provider, call.ActorID, nil, "")
}

func (r *tscRepository) CompleteCloud(ctx context.Context, call domain.TSCCloudCall, answer domain.TSCAnswer, code string) error {
	state := "failed"
	var cipher []byte
	var hash string
	if code == "" {
		raw, e := json.Marshal(answer)
		if e != nil {
			return e
		}
		cipher, hash, e = r.cipher.Seal(raw)
		if e != nil {
			return e
		}
		state = "ready"
	}
	n, e := r.q.CompleteTSCCloudCall(ctx, db.CompleteTSCCloudCallParams{ID: uuid(call.ID), State: state, ErrorCode: code, ResultCipher: cipher, ResultHash: pgtype.Text{String: hash, Valid: hash != ""}})
	if e != nil {
		return e
	}
	if n != 1 {
		return domain.CaptureFailure(409, "cloud_call_changed", "Cloud request has already been completed")
	}
	return r.event(ctx, domain.TSCJob{ID: call.JobID, Version: 1}, "cloud_"+state, call.ID+":"+call.Provider+":"+code, call.ActorID, cipher, hash)
}
