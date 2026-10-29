package application

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/ports"
)

type Service struct {
	u       ports.Unit
	a       ports.Auth
	v       ports.Values
	objects ports.Objects
	limits  ports.RequestLimits
}

func New(u ports.Unit, a ports.Auth, v ports.Values, o ports.Objects) *Service {
	return &Service{u: u, a: a, v: v, objects: o}
}
func Live(ctx context.Context, r ports.Repository, id string, write bool) error {
	if e := r.Guard(ctx); e != nil {
		return e
	}
	a, e := r.Actor(ctx, id)
	if e != nil {
		return e
	}
	if a == nil {
		return domain.Unauthorized
	}
	return a.Authorize(write)
}
func owned(ctx context.Context, r ports.Repository, id, owner string) (domain.Receipt, error) {
	v, e := r.Get(ctx, id)
	if e != nil {
		return v, e
	}
	if v.OwnerID != owner {
		return domain.Receipt{}, domain.Missing
	}
	return v, nil
}
func available(v domain.Receipt) error {
	if v.State == "expired" {
		return domain.Fail(409, "receipt_expired", "Receipt has expired")
	}
	if v.State == "consumed" {
		return domain.Fail(409, "receipt_already_consumed", "Receipt is already attached to a Tender")
	}
	return nil
}
func (s *Service) Create(ctx context.Context, token, key string, files []domain.Declaration) (out domain.Receipt, replay bool, e error) {
	actor, e := s.a.Subject(ctx, token)
	if e == nil {
		e = s.limit(actor, "create")
	}
	if e != nil {
		return out, false, e
	}
	if e = domain.UUID(key); e != nil {
		return
	}
	if e = domain.Validate(files); e != nil {
		return
	}
	fp, e := s.v.Fingerprint(files)
	if e != nil {
		return
	}
	op := domain.Operation{Actor: actor, Command: "create", Context: "tender_capture", Key: key, Fingerprint: fp}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository) error {
		if e := Live(ctx, r, actor, true); e != nil {
			return e
		}
		if e := r.LockOperation(ctx, op); e != nil {
			return e
		}
		prev, e := r.Operation(ctx, op)
		if e != nil {
			return e
		}
		if prev != nil {
			if prev.Fingerprint != fp {
				return domain.Fail(409, "idempotency_conflict", "Operation key was used with different input")
			}
			replay = true
			out, e = owned(ctx, r, prev.ReceiptID, actor)
			return e
		}
		id := s.v.ID()
		if e := r.Create(ctx, id, actor, files); e != nil {
			return e
		}
		op.ReceiptID = id
		if e := r.SaveOperation(ctx, op); e != nil {
			return e
		}
		out, e = owned(ctx, r, id, actor)
		return e
	})
	return
}
func (s *Service) Get(ctx context.Context, token, id string) (out domain.Receipt, e error) {
	actor, e := s.a.Subject(ctx, token)
	if e == nil {
		e = s.limit(actor, "read")
	}
	if e != nil {
		return out, e
	}
	if e = domain.UUID(id); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository) error {
		if e := Live(ctx, r, actor, false); e != nil {
			return e
		}
		out, e = owned(ctx, r, id, actor)
		return e
	})
	return
}
func (s *Service) Preflight(ctx context.Context, token, id, file string) error {
	actor, e := s.a.Subject(ctx, token)
	if e != nil {
		return e
	}
	if e = domain.UUID(id); e != nil {
		return e
	}
	if e = domain.UUID(file); e != nil {
		return e
	}
	return s.u.Within(ctx, func(ctx context.Context, r ports.Repository) error {
		if e := Live(ctx, r, actor, true); e != nil {
			return e
		}
		if e := r.LockOperation(ctx, domain.Operation{Actor: actor, Command: "upload-quota"}); e != nil {
			return e
		}
		v, e := owned(ctx, r, id, actor)
		if e != nil {
			return e
		}
		if e = available(v); e != nil {
			return e
		}
		if v.State != "receiving" && v.State != "verifying" && v.State != "ready" {
			return domain.Stale
		}
		_, e = r.File(ctx, id, file)
		return e
	})
}
func (s *Service) Upload(ctx context.Context, token, id, file, key string, version int64, body domain.ByteStream) (out domain.Receipt, replay bool, e error) {
	actor, e := s.a.Subject(ctx, token)
	if e == nil {
		e = s.limit(actor, "upload")
	}
	if e != nil {
		return out, false, e
	}
	for _, v := range []string{id, file, key} {
		if e = domain.UUID(v); e != nil {
			return
		}
	}
	if version < 1 || version > domain.SafeVersion {
		return out, false, domain.Stale
	}
	var source domain.File
	lease := s.v.ID()
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository) error {
		if e := Live(ctx, r, actor, true); e != nil {
			return e
		}
		if e := r.LockOperation(ctx, domain.Operation{Actor: actor, Command: "upload-quota"}); e != nil {
			return e
		}
		v, e := owned(ctx, r, id, actor)
		if e != nil {
			return e
		}
		if e = available(v); e != nil {
			return e
		}
		source, e = r.File(ctx, id, file)
		if e != nil {
			return e
		}
		if source.UploadKey == key && (source.State == "received" || source.State == "ready") {
			if source.UploadVersion != version {
				return domain.Fail(409, "idempotency_conflict", "Upload key was used with different input")
			}
			replay = true
			out = v
			return nil
		}
		if v.State != "receiving" || source.Version != version {
			return domain.Stale
		}
		if source.State == "received" || source.State == "ready" {
			return domain.Fail(409, "idempotency_conflict", "File is already uploaded")
		}
		return r.BeginUpload(ctx, source, lease, key)
	})
	if e != nil || replay {
		return
	}
	// Streaming/object I/O is outside the short claim/finalize transactions.
	if e = s.objects.Upload(ctx, lease, body, source.Size, source.SHA256); e != nil {
		_ = s.u.Within(ctx, func(ctx context.Context, r ports.Repository) error { return r.AbortUpload(ctx, source, lease) })
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository) error {
		if e := Live(ctx, r, actor, true); e != nil {
			return e
		}
		if e := r.LockOperation(ctx, domain.Operation{Actor: actor, Command: "upload-quota"}); e != nil {
			return e
		}
		v, e := owned(ctx, r, id, actor)
		if e != nil {
			return e
		}
		if e = available(v); e != nil {
			return e
		}
		if e = r.FinishUpload(ctx, source, lease, key); e != nil {
			return e
		}
		out, e = owned(ctx, r, id, actor)
		return e
	})
	return
}
func (s *Service) Seal(ctx context.Context, token, id, key string, version int64) (out domain.Receipt, replay bool, e error) {
	actor, e := s.a.Subject(ctx, token)
	if e == nil {
		e = s.limit(actor, "seal")
	}
	if e != nil {
		return out, false, e
	}
	for _, v := range []string{id, key} {
		if e = domain.UUID(v); e != nil {
			return
		}
	}
	fp, e := s.v.Fingerprint(version)
	if e != nil {
		return
	}
	op := domain.Operation{Actor: actor, Command: "seal", Context: id, Key: key, Fingerprint: fp}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository) error {
		if e := Live(ctx, r, actor, true); e != nil {
			return e
		}
		if e := r.LockOperation(ctx, op); e != nil {
			return e
		}
		out, e = owned(ctx, r, id, actor)
		if e != nil {
			return e
		}
		prev, e := r.Operation(ctx, op)
		if e != nil {
			return e
		}
		if prev != nil {
			if prev.Fingerprint != fp {
				return domain.Fail(409, "idempotency_conflict", "Operation key was used with different input")
			}
			replay = true
			return nil
		}
		if e = available(out); e != nil {
			return e
		}
		if out.Version != version || (out.State != "receiving" && out.State != "failed") || out.Attempts >= 3 {
			return domain.Stale
		}
		for _, f := range out.Files {
			if f.State != "received" && f.State != "ready" {
				return domain.Fail(409, "receipt_not_ready", "Upload every declared file before verification")
			}
		}
		if e = r.Seal(ctx, out); e != nil {
			return e
		}
		op.ReceiptID = id
		if e = r.SaveOperation(ctx, op); e != nil {
			return e
		}
		out, e = owned(ctx, r, id, actor)
		return e
	})
	return
}

