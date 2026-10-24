package repository

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	inbox "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	dto "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type VendorReviewAssignmentUnitOfWork struct {
	m       *database.Manager
	users   func(database.Connection) auth.AssignmentDirectory
	events  func(database.Connection) events.Writer
	notices func(database.Connection) inbox.SupplierReviewWriter
}

func NewVendorReviewAssignmentUnitOfWork(b database.Beginner, u func(database.Connection) auth.AssignmentDirectory, j func(database.Connection) events.Writer, n func(database.Connection) inbox.SupplierReviewWriter) *VendorReviewAssignmentUnitOfWork {
	return &VendorReviewAssignmentUnitOfWork{database.NewManager(b), u, j, n}
}
func (u *VendorReviewAssignmentUnitOfWork) Within(ctx context.Context, work func(context.Context, ports.VendorReviewAssignmentRepository, ports.VendorReviewAssignmentUsers) error) error {
	return u.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return work(ctx, &vendorReviewAssignmentRepository{NewStore(c).Queries, u.events(c), u.notices(c)}, &vendorReviewUsers{VendorVerificationUserDirectory{u.users(c)}, u.users(c)})
	})
}

type vendorReviewUsers struct {
	VendorVerificationUserDirectory
	directory auth.AssignmentDirectory
}

func (u *vendorReviewUsers) Candidates(ctx context.Context) ([]domain.VendorReviewer, error) {
	values, err := u.directory.List(ctx)
	if err != nil {
		return nil, err
	}
	result := []domain.VendorReviewer{}
	for _, v := range values {
		result = append(result, domain.VendorReviewer{ID: v.ID, Name: v.Name, Eligible: v.Allows("accounts.vendor_verification.view") && v.Allows("accounts.vendor_verification.read") && v.Allows("accounts.vendor_verification.write")})
	}
	return result, nil
}

type vendorReviewAssignmentRepository struct {
	q       *db.Queries
	journal events.Writer
	notices inbox.SupplierReviewWriter
}

func (r *vendorReviewAssignmentRepository) LockSupplier(ctx context.Context, id string) (bool, error) {
	_, err := r.q.VendorReviewLockSupplier(ctx, dto.UUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
func (r *vendorReviewAssignmentRepository) Get(ctx context.Context, id string) (*domain.VendorReviewAssignment, error) {
	v, err := r.q.VendorReviewAssignmentGet(ctx, dto.UUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return dto.VendorReviewAssignment(v), nil
}
func (r *vendorReviewAssignmentRepository) Save(ctx context.Context, v domain.VendorReviewAssignment, reason, event string) (*domain.VendorReviewAssignment, error) {
	err := r.q.VendorReviewAssignmentSave(ctx, db.VendorReviewAssignmentSaveParams{ID: dto.UUID(v.ID), SupplierID: dto.UUID(v.SupplierID), UserID: dto.UUID(v.UserID), Version: v.Version, IsActive: v.Active, CreatedByID: dto.UUID(*v.UpdatedBy)})
	if err != nil {
		return nil, err
	}
	if v.Active {
		if err = r.q.DocumentTasksReassign(ctx, db.DocumentTasksReassignParams{SupplierID: dto.UUID(v.SupplierID), UserID: dto.UUID(v.UserID), UpdatedByID: dto.UUID(*v.UpdatedBy)}); err != nil {
			return nil, err
		}
	}
	saved, err := r.Get(ctx, v.SupplierID)
	if err != nil {
		return nil, err
	}
	snapshot, err := dto.VendorReviewAssignmentSnapshot(saved)
	if err != nil {
		return nil, err
	}
	if err = r.q.VendorReviewAssignmentJournal(ctx, db.VendorReviewAssignmentJournalParams{ID: dto.UUID(event), SupplierID: dto.UUID(v.SupplierID), Version: v.Version, Reason: reason, Snapshot: snapshot, CreatedByID: dto.UUID(*v.UpdatedBy)}); err != nil {
		return nil, err
	}
	action := "assigned"
	if !v.Active {
		action = "revoked"
	}
	if err = r.journal.Record(ctx, events.Event{ID: event, Module: "supplier_review_assignments", EntityType: "supplier_review_assignment", EntityID: v.ID, EntityVersion: int64(v.Version), Action: action, ActorID: *v.UpdatedBy, ActorKind: "user", CorrelationID: event, ScopeKind: "platform", Visibility: "restricted", OccurredAt: saved.UpdatedAt}, nil); err != nil {
		return nil, err
	}
	if v.Active {
		if err = r.notices.SupplierReviewNotice(ctx, event, v.UserID, "Supplier document review assigned", "/dashboard?screen=suppliers&supplier_id="+v.SupplierID); err != nil {
			return nil, err
		}
	}
	return saved, nil
}
