package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"
	"sort"

	pgx "github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type VendorVerificationUnitOfWork struct {
	approval  func(database.Connection) masterPorts.DocumentApprovalLifecycle
	m         *database.Manager
	suppliers func(database.Connection) masterContracts.VendorDirectory
	documents func(database.Connection) masterContracts.VendorDocumentReader
	events    func(database.Connection) events.Writer
	users     func(database.Connection) auth.AssignmentDirectory
}

func NewVendorVerificationUnitOfWork(b database.Beginner, s func(database.Connection) masterContracts.VendorDirectory, d func(database.Connection) masterContracts.VendorDocumentReader, e func(database.Connection) events.Writer, users func(database.Connection) auth.AssignmentDirectory) *VendorVerificationUnitOfWork {
	return &VendorVerificationUnitOfWork{m: database.NewManager(b), suppliers: s, documents: d, events: e, users: users}
}
func (u *VendorVerificationUnitOfWork) Within(ctx context.Context, w func(context.Context, masterPorts.VendorVerificationRepository, masterPorts.VendorVerificationEvidence, masterPorts.VendorVerificationUsers) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		var repository masterPorts.VendorVerificationRepository = &vendorVerificationRepository{NewStore(c).Queries, u.events(c)}
		if u.approval != nil {
			repository = &verificationWorkflowRepository{repository, u.approval(c)}
		}
		return w(ctx, repository, &VendorVerificationEvidenceBridge{u.suppliers(c), u.documents(c)}, &VendorVerificationUserDirectory{u.users(c)})
	})
}

type vendorVerificationRepository struct {
	q      *db.Queries
	events events.Writer
}

func (r *vendorVerificationRepository) Get(ctx context.Context, id string) (*masterDomain.VendorVerificationReview, error) {
	v, e := r.q.VendorVerificationGetReview(ctx, masterDTO.UUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return masterDTO.VendorVerificationFromRow(v)
}
func (r *vendorVerificationRepository) Save(ctx context.Context, v masterDomain.VendorVerificationReview, actor string) (*masterDomain.VendorVerificationReview, error) {
	data, e := masterDTO.MarshalMasterJSON(v.Decisions)
	if e != nil {
		return nil, e
	}
	row, e := r.q.VendorVerificationSaveReview(ctx, db.VendorVerificationSaveReviewParams{SupplierID: masterDTO.UUID(v.SupplierID), Version: v.Version, Fingerprint: v.Fingerprint, Decisions: data, Reason: v.Reason, Actor: masterDTO.UUID(actor)})
	if e != nil {
		return nil, e
	}
	return masterDTO.VendorVerificationFromRow(row)
}
func (r *vendorVerificationRepository) Journal(ctx context.Context, v masterDomain.VendorVerificationReview, event string) error {
	payload, e := masterDTO.MarshalMasterJSON(v)
	if e != nil {
		return e
	}
	if e = r.q.VendorVerificationRecordChange(ctx, db.VendorVerificationRecordChangeParams{ID: masterDTO.UUID(event), SupplierID: masterDTO.UUID(v.SupplierID), Version: v.Version, Snapshot: payload, CreatedByID: masterDTO.UUID(*v.UpdatedBy)}); e != nil {
		return e
	}
	return r.events.Record(ctx, events.Event{ID: event, Module: "supplier_verification", EntityType: "supplier_verification", EntityID: v.SupplierID, EntityVersion: int64(v.Version), Action: "reviewed", ActorID: *v.UpdatedBy, ActorKind: "user", CorrelationID: event, ScopeKind: "platform", Visibility: "restricted", OccurredAt: v.UpdatedAt}, nil)
}

type VendorVerificationEvidenceBridge struct {
	s masterContracts.VendorDirectory
	d masterContracts.VendorDocumentReader
}

func (e *VendorVerificationEvidenceBridge) Snapshot(ctx context.Context, id string) (*masterDomain.VendorVerificationSnapshot, error) {
	s, err := e.s.Snapshot(ctx, id)
	if err != nil || s == nil {
		return nil, err
	}
	docs, err := e.d.Active(ctx, id)
	if err != nil {
		return nil, err
	}
	v := &masterDomain.VendorVerificationSnapshot{ID: s.ID, Version: s.Version, Active: s.Active, GSTRegistered: s.GSTRegistered, Documents: []masterDomain.VendorVerificationEvidence{}}
	for _, d := range docs {
		v.Documents = append(v.Documents, masterDomain.VendorVerificationEvidence{ID: d.ID, DocType: d.DocType, SHA256: d.SHA256, Version: d.Version, Previewable: d.Previewable, ApprovalKnown: d.ApprovalKnown, Approval: d.Approval, ApprovalComment: d.ApprovalComment})
	}
	return v, nil
}

type VendorVerificationValues struct{}

func (VendorVerificationValues) ID() string {
	return newRecordID()
}

func (VendorVerificationValues) Fingerprint(s masterDomain.VendorVerificationSnapshot) string {
	copy := s
	copy.Documents = append([]masterDomain.VendorVerificationEvidence{}, s.Documents...)
	for i := range copy.Documents {
		copy.Documents[i].ApprovalKnown = false
		copy.Documents[i].Approval = ""
		copy.Documents[i].ApprovalComment = ""
	}
	sort.Slice(copy.Documents, func(i, j int) bool { return copy.Documents[i].ID < copy.Documents[j].ID })
	type evidence struct {
		ID, DocType, SHA256 string
		Version             int32
		Previewable         bool
	}
	wire := struct {
		ID                    string
		Version               int32
		Active, GSTRegistered bool
		Documents             []evidence
	}{ID: copy.ID, Version: copy.Version, Active: copy.Active, GSTRegistered: copy.GSTRegistered, Documents: []evidence{}}
	for _, d := range copy.Documents {
		wire.Documents = append(wire.Documents, evidence{d.ID, d.DocType, d.SHA256, d.Version, d.Previewable})
	}
	data, _ := json.Marshal(wire)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

type VendorVerificationUserDirectory struct{ r auth.AssignmentDirectory }

func (u *VendorVerificationUserDirectory) Get(ctx context.Context, id string) (*sharedIdentity.Principal, error) {
	v, e := u.r.Get(ctx, id)
	if e != nil || v == nil {
		return nil, e
	}
	return &sharedIdentity.Principal{ID: v.ID, Active: v.Active, Superuser: v.Superuser, Permissions: v.Permissions}, nil
}

func (u *VendorVerificationUnitOfWork) WithApprovalLifecycle(f func(database.Connection) masterPorts.DocumentApprovalLifecycle) *VendorVerificationUnitOfWork {
	u.approval = f
	return u
}