type Consumer struct{ r ports.Repository }

func NewConsumer(r ports.Repository) *Consumer { return &Consumer{r} }
func (c *Consumer) Consume(ctx context.Context, actor, id string, version int64, tender string) (out domain.Receipt, e error) {
	if e = Live(ctx, c.r, actor, true); e != nil {
		return
	}
	out, e = owned(ctx, c.r, id, actor)
	if e != nil {
		return
	}
	if e = available(out); e != nil {
		return
	}
	if out.State != "ready" {
		return out, domain.Fail(409, "receipt_not_ready", "Document receipt must pass all verification checks")
	}
	if out.Version != version {
		return out, domain.Stale
	}
	if e = c.r.Consume(ctx, out, actor, tender); e != nil {
		return
	}
	out.ConsumedBy = tender
	out.State = "consumed"
	return
}
func (c *Consumer) MemberIDs(ctx context.Context, r domain.Receipt) ([]string, error) {
	if e := c.r.Guard(ctx); e != nil {
		return nil, e
	}
	return c.r.MemberIDs(ctx, r)
}
func (c *Consumer) Member(ctx context.Context, r domain.Receipt, id string) (domain.Member, error) {
	if e := c.r.Guard(ctx); e != nil {
		return domain.Member{}, e
	}
	return c.r.Member(ctx, r, id)
}

