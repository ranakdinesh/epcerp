package outbound

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	notices "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	packages "github.com/ranakdinesh/epcERP/internal/modules/document_packages/contracts"
	reading "github.com/ranakdinesh/epcERP/internal/modules/document_reading/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

func (Values) Fingerprint(v any) (string, error) {
	b, e := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), e
}
func optional(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

type CaptureRepository struct {
	c database.Connection
	q *db.Queries
	j events.Writer
}

func NewCaptureRepository(c database.Connection, j events.Writer) *CaptureRepository {
	return &CaptureRepository{c, db.New(c), j}
}
func (r *CaptureRepository) Guard(ctx context.Context) error {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.CaptureFailure(500, "transaction_binding_required", "Shared transaction required")
	}
	return r.q.LockTenderIdentity(ctx)
}
func (r *CaptureRepository) LockOperation(ctx context.Context, o domain.CaptureOperation) error {
	return r.q.CaptureOperationLock(ctx, o.Actor+":"+o.Command+":"+o.Context+":"+o.Key)
}
func (r *CaptureRepository) Operation(ctx context.Context, o domain.CaptureOperation) (*domain.CaptureOperation, error) {
	v, e := r.q.FindCaptureOperation(ctx, db.FindCaptureOperationParams{ActorID: uuid(o.Actor), Command: o.Command, ContextKey: o.Context, IdempotencyKey: uuid(o.Key)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &domain.CaptureOperation{Actor: v.ActorID.String(), Command: v.Command, Context: v.ContextKey, Key: v.IdempotencyKey.String(), Fingerprint: v.Fingerprint, TenderID: v.TenderID.String()}, nil
}
func (r *CaptureRepository) SaveOperation(ctx context.Context, o domain.CaptureOperation) error {
	return r.q.SaveCaptureOperation(ctx, db.SaveCaptureOperationParams{ActorID: uuid(o.Actor), Command: o.Command, ContextKey: o.Context, IdempotencyKey: uuid(o.Key), Fingerprint: o.Fingerprint, TenderID: uuid(o.TenderID)})
}
func (r *CaptureRepository) Bind(ctx context.Context, v domain.Record, in domain.CaptureInput, actor string) error {
	if e := r.q.BindCapture(ctx, db.BindCaptureParams{ID: uuid(v.ID), ReceiptID: uuid(in.ReceiptID), ExternalReference: optional(in.ExternalReference), SourceUrl: optional(in.SourceURL), ActorID: uuid(actor)}); e != nil {
		return e
	}
	return r.q.CreateProvisionOperation(ctx, db.CreateProvisionOperationParams{ID: uuid(Values{}.ID()), TenderID: uuid(v.ID), ActorID: uuid(actor)})
}
func (r *CaptureRepository) LinkSource(ctx context.Context, tender, member, document, actor string) error {
	return r.q.LinkTenderSource(ctx, db.LinkTenderSourceParams{TenderID: uuid(tender), MemberID: uuid(member), DocumentID: uuid(document), ActorID: uuid(actor)})
}
func (r *CaptureRepository) Projection(ctx context.Context, record domain.Record) (domain.Captured, error) {
	out := domain.Captured{Record: record, Deliveries: []domain.Delivery{}}
	v, e := r.q.CaptureProvision(ctx, uuid(record.ID))
	if errors.Is(e, pgx.ErrNoRows) {
		return out, domain.Missing
	}
	if e != nil {
		return out, e
	}
	out.ProvisionID = v.ID.String()
	out.ProvisionState = v.State
	out.ProvisionVersion = v.Version
	out.Blocker = v.BlockerCode
	out.ProjectID = v.ProjectID.String()
	out.EstimateID = v.EstimateID.String()
	rows, e := r.q.CaptureDeliveries(ctx, uuid(record.ID))
	if e != nil {
		return out, e
	}
	for _, v := range rows {
		out.Deliveries = append(out.Deliveries, domain.Delivery{ActionID: v.ID.String(), NoticeID: v.NoticeID.String(), EmailJobID: v.EmailJobID.String(), EmailState: v.EmailState})
	}
	return out, nil
}
func (r *CaptureRepository) ManagerWork(ctx context.Context, record domain.Record, actor string) (domain.AssignmentAction, error) {
	var out domain.AssignmentAction
	event := Values{}.ID()
	id := Values{}.ID()
	v, e := r.q.CreateManagerAction(ctx, db.CreateManagerActionParams{ID: uuid(id), TenderID: uuid(record.ID), AssignmentID: uuid(record.ManagerAssignmentID), RecipientID: uuid(record.ManagerID), SourceEventID: uuid(event), AssignmentVersion: record.AssignmentVersion, ActorID: uuid(actor)})
	// The event FK is deferred by migration so source action and journal can share
	// the exact database timestamp without a pre-transaction wall-clock guess.
	if e != nil {
		return out, e
	}
	if e = r.j.Record(ctx, events.Event{ID: event, CorrelationID: event, Module: "tender_fms", EntityType: "tender_assignment", EntityID: record.ManagerAssignmentID, EntityVersion: record.AssignmentVersion, Action: "manager_assigned", ActorID: actor, ActorKind: "user", ScopeKind: "platform", Visibility: "restricted", OccurredAt: v.CreatedAt.Time}, nil); e != nil {
		return out, e
	}
	if e = r.q.CreateManagerEmail(ctx, db.CreateManagerEmailParams{ID: uuid(Values{}.ID()), ActionID: v.ID, RecipientID: uuid(record.ManagerID), ActorID: uuid(actor)}); e != nil {
		return out, e
	}
	return domain.AssignmentAction{ID: id, TenderID: record.ID, AssignmentID: record.ManagerAssignmentID, RecipientID: record.ManagerID, EventID: event, ActorID: actor, AssignmentVersion: record.AssignmentVersion, At: v.CreatedAt.Time}, nil
}
func (r *CaptureRepository) LinkNotice(ctx context.Context, action, notice string) error {
	return r.q.LinkManagerNotice(ctx, db.LinkManagerNoticeParams{ID: uuid(action), NoticeID: uuid(notice)})
}
func (r *CaptureRepository) ChangeManager(ctx context.Context, record domain.Record, in domain.ChangeManager, actor string) (domain.Record, error) {
	assignment := Values{}.ID()
	if e := r.q.InsertManagerAssignment(ctx, db.InsertManagerAssignmentParams{ID: uuid(assignment), IntakeID: uuid(record.ID), ManagerID: uuid(in.ManagerID), Version: record.AssignmentVersion + 1, Reason: in.Reason, ActorID: uuid(actor)}); e != nil {
		return domain.Record{}, e
	}
	v, e := r.q.ReplaceNamedManager(ctx, db.ReplaceNamedManagerParams{ID: uuid(record.ID), ManagerID: uuid(in.ManagerID), AssignmentID: uuid(assignment), ActorID: uuid(actor), ExpectedVersion: in.Version, ExpectedAssignmentVersion: in.AssignmentVersion})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Record{}, domain.CaptureFailure(409, "stale_version", "Tender assignment changed")
	}
	if e != nil {
		return domain.Record{}, e
	}
	if e = r.q.CancelOldManagerMail(ctx, db.CancelOldManagerMailParams{TenderID: uuid(record.ID), ActorID: uuid(actor)}); e != nil {
		return domain.Record{}, e
	}
	if e = r.q.CancelOldManagerWork(ctx, db.CancelOldManagerWorkParams{TenderID: uuid(record.ID), ActorID: uuid(actor)}); e != nil {
		return domain.Record{}, e
	}
	if e = r.recordChange(ctx, v, actor, "manager_assigned"); e != nil {
		return domain.Record{}, e
	}
	return namedRecord(v), nil
}
func (r *CaptureRepository) recordChange(ctx context.Context, v db.TenderFmsIntake, actor, action string) error {
	event := Values{}.ID()
	if e := r.q.RecordNamedChange(ctx, db.RecordNamedChangeParams{ID: uuid(event), IntakeID: v.ID, BusinessVersion: v.BusinessVersion, Action: action, ActorID: uuid(actor)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: event, CorrelationID: event, Module: "tender_fms", EntityType: "tender_record", EntityID: v.ID.String(), EntityVersion: v.BusinessVersion, Action: action, ActorID: actor, ActorKind: "user", ScopeKind: "platform", Visibility: "restricted", OccurredAt: v.UpdatedAt.Time}, nil)
}
func (r *CaptureRepository) Patch(ctx context.Context, record domain.Record, in domain.Patch, actor string) (domain.Record, error) {
	v, e := r.q.PatchNamedTender(ctx, db.PatchNamedTenderParams{ID: uuid(record.ID), Name: in.Name, SetReference: in.SetReference, SetUrl: in.SetURL, ExternalReference: optional(in.ExternalReference), SourceUrl: optional(in.SourceURL), ActorID: uuid(actor), ExpectedVersion: in.Version})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Record{}, domain.CaptureFailure(409, "stale_version", "Tender changed")
	}
	if e != nil {
		return domain.Record{}, e
	}
	if e = r.recordChange(ctx, v, actor, "metadata_updated"); e != nil {
		return domain.Record{}, e
	}
	return namedRecord(v), nil
}
func (r *CaptureRepository) List(ctx context.Context, actor string, portfolio bool, page, size int32, search, sort string, descending bool) ([]string, int64, error) {
	total, e := r.q.CountNamedCaptures(ctx, db.CountNamedCapturesParams{ActorID: uuid(actor), Portfolio: portfolio, Search: search})
	if e != nil {
		return nil, 0, e
	}
	rows, e := r.q.ListNamedCaptures(ctx, db.ListNamedCapturesParams{ActorID: uuid(actor), Portfolio: portfolio, PageSize: size, Search: search, SortKey: sort, Descending: descending, PageOffset: int64(page-1) * int64(size)})
	out := []string{}
	for _, id := range rows {
		out = append(out, id.String())
	}
	return out, total, e
}
func (r *CaptureRepository) ReadAction(ctx context.Context, id string) (domain.AssignmentAction, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.AssignmentAction{}, domain.CaptureFailure(500, "transaction_binding_required", "Shared transaction required")
	}
	v, e := r.q.ReadAssignmentAction(ctx, uuid(id))
	if e != nil {
		return domain.AssignmentAction{}, e
	}
	if v.State != "pending" || !v.IsActive {
		return domain.AssignmentAction{}, domain.Missing
	}
	return domain.AssignmentAction{ID: v.ID.String(), TenderID: v.TenderID.String(), AssignmentID: v.AssignmentID.String(), RecipientID: v.RecipientID.String(), EventID: v.SourceEventID.String(), ActorID: v.CreatedByID.String(), AssignmentVersion: v.AssignmentVersion, At: v.CreatedAt.Time}, nil
}

