package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	reading "github.com/ranakdinesh/epcERP/internal/modules/document_reading/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
	"strings"
	"time"
)

type TSCUnit struct {
	drafts  *DraftUnit
	cipher  *infrastructure.DocumentCipher
	journal func(database.Connection) events.Writer
}

func NewTSCUnit(d *DraftUnit, c *infrastructure.DocumentCipher, j func(database.Connection) events.Writer) *TSCUnit {
	return &TSCUnit{d, c, j}
}
func (u *TSCUnit) Within(ctx context.Context, f func(context.Context, ports.TSCRepository, ports.DraftRepository) error) error {
	return u.drafts.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		d := &DraftRepository{c, db.New(c), u.drafts.users(c), u.drafts.packages(c), u.drafts.reader(c), u.drafts.prepared(c)}
		return f(ctx, &tscRepository{q: db.New(c), drafts: d, cipher: u.cipher, journal: u.journal(c)}, d)
	})
}

type tscRepository struct {
	q       *db.Queries
	drafts  *DraftRepository
	cipher  *infrastructure.DocumentCipher
	journal events.Writer
}

func (r *tscRepository) Authorize(ctx context.Context, actor, action string) error {
	u, e := r.drafts.users.Get(ctx, actor)
	if e != nil {
		return e
	}
	if u == nil || !u.Active || !u.Ready {
		return domain.Forbidden
	}
	p := domain.Principal{ID: u.ID, Active: u.Active, Superuser: u.Superuser, Permissions: u.Permissions}
	if !p.Allows("tenders.tsc.read") || !p.Allows("tenders.tsc."+action) {
		return domain.Forbidden
	}
	return nil
}
func (r *tscRepository) Snapshot(ctx context.Context, d domain.DocumentDraft) (domain.TSCInput, error) {
	in, e := TSCRegistry()
	if e != nil {
		return in, e
	}
	in.Name = d.Name
	if e = r.drafts.Populate(ctx, &d); e != nil {
		return in, e
	}
	if e = domain.ValidateTSCSources(d); e != nil {
		return in, e
	}
	seen := map[string]bool{}
	total := 0
	for _, b := range d.Batches {
		for _, source := range b.Sources {
			in.Sources = append(in.Sources, domain.TSCSnapshotSource{BatchID: b.ID, Source: source})
			if domain.IsTSCReferenceSource(source.Path) {
				in.Parts = append(in.Parts, domain.TSCSourcePart{DocumentID: source.ID, Path: source.Path, Disposition: "excluded", Reason: "Completed checklist/reference answers are excluded from generation evidence"})
				in.Warnings = append(in.Warnings, source.Path+": checklist/reference document excluded from TSC evidence.")
				continue
			}
			for _, w := range source.Warnings {
				in.Warnings = append(in.Warnings, source.Path+": "+w)
			}
			if seen[source.ID] {
				continue
			}
			seen[source.ID] = true
			raw, e := r.drafts.Artifact(ctx, d, source.ID, "result")
			if e != nil {
				return in, e
			}
			total += len(raw)
			if total > 16<<20 {
				return in, domain.Invalid("Combined reading evidence exceeds the 16 MiB TSC snapshot limit; no sources were silently omitted")
			}
			var result reading.Result
			if e = json.Unmarshal(raw, &result); e != nil {
				return in, e
			}
			if result.SourceSHA256 != source.SHA256 {
				return in, errors.New("Reading hash mismatch")
			}
			for _, part := range result.Parts {
				in.Parts = append(in.Parts, domain.TSCSourcePart{DocumentID: source.ID, Path: part.Path, Disposition: part.Disposition, Reason: part.Reason})
			}
			evidence := reading.NormalizeEvidence(result)
			in.Coverage.Version = evidence.Version
			in.Coverage.Documents++
			in.Coverage.Segments += evidence.Segments
			in.Coverage.EmptySegments += evidence.EmptySegments
			in.Coverage.Blocks += len(evidence.Blocks)
			in.Coverage.Suppressed += evidence.Suppressed
			in.Coverage.LowConfidence += evidence.LowConfidence
			documentType := tscDocumentType(source.Path, evidence.Blocks)
			for _, block := range evidence.Blocks {
				in.Passages = append(in.Passages, domain.TSCPassage{
					ID: source.ID + ":" + block.ID, DocumentID: source.ID, Revision: source.Version, SHA256: source.SHA256,
					Filename: source.Path, Locator: fmt.Sprintf("%s / bytes %d-%d", block.Label, block.Start, block.End),
					Text: block.Text, SearchText: block.SearchText, Heading: block.Heading, Kind: block.Kind, Method: block.Method,
					Confidence: block.Confidence, Segment: block.Segment, Start: block.Start, End: block.End,
					Alternative: block.Alternative, Suppressed: block.Suppressed, Reason: block.Reason, DocumentType: documentType,
				})
			}
		}
	}
	if len(in.Passages) == 0 {
		return in, domain.Invalid("No readable source text is available for TSC generation")
	}
	if in.Coverage.EmptySegments > 0 {
		in.Warnings = append(in.Warnings, fmt.Sprintf("%d source segments have no text; inspect the original pages before treating missing answers as absent.", in.Coverage.EmptySegments))
	}
	if in.Coverage.LowConfidence > 0 {
		in.Warnings = append(in.Warnings, fmt.Sprintf("%d source segments have low OCR confidence; figures, dates and conditions require source review.", in.Coverage.LowConfidence))
	}
	return in, nil
}
func (r *tscRepository) decode(v db.TenderTscJob) (domain.TSCJob, error) {
	out := domain.TSCJob{ID: v.ID.String(), DraftID: v.DraftID.String(), DraftVersion: v.DraftVersion, ActorID: v.ActorID.String(), ClaimID: v.ClaimID.String(), Mode: v.Mode, Model: v.Model, State: v.State, Error: v.ErrorCode, Version: v.Version, Attempts: v.Attempts, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time}
	raw, e := r.cipher.Open(v.InputCipher, v.InputHash)
	if e != nil {
		return out, e
	}
	if e = json.Unmarshal(raw, &out.Input); e != nil {
		return out, e
	}
	if len(v.ResultCipher) > 0 {
		raw, e = r.cipher.Open(v.ResultCipher, v.ResultHash.String)
		if e != nil {
			return out, e
		}
		e = json.Unmarshal(raw, &out.Result)
	}
	return out, e
}
func (r *tscRepository) FindOperation(ctx context.Context, draft, actor, key string) (*domain.TSCJob, error) {
	v, e := r.q.FindTSCOperation(ctx, db.FindTSCOperationParams{DraftID: uuid(draft), ActorID: uuid(actor), OperationKey: uuid(key)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := r.decode(v)
	return &out, e
}
func (r *tscRepository) event(ctx context.Context, j domain.TSCJob, action, reason, actor string, cipher []byte, hash string) error {
	id := Values{}.ID()
	if e := r.q.InsertTSCEvent(ctx, db.InsertTSCEventParams{ID: uuid(id), JobID: uuid(j.ID), Action: action, Version: j.Version, Reason: reason, ActorID: uuid(actor), ResultCipher: cipher, ResultHash: pgtype.Text{String: hash, Valid: hash != ""}}); e != nil {
		return e
	}
	entityType, entityID, version := "tsc_draft", j.ID, j.Version
	if action == "export_requested" {
		// A download request is a distinct audited action, not another mutation of
		// the same snapshot version. Its source row retains the parent job link.
		entityType, entityID, version = "tsc_export_request", id, 1
	}
	if strings.HasPrefix(action, "cloud_") {
		entityType, entityID, version = "tsc_cloud_request", id, 1
	}
	return r.journal.Record(ctx, events.Event{ID: id, CorrelationID: id, Module: "tender_fms", EntityType: entityType, EntityID: entityID, EntityVersion: version, Action: "tsc_" + action, ActorID: actor, ActorKind: "user", ScopeKind: "platform", Visibility: "restricted", OccurredAt: time.Now().UTC()}, nil)
}
func (r *tscRepository) Create(ctx context.Context, d domain.DocumentDraft, actor, key, mode, model string, in domain.TSCInput) (domain.TSCJob, error) {
	n, e := r.q.CountTSCJobs(ctx, uuid(d.ID))
	if e != nil {
		return domain.TSCJob{}, e
	}
	if n >= 50 {
		return domain.TSCJob{}, domain.Invalid("This draft has reached its 50-generation retention limit")
	}
	raw, e := json.Marshal(in)
	if e != nil {
		return domain.TSCJob{}, e
	}
	cipher, hash, e := r.cipher.Seal(raw)
	if e != nil {
		return domain.TSCJob{}, e
	}
	v, e := r.q.CreateTSCJob(ctx, db.CreateTSCJobParams{ID: uuid(Values{}.ID()), DraftID: uuid(d.ID), DraftVersion: d.Version, ActorID: uuid(actor), OperationKey: uuid(key), Mode: mode, Model: model, InputCipher: cipher, InputHash: hash})
	if e != nil {
		return domain.TSCJob{}, tscDBError(e)
	}
	out, e := r.decode(v)
	if e != nil {
		return out, e
	}
	return out, r.event(ctx, out, "queued", "", actor, nil, "")
}
func (r *tscRepository) Get(ctx context.Context, draft, id string) (domain.TSCJob, error) {
	v, e := r.q.GetTSCJob(ctx, db.GetTSCJobParams{DraftID: uuid(draft), ID: uuid(id)})
	if e != nil {
		return domain.TSCJob{}, tscDBError(e)
	}
	return r.decode(v)
}
func (r *tscRepository) List(ctx context.Context, draft string) ([]domain.TSCJob, error) {
	rows, e := r.q.ListTSCJobs(ctx, uuid(draft))
	out := []domain.TSCJob{}
	for _, v := range rows {
		out = append(out, domain.TSCJob{ID: v.ID.String(), DraftID: draft, DraftVersion: v.DraftVersion, Mode: v.Mode, Model: v.Model, State: v.State, Version: v.Version, Attempts: v.Attempts, Error: v.ErrorCode, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time})
	}
	return out, e
}
func (r *tscRepository) Edit(ctx context.Context, j domain.TSCJob, version int64, a domain.TSCAnswer, reason, actor string) (domain.TSCJob, error) {
	action := "answer_corrected"
	for i := range j.Result.Answers {
		if j.Result.Answers[i].QuestionID == a.QuestionID {
			if a.Accepted {
				action = "answer_accepted"
			} else if j.Result.Answers[i].Accepted {
				action = "answer_reopened"
			}
			j.Result.Answers[i] = a
		}
	}
	raw, e := json.Marshal(j.Result)
	if e != nil {
		return j, e
	}
	cipher, hash, e := r.cipher.Seal(raw)
	if e != nil {
		return j, e
	}
	v, e := r.q.EditTSCJob(ctx, db.EditTSCJobParams{ID: uuid(j.ID), ExpectedVersion: version, ResultCipher: cipher, ResultHash: pgtype.Text{String: hash, Valid: true}})
	if e != nil {
		return j, tscDBError(e)
	}
	out, e := r.decode(v)
	if e != nil {
		return out, e
	}
	return out, r.event(ctx, out, action, reason, actor, cipher, hash)
}
func (r *tscRepository) Claim(ctx context.Context) (*domain.TSCJob, error) {
	expired, e := r.q.ExpireTSCJobs(ctx)
	if e != nil {
		return nil, e
	}
	for _, v := range expired {
		j, e := r.decode(v)
		if e != nil {
			return nil, e
		}
		if e = r.event(ctx, j, "failed", "attempts_exhausted", j.ActorID, nil, ""); e != nil {
			return nil, e
		}
	}
	v, e := r.q.ClaimTSCJob(ctx, uuid(Values{}.ID()))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	j, e := r.decode(v)
	if e != nil {
		return nil, e
	}
	if e = r.event(ctx, j, "claimed", "", j.ActorID, nil, ""); e != nil {
		return nil, e
	}
	return &j, nil
}
func (r *tscRepository) ExportRequested(ctx context.Context, j domain.TSCJob, actor string) error {
	return r.event(ctx, j, "export_requested", "", actor, nil, "")
}
func (r *tscRepository) Renew(ctx context.Context, j domain.TSCJob) error {
	n, e := r.q.RenewTSCJob(ctx, db.RenewTSCJobParams{ID: uuid(j.ID), ClaimID: uuid(j.ClaimID)})
	if e != nil {
		return e
	}
	if n != 1 {
		return domain.CaptureFailure(409, "claim_expired", "TSC generation claim expired")
	}
	return nil
}
func (r *tscRepository) Finish(ctx context.Context, j domain.TSCJob, result domain.TSCResult, code string) error {
	state := "ready"
	var cipher []byte
	hash := ""
	if code != "" {
		state = "failed"
	} else {
		raw, e := json.Marshal(result)
		if e != nil {
			return e
		}
		cipher, hash, e = r.cipher.Seal(raw)
		if e != nil {
			return e
		}
	}
	v, e := r.q.FinishTSCJob(ctx, db.FinishTSCJobParams{ID: uuid(j.ID), ClaimID: uuid(j.ClaimID), State: state, ErrorCode: code, ResultCipher: cipher, ResultHash: pgtype.Text{String: hash, Valid: hash != ""}})
	if e != nil {
		return tscDBError(e)
	}
	out, e := r.decode(v)
	if e != nil {
		return e
	}
	return r.event(ctx, out, state, code, j.ActorID, cipher, hash)
}

// PostgreSQL diagnostics never reach response bodies.
func tscDBError(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.CaptureFailure(409, "tsc_state_changed", "TSC state changed or the requested draft is unavailable")
	}
	var pg *pgconn.PgError
	if errors.As(e, &pg) && pg.Code == "23505" {
		return domain.CaptureFailure(409, "tsc_pending", "A generation is already pending; use its current job")
	}
	return e
}