type Worker struct {
	u      ports.Unit
	v      ports.Values
	verify ports.Verifier
}

func NewWorker(u ports.Unit, v ports.Values, verify ports.Verifier) *Worker {
	return &Worker{u, v, verify}
}
func (w *Worker) Once(ctx context.Context) (worked bool, e error) {
	var claim *domain.Receipt
	e = w.u.Within(ctx, func(ctx context.Context, r ports.Repository) error {
		var e error
		claim, e = r.Claim(ctx, w.v.ID())
		return e
	})
	if e != nil || claim == nil {
		return false, e
	}
	worked = true
	parentCtx := ctx
	stopClaim := func() {}
	if keeper, ok := w.u.(ports.LeaseKeeper); ok {
		ctx, stopClaim = keeper.KeepClaim(ctx, *claim)
		defer stopClaim()
	}
	emit := func(m domain.Member) error {
		m.ID = w.v.ID()
		m.ReceiptID = claim.ID
		m.ClaimID = claim.ClaimID
		return w.u.Within(ctx, func(ctx context.Context, r ports.Repository) error { return r.AddMember(ctx, *claim, m) })
	}
	var err error
	if verifier, ok := w.verify.(ports.ManifestVerifier); ok {
		err = verifier.VerifyManifest(ctx, domain.Verification{Receipt: *claim, Files: claim.Files}, emit, func(d domain.Disposition) error {
			return w.u.Within(ctx, func(ctx context.Context, r ports.Repository) error { return r.AddDisposition(ctx, *claim, d) })
		})
	} else {
		err = w.verify.Verify(ctx, domain.Verification{Receipt: *claim, Files: claim.Files}, emit)
	}
	if ctx.Err() != nil {
		return worked, ctx.Err()
	}
	code := ""
	if err != nil {
		code = "verification_failed"
		var d *domain.Error
		if errors.As(err, &d) {
			code = d.Code
		}
	}
	stopClaim()
	ctx = parentCtx
	e = w.u.Within(ctx, func(ctx context.Context, r ports.Repository) error { return r.FinishVerification(ctx, *claim, code) })
	return worked, e
}

func (s *Service) Authorize(ctx context.Context, token string) error {
	actor, e := s.a.Subject(ctx, token)
	if e != nil {
		return e
	}
	return s.u.Within(ctx, func(ctx context.Context, r ports.Repository) error { return Live(ctx, r, actor, true) })
}