type sourceBridge struct {
	packages packages.Consumer
	reader   reading.PreparedReader
}

func (s sourceBridge) Consume(ctx context.Context, actor, id string, version int64, tender string) (domain.PackageRef, error) {
	v, e := s.packages.Consume(ctx, actor, id, version, tender)
	return domain.PackageRef{ID: v.ID, ClaimID: v.ClaimID, OwnerID: v.OwnerID, TenderID: tender}, e
}
func receipt(r domain.PackageRef) packages.Receipt {
	return packages.Receipt{ID: r.ID, ClaimID: r.ClaimID, OwnerID: r.OwnerID, ConsumedBy: r.TenderID}
}
func (s sourceBridge) MemberIDs(ctx context.Context, r domain.PackageRef) ([]string, error) {
	return s.packages.MemberIDs(ctx, receipt(r))
}
func (s sourceBridge) Queue(ctx context.Context, r domain.PackageRef, member, actor, tender string) (string, error) {
	m, e := s.packages.Member(ctx, receipt(r), member)
	if e != nil {
		return "", e
	}
	v, e := s.reader.SubmitPrepared(ctx, reading.Input{Module: "tender_fms", EntityID: tender, ActorID: actor}, reading.Prepared{Filename: m.Filename, Format: m.Kind, SHA256: m.SHA256, CipherSHA256: m.PayloadSHA256, Size: m.Size, Ciphertext: m.Payload})
	return v.ID, e
}

