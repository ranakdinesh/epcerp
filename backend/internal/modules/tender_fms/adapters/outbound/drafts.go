package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	packages "github.com/ranakdinesh/epcERP/internal/modules/document_packages/contracts"
	reading "github.com/ranakdinesh/epcERP/internal/modules/document_reading/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

type DraftUnit struct {
	m        *database.Manager
	users    func(database.Connection) auth.AssignmentDirectory
	packages func(database.Connection) packages.DraftConsumer
	reader   func(database.Connection) reading.Reader
	prepared func(database.Connection) reading.PreparedReader
}

func NewDraftUnit(b database.Beginner, u func(database.Connection) auth.AssignmentDirectory, p func(database.Connection) packages.DraftConsumer, r func(database.Connection) reading.Reader, pr func(database.Connection) reading.PreparedReader) *DraftUnit {
	return &DraftUnit{database.NewManager(b), u, p, r, pr}
}
func (u *DraftUnit) Within(ctx context.Context, f func(context.Context, ports.DraftRepository) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return f(ctx, &DraftRepository{c, db.New(c), u.users(c), u.packages(c), u.reader(c), u.prepared(c)})
	})
}

type DraftRepository struct {
	c     database.Connection
	q     *db.Queries
	users auth.AssignmentDirectory
	p     packages.DraftConsumer
	r     reading.Reader
	pr    reading.PreparedReader
}

