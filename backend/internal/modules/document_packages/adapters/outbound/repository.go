package outbound

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/identity"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/ports"
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
func (Values) Fingerprint(v any) (string, error) {
	b, e := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), e
}
func uuid(s string) pgtype.UUID { var v pgtype.UUID; _ = v.Scan(s); return v }
func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
func id(v pgtype.UUID) string {
	if !v.Valid {
		return ""
	}
	return v.String()
}

type Repository struct {
	c     database.Connection
	q     *db.Queries
	users auth.AssignmentDirectory
}

func NewRepository(c database.Connection, u auth.AssignmentDirectory) *Repository {
	return &Repository{c, db.New(c), u}
}
func (r *Repository) Guard(ctx context.Context) error {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.Fail(500, "transaction_binding_required", "Shared transaction required")
	}
	return r.q.IdentityGuard(ctx)
}
func (r *Repository) Actor(ctx context.Context, key string) (*domain.Actor, error) {
	v, e := r.users.Get(ctx, key)
	if e != nil || v == nil {
		return nil, e
	}
	return &domain.Actor{Principal: identity.Principal{ID: v.ID, Active: v.Active, Superuser: v.Superuser, Permissions: v.Permissions}, Ready: v.Ready}, nil
}
func (r *Repository) LockOperation(ctx context.Context, o domain.Operation) error {
	return r.q.OperationLock(ctx, o.Actor+":"+o.Command+":"+o.Context+":"+o.Key)
}
func (r *Repository) Operation(ctx context.Context, o domain.Operation) (*domain.Operation, error) {
	v, e := r.q.FindOperation(ctx, db.FindOperationParams{ActorID: uuid(o.Actor), Command: o.Command, ContextKey: o.Context, IdempotencyKey: uuid(o.Key)})
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &domain.Operation{Actor: id(v.ActorID), Command: v.Command, Context: v.ContextKey, Key: id(v.IdempotencyKey), Fingerprint: v.Fingerprint, ReceiptID: id(v.ReceiptID)}, nil
}
func (r *Repository) SaveOperation(ctx context.Context, o domain.Operation) error {
	return r.q.SaveOperation(ctx, db.SaveOperationParams{ActorID: uuid(o.Actor), Command: o.Command, ContextKey: o.Context, IdempotencyKey: uuid(o.Key), Fingerprint: o.Fingerprint, ReceiptID: uuid(o.ReceiptID)})
}
func (r *Repository) event(ctx context.Context, receipt, file, action, actor string, version int64, code string) error {
	return r.q.AddEvent(ctx, db.AddEventParams{ID: uuid(Values{}.ID()), ReceiptID: uuid(receipt), FileID: uuid(file), Action: action, Version: version, ErrorCode: text(code), ActorID: uuid(actor)})
}
func (r *Repository) Create(ctx context.Context, key, actor string, files []domain.Declaration) error {
	if e := r.q.CreateReceipt(ctx, db.CreateReceiptParams{ID: uuid(key), ActorID: uuid(actor)}); e != nil {
		return e
	}
	for _, f := range files {
		if e := r.q.CreateFile(ctx, db.CreateFileParams{ID: uuid(Values{}.ID()), ReceiptID: uuid(key), ClientFileKey: uuid(f.ClientKey), Filename: f.Filename, Kind: f.Kind, DeclaredSize: f.Size, DeclaredSha256: f.SHA256, ActorID: uuid(actor)}); e != nil {
			return e
		}
	}
	return r.event(ctx, key, "", "created", actor, 1, "")
}
func file(v db.DocumentPackageFile) domain.File {
	return domain.File{ID: id(v.ID), ReceiptID: id(v.ReceiptID), ClientKey: id(v.ClientFileKey), Filename: v.Filename, Kind: v.Kind, Size: v.DeclaredSize, SHA256: v.DeclaredSha256, Version: v.Version, UploadVersion: v.UploadInputVersion.Int64, State: v.State, ObjectKey: id(v.ObjectKey), UploadKey: id(v.UploadKey), ClaimID: id(v.ClaimID), LeaseUntil: v.LeaseUntil.Time, ErrorCode: v.ErrorCode.String}
}
func (r *Repository) Get(ctx context.Context, key string) (domain.Receipt, error) {
	v, e := r.q.GetReceipt(ctx, uuid(key))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Receipt{}, domain.Missing
	}
	if e != nil {
		return domain.Receipt{}, e
	}
	out := domain.Receipt{ID: id(v.ID), OwnerID: id(v.OwnerID), State: v.State, Version: v.Version, LimitRevision: v.LimitProfileRevision, ExpiresAt: v.ExpiresAt.Time, CreatedAt: v.CreatedAt.Time, ClaimID: id(v.ClaimID), LeaseUntil: v.LeaseUntil.Time, Attempts: v.Attempts, ErrorCode: v.ErrorCode.String, ConsumedBy: id(v.ConsumedByTenderID), Files: []domain.File{}}
	if v.State != "consumed" && !time.Now().Before(out.ExpiresAt) {
		out.State = "expired"
		out.ErrorCode = "receipt_expired"
	}
	rows, e := r.q.ListFiles(ctx, v.ID)
	if e != nil {
		return out, e
	}
	for _, v := range rows {
		out.Files = append(out.Files, file(v))
	}
	totals, e := r.q.MemberTotals(ctx, db.MemberTotalsParams{ReceiptID: v.ID, ClaimID: v.ClaimID})
	if e != nil {
		return out, e
	}
	out.MemberCount = totals.Members
	if out.State != "ready" && out.State != "consumed" {
		out.Unresolved = int64(len(out.Files))
	}
	return out, nil
}
func (r *Repository) File(ctx context.Context, receipt, key string) (domain.File, error) {
	v, e := r.q.GetFile(ctx, db.GetFileParams{ID: uuid(key), ReceiptID: uuid(receipt)})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.File{}, domain.Missing
	}
	return file(v), e
}
func (r *Repository) BeginUpload(ctx context.Context, f domain.File, lease, key string) error {
	receipt, e := r.Get(ctx, f.ReceiptID)
	if e != nil {
		return e
	}
	n, e := r.q.ActiveUploads(ctx, uuid(receipt.OwnerID))
	if e != nil {
		return e
	}
	if n >= 2 {
		return domain.Fail(429, "rate_limited", "At most two uploads may be active")
	}
	n, e = r.q.StartUpload(ctx, db.StartUploadParams{ID: uuid(f.ID), Version: f.Version, ClaimID: uuid(lease), UploadKey: uuid(key), ActorID: uuid(receipt.OwnerID)})
	if e != nil {
		return e
	}
	if n != 1 {
		return domain.Stale
	}
	if e = r.q.RecordUploadObject(ctx, db.RecordUploadObjectParams{ID: uuid(lease), ReceiptID: uuid(f.ReceiptID), FileID: uuid(f.ID)}); e != nil {
		return e
	}
	return r.event(ctx, f.ReceiptID, f.ID, "upload_claimed", receipt.OwnerID, f.Version+1, "")
}
func (r *Repository) FinishUpload(ctx context.Context, f domain.File, lease, key string) error {
	receipt, e := r.Get(ctx, f.ReceiptID)
	if e != nil {
		return e
	}
	n, e := r.q.CompleteUpload(ctx, db.CompleteUploadParams{ID: uuid(f.ID), ClaimID: uuid(lease), UploadKey: uuid(key), ActorID: uuid(receipt.OwnerID)})
	if e != nil {
		return e
	}
	if n != 1 {
		return domain.Stale
	}
	if e = r.q.TouchReceipt(ctx, db.TouchReceiptParams{ID: uuid(f.ReceiptID), ActorID: uuid(receipt.OwnerID)}); e != nil {
		return e
	}
	return r.event(ctx, f.ReceiptID, f.ID, "uploaded", receipt.OwnerID, f.Version+2, "")
}
func (r *Repository) Seal(ctx context.Context, v domain.Receipt) error {
	n, e := r.q.SealReceipt(ctx, db.SealReceiptParams{ID: uuid(v.ID), Version: v.Version, ActorID: uuid(v.OwnerID)})
	if e != nil {
		return e
	}
	if n != 1 {
		return domain.Stale
	}
	return r.event(ctx, v.ID, "", "verification_queued", v.OwnerID, v.Version+1, "")
}
func (r *Repository) Claim(ctx context.Context, key string) (*domain.Receipt, error) {
	expired, e := r.q.FailExhaustedVerification(ctx)
	if e != nil {
		return nil, e
	}
	for _, v := range expired {
		if e = r.event(ctx, id(v.ID), "", "verification_exhausted", id(v.OwnerID), v.Version, "verification_attempts_exhausted"); e != nil {
			return nil, e
		}
	}
	v, e := r.q.ClaimReceipt(ctx, uuid(key))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	out, e := r.Get(ctx, id(v.ID))
	return &out, e
}
func (r *Repository) AddMember(ctx context.Context, v domain.Receipt, m domain.Member) error {
	current, e := r.Get(ctx, v.ID)
	if e != nil {
		return e
	}
	if current.State != "verifying" || current.ClaimID != v.ClaimID || !time.Now().Before(current.LeaseUntil) {
		return domain.Stale
	}
	total, e := r.q.MemberTotals(ctx, db.MemberTotalsParams{ReceiptID: uuid(v.ID), ClaimID: uuid(v.ClaimID)})
	if e != nil {
		return e
	}
	if total.Members >= 100 || total.Bytes+m.ExpandedSize > domain.PackageBytes || total.Pages+int64(m.Pages) > 3000 || total.Sheets+int64(m.Sheets) > 200 || total.Cells+m.Cells > 250000 {
		return domain.Fail(413, "payload_too_large", "Package exceeds cumulative validation limits")
	}
	return r.q.AddMember(ctx, db.AddMemberParams{ID: uuid(m.ID), ReceiptID: uuid(v.ID), FileID: uuid(m.FileID), Path: m.Path, NormalizedPath: m.NormalizedPath, Filename: m.Filename, Kind: m.Kind, ByteSize: m.Size, ExpandedBytes: m.ExpandedSize, Sha256: m.SHA256, ReaderPayload: m.Payload, ReaderPayloadSha256: m.PayloadSHA256, ValidationClaimID: uuid(v.ClaimID), ScannerRevision: m.ScannerRevision, Pages: m.Pages, Sheets: m.Sheets, Cells: m.Cells, ActorID: uuid(v.OwnerID)})
}
func (r *Repository) FinishVerification(ctx context.Context, v domain.Receipt, code string) error {
	state := "ready"
	if code != "" {
		state = "failed"
	}
	total, e := r.q.MemberTotals(ctx, db.MemberTotalsParams{ReceiptID: uuid(v.ID), ClaimID: uuid(v.ClaimID)})
	if e != nil {
		return e
	}
	if total.Members == 0 && code == "" {
		state = "failed"
		code = "empty_package"
	}
	n, e := r.q.FinishVerification(ctx, db.FinishVerificationParams{ID: uuid(v.ID), ClaimID: uuid(v.ClaimID), State: state, ErrorCode: text(code)})
	if e != nil {
		return e
	}
	if n != 1 {
		return domain.Stale
	}
	if state == "ready" {
		if e = r.q.ReadyFiles(ctx, db.ReadyFilesParams{ReceiptID: uuid(v.ID), ActorID: uuid(v.OwnerID)}); e != nil {
			return e
		}
	}
	return r.event(ctx, v.ID, "", state, v.OwnerID, v.Version+1, code)
}
func (r *Repository) Consume(ctx context.Context, v domain.Receipt, actor, tender string) error {
	n, e := r.q.ConsumeReceipt(ctx, db.ConsumeReceiptParams{ID: uuid(v.ID), Version: v.Version, TenderID: uuid(tender), ActorID: uuid(actor)})
	if e != nil {
		return e
	}
	if n != 1 {
		return domain.Stale
	}
	return r.event(ctx, v.ID, "", "consumed", actor, v.Version+1, "")
}
func (r *Repository) MemberIDs(ctx context.Context, v domain.Receipt) ([]string, error) {
	rows, e := r.q.MemberIDs(ctx, db.MemberIDsParams{ReceiptID: uuid(v.ID), ClaimID: uuid(v.ClaimID), TenderID: uuid(v.ConsumedBy)})
	out := []string{}
	for _, id := range rows {
		out = append(out, id.String())
	}
	return out, e
}
func (r *Repository) Member(ctx context.Context, v domain.Receipt, key string) (domain.Member, error) {
	m, e := r.q.GetMember(ctx, db.GetMemberParams{ReceiptID: uuid(v.ID), ClaimID: uuid(v.ClaimID), TenderID: uuid(v.ConsumedBy), ID: uuid(key)})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Member{}, domain.Missing
	}
	if e != nil {
		return domain.Member{}, e
	}
	return domain.Member{ID: id(m.ID), ReceiptID: id(m.ReceiptID), FileID: id(m.FileID), Path: m.Path, NormalizedPath: m.NormalizedPath, Filename: m.Filename, Kind: m.Kind, Size: m.ByteSize, ExpandedSize: m.ExpandedBytes, SHA256: m.Sha256, Payload: m.ReaderPayload, PayloadSHA256: m.ReaderPayloadSha256, ClaimID: id(m.ValidationClaimID), ScannerRevision: m.ScannerRevision, Pages: m.Pages, Sheets: m.Sheets, Cells: m.Cells}, nil
}

