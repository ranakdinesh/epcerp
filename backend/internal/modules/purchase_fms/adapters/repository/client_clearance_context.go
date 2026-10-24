package repository

import (
	"context"
	"errors"

	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type ClientClearanceProviders struct {
	Projects projects.ContextReader
	Purchase contracts.WorkspaceAccessGate
	Access   access.Gate
	Users    auth.AssignmentDirectory
	Work     contracts.WorkflowSourceReader
	Quality  contracts.QualityVerifiedReader
}
type clientClearanceSourceContext struct{ p ClientClearanceProviders }

func NewClientClearanceContext(p ClientClearanceProviders) *clientClearanceSourceContext {
	return &clientClearanceSourceContext{p}
}
func clientClearanceMapped(e error) error {
	var ic *contracts.QualityError
	if errors.As(e, &ic) {
		return &domain.ClientClearanceError{Status: ic.Status, Field: ic.Field, Message: ic.Message}
	}
	var a *access.Error
	if errors.As(e, &a) {
		return &domain.ClientClearanceError{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var w *contracts.WorkflowError
	if errors.As(e, &w) {
		return &domain.ClientClearanceError{Status: w.Status, Field: w.Field, Message: w.Message}
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.ClientClearanceInvalid(p.Field, p.Message)
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.ClientClearanceForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.ClientClearanceMissing
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.ClientClearanceUnauthorized
	}
	return e
}
func clientClearanceCan(a domain.Principal, code string) bool {
	if !a.Active {
		return false
	}
	if a.Superuser {
		return true
	}
	for _, p := range a.Permissions {
		if p == code {
			return true
		}
	}
	return false
}
func (c *clientClearanceSourceContext) scoped(ctx context.Context, a domain.Principal, p, code string, write bool) (bool, error) {
	v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	if e == nil {
		return !write || v.CanWrite, nil
	}
	e = clientClearanceMapped(e)
	var d *domain.ClientClearanceError
	if !errors.As(e, &d) || d.Status != 403 {
		return false, e
	}
	x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	e = clientClearanceMapped(e)
	if errors.As(e, &d) && d.Status == 403 {
		return false, nil
	}
	return e == nil && (!write || x.CanWrite), e
}
func (c *clientClearanceSourceContext) Access(ctx context.Context, a domain.Principal, p, id string) (out domain.ClientClearanceAccess, e error) {

	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.ClientClearanceMissing
	}
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active || u.ID != a.ID {
		return out, domain.ClientClearanceUnauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	live := func(code string) bool { return clientClearanceCan(a, code) && u.Allows(code) }
	if !live("projects.purchase_client_clearance.read") {
		return out, nil
	}
	out.Scoped, e = c.scoped(ctx, a, p, "projects.purchase_client_clearance.read", false)
	if e != nil {
		return out, e
	}
	out.Read = out.Scoped
	wa := contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}
	for _, n := range []int{15} {
		allowed, err := c.p.Work.CanWork(ctx, wa, p, id, n)
		if err != nil {
			var d *domain.ClientClearanceError
			err = clientClearanceMapped(err)
			if !errors.As(err, &d) || d.Status != 422 {
				return out, err
			}
		}
		out.Work = allowed && err == nil
		taskRead, err := c.p.Work.CanReadTask(ctx, wa, p, id, n)
		if err != nil {
			return out, clientClearanceMapped(err)
		}
		out.Read = out.Read || taskRead
	}
	out.Write = out.Read && live("projects.purchase_client_clearance.write")

	return out, e
}

func clientClearanceBlock(out domain.ClientClearanceSource, message string) (domain.ClientClearanceSource, error) {
	out.Available = false
	out.Blockers = append(out.Blockers, message)
	return out, nil
}
func (c *clientClearanceSourceContext) Source(ctx context.Context, p, id string) (domain.ClientClearanceSource, error) {
	kit, current, e := c.p.Work.Inspect(ctx, p, id)
	if e != nil {
		return domain.ClientClearanceSource{}, clientClearanceMapped(e)
	}
	out := domain.ClientClearanceSource{ClientClearanceSourceFacts: domain.ClientClearanceSourceFacts{ProjectID: p, PackageID: id}, Available: current.Available && current.Eligible, Blockers: append([]string{}, current.Blockers...)}
	for _, step := range kit.Steps {
		if step.Number == 15 && step.State == "blocked" {
			out.Blockers = append(out.Blockers, "Client dispatch clearance has an unresolved blocker")
		}
	}
	actual, e := c.p.Quality.Verified(ctx, p, id)
	if e != nil {
		e = clientClearanceMapped(e)
		var d *domain.ClientClearanceError
		if errors.As(e, &d) && (d.Status == 404 || d.Status == 409 || d.Status == 422) {
			return clientClearanceBlock(out, d.Message)
		}
		return out, e
	}
	if actual == nil {
		return clientClearanceBlock(out, "Current quality certificates must be verified before recording client dispatch clearance")
	}
	if actual.ProjectID != p || actual.PackageID != id {
		return domain.ClientClearanceSource{}, domain.ClientClearanceInvalid("source", "Report belongs to another project/package")
	}
	source := actual.Source
	out.ClientClearanceSourceFacts = domain.ClientClearanceSourceFacts{ProjectID: p, PackageID: id, QualityID: actual.ID, QualityVersion: actual.Version, QualityRevision: actual.Revision, ReportID: source.ReportID, ReportVersion: source.ReportVersion, ReportRevision: source.ReportRevision, IRNumber: source.IRNumber, IRNNumber: source.IRNNumber, IssueDate: source.IssueDate, OrderID: source.OrderID, OrderVersion: source.OrderVersion, Number: source.Number, SupplierName: source.SupplierName, Description: source.Description, Quantity: source.Quantity, UOM: source.UOM, InspectionID: source.InspectionID, InspectionVersion: source.InspectionVersion}
	return out, nil
}