// Maintain removes only bounded, leased, expired/unreferenced technical objects;
// consumed Tender evidence is excluded by the owning query and cannot be selected.
func (w *Worker) Maintain(ctx context.Context, objects ports.Objects) error {
	var candidates []domain.ObjectRef
	e := w.u.Within(ctx, func(ctx context.Context, r ports.Repository) error {
		var e error
		candidates, e = r.PurgeCandidates(ctx, w.v.ID())
		return e
	})
	if e != nil {
		return e
	}
	for _, o := range candidates {
		if e = objects.Remove(ctx, o.ID); e != nil {
			return e
		}
		if e = w.u.Within(ctx, func(ctx context.Context, r ports.Repository) error { return r.FinishPurge(ctx, o) }); e != nil {
			return e
		}
	}
	return nil
}

func (s *Service) WithLimits(l ports.RequestLimits) *Service { s.limits = l; return s }
func (s *Service) limit(actor, operation string) error {
	if s.limits != nil && !s.limits.Allow(actor, operation) {
		return domain.Fail(429, "rate_limited", "Too many document receipt requests")
	}
	return nil
}

func (c *Consumer) Retain(ctx context.Context, actor, id, draft string) error {
	if e := Live(ctx, c.r, actor, true); e != nil {
		return e
	}
	v, e := owned(ctx, c.r, id, actor)
	if e != nil {
		return e
	}
	if e = available(v); e != nil {
		return e
	}
	return c.r.RetainDraft(ctx, id, actor, draft)
}
func (c *Consumer) Receipts(ctx context.Context, actor, draft string) ([]domain.Receipt, error) {
	if e := Live(ctx, c.r, actor, false); e != nil {
		return nil, e
	}
	return c.r.DraftReceipts(ctx, draft)
}
func (c *Consumer) Publish(ctx context.Context, actor, id, draft string) (domain.Receipt, error) {
	if e := Live(ctx, c.r, actor, true); e != nil {
		return domain.Receipt{}, e
	}
	v, e := owned(ctx, c.r, id, actor)
	if e != nil {
		return v, e
	}
	if v.DraftID != draft || v.State != "ready" {
		return v, domain.Stale
	}
	if e = c.r.PublishDraft(ctx, v, draft); e != nil {
		return v, e
	}
	v.ConsumedBy = draft
	v.State = "consumed"
	return v, nil
}

func (c *Consumer) MemberInfo(ctx context.Context, r domain.Receipt, id string) (domain.Member, error) {
	if e := c.r.Guard(ctx); e != nil {
		return domain.Member{}, e
	}
	return c.r.MemberInfo(ctx, r, id)
}

func (c *Consumer) Dispositions(ctx context.Context, v domain.Receipt) ([]domain.Disposition, error) {
	if e := c.r.Guard(ctx); e != nil {
		return nil, e
	}
	return c.r.Dispositions(ctx, v)
}

func (s *Service) Original(ctx context.Context, token, id, fileID string) (out []byte, name string, e error) {
	actor, e := s.a.Subject(ctx, token)
	if e != nil {
		return nil, "", e
	}
	if e = s.limit(actor, "read"); e != nil {
		return nil, "", e
	}
	if e = domain.UUID(id); e != nil {
		return nil, "", e
	}
	if e = domain.UUID(fileID); e != nil {
		return nil, "", e
	}
	var file domain.File
	e = s.u.Within(ctx, func(ctx context.Context, r ports.Repository) error {
		if e := Live(ctx, r, actor, false); e != nil {
			return e
		}
		if _, e := owned(ctx, r, id, actor); e != nil {
			return e
		}
		var e error
		file, e = r.File(ctx, id, fileID)
		return e
	})
	if e != nil {
		return nil, "", e
	}
	if file.ObjectKey == "" || file.State == "declared" || file.State == "uploading" {
		return nil, "", domain.Missing
	}
	out, e = s.objects.Original(ctx, file.ObjectKey, file.Size, file.SHA256)
	return out, file.Filename, e
}

func (c *Consumer) ReadyDrafts(ctx context.Context) ([]domain.Receipt, error) {
	if e := c.r.Guard(ctx); e != nil {
		return nil, e
	}
	return c.r.ReadyDrafts(ctx)
}