type Unit struct {
	m     *database.Manager
	users func(database.Connection) auth.AssignmentDirectory
}

func NewUnit(b database.Beginner, u func(database.Connection) auth.AssignmentDirectory) *Unit {
	return &Unit{database.NewManager(b), u}
}
func (u *Unit) Within(ctx context.Context, f func(context.Context, ports.Repository) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error { return f(ctx, NewRepository(c, u.users(c))) })
}

type Auth struct{ Resolver auth.Resolver }

func (a Auth) Subject(ctx context.Context, t string) (string, error) {
	v, e := a.Resolver.Resolve(ctx, t)
	if e != nil {
		return "", e
	}
	if v.AccountNotReady {
		return "", domain.Fail(403, "account_not_ready", "Complete account setup")
	}
	if !v.Active || v.ID == "" {
		return "", domain.Unauthorized
	}
	return v.ID, nil
}

func (r *Repository) AbortUpload(ctx context.Context, f domain.File, claim string) error {
	receipt, e := r.Get(ctx, f.ReceiptID)
	if e != nil {
		return e
	}
	n, e := r.q.AbortUpload(ctx, db.AbortUploadParams{ID: uuid(f.ID), ClaimID: uuid(claim), ActorID: uuid(receipt.OwnerID)})
	if e != nil {
		return e
	}
	if n == 0 {
		return nil
	}
	return r.event(ctx, f.ReceiptID, f.ID, "upload_failed", receipt.OwnerID, f.Version+2, "upload_failed")
}