type noticeBridge struct{ writer notices.TenderWriter }

func (n noticeBridge) Deliver(ctx context.Context, a domain.AssignmentAction) (string, error) {
	return n.writer.TenderAssignment(ctx, a.ID)
}

type CaptureUnit struct {
	m        *database.Manager
	users    func(database.Connection) auth.AssignmentDirectory
	bundles  func(database.Connection) auth.BundleDirectory
	approved []string
	journal  func(database.Connection) events.Writer
	records  func(database.Connection) contracts.Records
	packages func(database.Connection) packages.Consumer
	reader   func(database.Connection) reading.PreparedReader
	notices  func(database.Connection) notices.TenderWriter
}

func NewCaptureUnit(b database.Beginner, u func(database.Connection) auth.AssignmentDirectory, bundles func(database.Connection) auth.BundleDirectory, approved []string, j func(database.Connection) events.Writer, records func(database.Connection) contracts.Records, p func(database.Connection) packages.Consumer, r func(database.Connection) reading.PreparedReader, n func(database.Connection) notices.TenderWriter) *CaptureUnit {
	return &CaptureUnit{database.NewManager(b), u, bundles, approved, j, records, p, r, n}
}
func (u *CaptureUnit) Within(ctx context.Context, f func(context.Context, ports.CaptureRepository, ports.RecordFoundation, ports.CaptureSources, ports.EligibleUsers, ports.AssignmentNotices) error) error {
	var callbackError error
	e := u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		callbackError = f(ctx, NewCaptureRepository(c, u.journal(c)), u.records(c), sourceBridge{u.packages(c), u.reader(c)}, NewEligibleUsers(u.users(c), u.bundles(c), u.approved), noticeBridge{u.notices(c)})
		return callbackError
	})
	if e != nil && callbackError == nil {
		return domain.CaptureFailure(503, "operation_outcome_unknown", "Operation outcome is unknown; retry with the same Idempotency-Key")
	}
	return e
}
