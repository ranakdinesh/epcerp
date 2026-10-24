package repository

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	dto "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type VendorReviewTasks struct {
	c     database.Connection
	q     *db.Queries
	users auth.AssignmentDirectory
}

func NewVendorReviewTasks(c database.Connection, u auth.AssignmentDirectory, _ contracts.VendorDirectory, _ contracts.VendorDocumentReader) *VendorReviewTasks {
	return &VendorReviewTasks{c, NewStore(c).Queries, u}
}
func (r *VendorReviewTasks) Own(ctx context.Context, user string) ([]contracts.VendorReviewTask, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, domain.VendorVerificationErrTransactionRequired
	}
	actor, e := r.users.Get(ctx, user)
	if e != nil {
		return nil, e
	}
	out := []contracts.VendorReviewTask{}
	if actor == nil || !actor.Active || !actor.Allows("work.tasks.read") || !actor.Allows("work.tasks.view") {
		return out, nil
	}
	rows, e := r.q.DocumentTasksOwn(ctx, dto.UUID(user))
	if e != nil {
		return nil, e
	}
	for _, row := range rows {
		if row.Kind == "review" && (!actor.Allows("accounts.vendor_verification.read") || !actor.Allows("accounts.vendor_verification.write")) {
			continue
		}
		if row.Kind == "correction" && !actor.Allows("settings.supplier_documents.write") {
			continue
		}
		title := row.Title
		if title == "" {
			title = row.OriginalFilename
		}
		state, message := "pending", "Review document: "+title
		if row.Kind == "correction" {
			message = "Replace rejected document: " + title
		}
		if row.CompletedAt.Valid {
			state = "completed"
			message = row.Outcome + ": " + title
		}
		out = append(out, contracts.VendorReviewTask{ID: row.ID.String(), DocumentID: row.DocumentID.String(), Title: title, Kind: row.Kind, Outcome: row.Outcome, Completed: row.CompletedAt.Valid, SupplierID: row.SupplierID.String(), Code: row.Code, Name: row.Name, State: state, Message: message, Link: "/dashboard?screen=suppliers&supplier_id=" + row.SupplierID.String() + "&document_id=" + row.DocumentID.String()})
	}
	return out, nil
}