func (r *DraftRepository) Authorize(ctx context.Context, actor string, write bool) error {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.Forbidden
	}
	if e := r.q.LockTenderIdentity(ctx); e != nil {
		return e
	}
	u, e := r.users.Get(ctx, actor)
	if e != nil {
		return e
	}
	if u == nil || !u.Active {
		return domain.Unauthorized
	}
	if !u.Ready {
		return domain.Forbidden
	}
	codes := []string{"tenders.register.view", "tenders.register.read", "tenders.processing.read"}
	if write {
		codes = append(codes, "tenders.register.write", "tenders.processing.write")
	}
	p := domain.Principal{ID: u.ID, Active: u.Active, Superuser: u.Superuser, Permissions: u.Permissions}
	for _, code := range codes {
		if !p.Allows(code) {
			return domain.Forbidden
		}
	}
	return nil
}
func draft(v db.TenderDocumentDraft) domain.DocumentDraft {
	return domain.DocumentDraft{ID: v.ID.String(), Name: v.Name, OwnerID: v.OwnerID.String(), Version: v.Version, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, Batches: []domain.DraftBatch{}}
}
func (r *DraftRepository) Create(ctx context.Context, actor, key, name string) (domain.DocumentDraft, error) {
	v, e := r.q.CreateDocumentDraft(ctx, db.CreateDocumentDraftParams{ID: uuid(Values{}.ID()), Name: name, ActorID: uuid(actor), OperationKey: uuid(key)})
	if e != nil {
		return domain.DocumentDraft{}, e
	}
	if v.Name != name {
		return domain.DocumentDraft{}, domain.CaptureFailure(409, "idempotency_conflict", "Operation key was used with another name")
	}
	// Exactly one creation event even when a request is replayed.
	_, e = r.c.Exec(ctx, `INSERT INTO tender.draft_events(id,draft_id,action,created_by_id) SELECT $1,$2,'created',$3 WHERE NOT EXISTS(SELECT 1 FROM tender.draft_events WHERE draft_id=$2 AND action='created')`, uuid(Values{}.ID()), v.ID, uuid(actor))
	return draft(v), e
}
func (r *DraftRepository) Get(ctx context.Context, id, actor string) (domain.DocumentDraft, error) {
	v, e := r.q.GetDocumentDraft(ctx, db.GetDocumentDraftParams{ID: uuid(id), ActorID: uuid(actor)})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.DocumentDraft{}, domain.Missing
	}
	return draft(v), e
}
func (r *DraftRepository) List(ctx context.Context, actor string, q domain.DraftQuery) ([]domain.DocumentDraft, int64, error) {
	rows, e := r.q.ListDocumentDrafts(ctx, db.ListDocumentDraftsParams{ActorID: uuid(actor), PageSize: q.Limit, PageOffset: q.Offset, Search: q.Search, SortName: q.SortName, Ascending: q.Ascending})
	if e != nil {
		return nil, 0, e
	}
	out := []domain.DocumentDraft{}
	for _, v := range rows {
		out = append(out, draft(v))
	}
	n, e := r.q.CountDocumentDrafts(ctx, db.CountDocumentDraftsParams{ActorID: uuid(actor), Search: q.Search})
	return out, n, e
}
func (r *DraftRepository) Attach(ctx context.Context, d domain.DocumentDraft, receipt, actor string) error {
	old, e := r.p.Receipts(ctx, actor, d.ID)
	if e != nil {
		return e
	}
	for _, v := range old {
		if v.ID == receipt {
			return nil
		}
	}
	if len(old) >= 100 {
		return domain.Invalid("Draft exceeds 100 upload batches; create a separate draft")
	}
	if e = r.p.Retain(ctx, actor, receipt, d.ID); e != nil {
		return e
	}
	if e = r.q.TouchDocumentDraft(ctx, db.TouchDocumentDraftParams{ID: uuid(d.ID), ActorID: uuid(actor)}); e != nil {
		return e
	}
	return r.q.DraftEvent(ctx, db.DraftEventParams{ID: uuid(Values{}.ID()), DraftID: uuid(d.ID), Action: "source_batch_retained", ReceiptID: uuid(receipt), ActorID: uuid(actor)})
}
func (r *DraftRepository) Populate(ctx context.Context, d *domain.DocumentDraft) error {
	receipts, e := r.p.Receipts(ctx, d.OwnerID, d.ID)
	if e != nil {
		return e
	}
	links, e := r.q.DraftSourceLinks(ctx, uuid(d.ID))
	if e != nil {
		return e
	}
	linked := map[string]string{}
	for _, v := range links {
		linked[v.MemberID.String()] = v.DocumentID.String()
	}
	docs, e := r.r.List(ctx, "tender_draft", d.ID)
	if e != nil {
		return e
	}
	byID := map[string]reading.Document{}
	for _, v := range docs {
		byID[v.ID] = v
	}
	for _, v := range receipts {
		b := domain.DraftBatch{ID: v.ID, Revision: v.ID, State: v.State, Error: v.ErrorCode, Version: v.Version, Attempts: v.Attempts, UpdatedAt: v.UpdatedAt, Files: []domain.DraftFile{}, Sources: []domain.DraftSource{}}
		dispositions, e := r.p.Dispositions(ctx, v)
		if e != nil {
			return e
		}
		b.Manifest = []domain.DraftDisposition{}
		for _, entry := range dispositions {
			b.Manifest = append(b.Manifest, domain.DraftDisposition{Path: entry.Path, State: entry.State, Code: entry.Code})
		}
		for _, f := range v.Files {
			b.Files = append(b.Files, domain.DraftFile{ID: f.ID, Name: f.Filename, State: f.State, Error: f.ErrorCode, SHA256: f.SHA256, Version: f.Version})
		}
		if v.State == "consumed" {
			v.ConsumedBy = d.ID
			ids, e := r.p.MemberIDs(ctx, v)
			if e != nil {
				return e
			}
			for _, id := range ids {
				m, e := r.p.MemberInfo(ctx, v, id)
				if e != nil {
					return e
				}
				doc := byID[linked[id]]
				b.Sources = append(b.Sources, domain.DraftSource{ID: doc.ID, MemberID: id, Path: m.Path, Filename: m.Filename, SHA256: m.SHA256, Status: doc.Status, Error: doc.Error, Version: doc.Version, Attempts: doc.Attempts, Segments: doc.SegmentCount, Warnings: doc.Warnings, UpdatedAt: doc.UpdatedAt, ExpectedPages: m.Pages, ExpectedSheets: m.Sheets})
			}
		}
		d.Batches = append(d.Batches, b)
	}
	return nil
}
func (r *DraftRepository) Publish(ctx context.Context, d domain.DocumentDraft) error {
	if e := r.Authorize(ctx, d.OwnerID, true); e != nil {
		return e
	}
	receipts, e := r.p.Receipts(ctx, d.OwnerID, d.ID)
	if e != nil {
		return e
	}
	for _, v := range receipts {
		if v.State != "ready" {
			continue
		}
		v, e = r.p.Publish(ctx, d.OwnerID, v.ID, d.ID)
		if e != nil {
			return e
		}
		ids, e := r.p.MemberIDs(ctx, v)
		if e != nil {
			return e
		}
		for _, id := range ids {
			m, e := r.p.Member(ctx, v, id)
			if e != nil {
				return e
			}
			doc, e := r.pr.SubmitPrepared(ctx, reading.Input{Module: "tender_draft", EntityID: d.ID, ActorID: d.OwnerID}, reading.Prepared{Filename: m.Filename, Format: m.Kind, SHA256: m.SHA256, CipherSHA256: m.PayloadSHA256, Size: m.Size, Ciphertext: m.Payload})
			if e != nil {
				return e
			}
			if e = r.q.LinkDraftSource(ctx, db.LinkDraftSourceParams{DraftID: uuid(d.ID), MemberID: uuid(id), DocumentID: uuid(doc.ID), ActorID: uuid(d.OwnerID)}); e != nil {
				return e
			}
		}
		if e = r.q.DraftEvent(ctx, db.DraftEventParams{ID: uuid(Values{}.ID()), DraftID: uuid(d.ID), Action: "readings_queued", ReceiptID: uuid(v.ID), ActorID: uuid(d.OwnerID)}); e != nil {
			return e
		}
	}
	return nil
}
func (r *DraftRepository) Artifact(ctx context.Context, d domain.DocumentDraft, doc, kind string) ([]byte, error) {
	if kind == "history" {
		history, e := r.r.History(ctx, "tender_draft", d.ID, doc)
		if e != nil {
			return nil, e
		}
		return json.MarshalIndent(history, "", "  ")
	}
	return r.r.Artifact(ctx, "tender_draft", d.ID, doc, kind)
}
func (r *DraftRepository) Retry(ctx context.Context, d domain.DocumentDraft, doc string, version int32) error {
	_, e := r.r.Retry(ctx, "tender_draft", d.ID, doc, version, d.OwnerID)
	return e
}

func (r *DraftRepository) Pending(ctx context.Context) ([]domain.DocumentDraft, error) {
	rows, e := r.p.ReadyDrafts(ctx)
	out := []domain.DocumentDraft{}
	for _, v := range rows {
		out = append(out, domain.DocumentDraft{ID: v.DraftID, OwnerID: v.OwnerID})
	}
	return out, e
}