func (r *Repository) PurgeCandidates(ctx context.Context, claim string) ([]domain.ObjectRef, error) {
	rows, e := r.q.ClaimPurgeObjects(ctx, uuid(claim))
	if e != nil {
		return nil, e
	}
	out := []domain.ObjectRef{}
	for _, v := range rows {
		n, e := r.q.ExpireReceipt(ctx, v.ReceiptID)
		if e != nil {
			return nil, e
		}
		if n > 0 {
			receipt, e := r.Get(ctx, id(v.ReceiptID))
			if e != nil {
				return nil, e
			}
			if e = r.event(ctx, receipt.ID, "", "expired", receipt.OwnerID, receipt.Version, "receipt_expired"); e != nil {
				return nil, e
			}
		}
		out = append(out, domain.ObjectRef{ID: id(v.ID), ReceiptID: id(v.ReceiptID), FileID: id(v.FileID), ClaimID: claim})
	}
	return out, nil
}
func (r *Repository) FinishPurge(ctx context.Context, o domain.ObjectRef) error {
	n, e := r.q.FinishPurgeObject(ctx, db.FinishPurgeObjectParams{ID: uuid(o.ID), ClaimID: uuid(o.ClaimID)})
	if e != nil {
		return e
	}
	if n != 1 {
		return domain.Stale
	}
	v, e := r.Get(ctx, o.ReceiptID)
	if e != nil {
		return e
	}
	return r.event(ctx, v.ID, o.FileID, "unconsumed_object_purged", v.OwnerID, v.Version, "")
}
