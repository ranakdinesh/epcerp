// Package httpx binds Master HTTP routes to application services.
package httpx

import (
	"net/http"

	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

// Services contains the application services used by Master HTTP handlers.
// The API composition root constructs them before registering routes.
type Services struct {
	VendorVerificationHTTPDisabled bool
	VendorDocumentsHTTPDisabled    bool
	ClientsHTTPDisabled            bool
	ReferencesHTTPDisabled         bool
	CatalogueHTTPDisabled          bool
	VendorProfilesHTTPDisabled     bool
	VendorReviewAssignments        *masterApplication.VendorReviewAssignmentService
	Vendors                        *masterApplication.VendorService
	VendorDocuments                *masterApplication.VendorDocumentService
	VendorVerification             *masterApplication.VendorVerificationService
	Items                          *masterApplication.ItemService
	Manufacturers                  *masterApplication.ManufacturerService
	Categories                     *masterApplication.CategoryService
	Templates                      *masterApplication.TemplateService
	References                     *masterApplication.ReferenceService
	Clients                        *masterApplication.ClientService
}

// Register binds routes and HTTP payload policy to already constructed services.
func Register(mux *infrastructure.Router, services Services) {
	mux.AddPayloadPolicy(func(r *http.Request) bool {
		return r.Method == http.MethodPost && VendorDocumentIsUploadPath(r.URL.Path)
	}, masterDomain.VendorDocumentMaximumBytes+(64<<10), nil)
	if !services.VendorProfilesHTTPDisabled {
		RegisterVendorRoutes(mux, services.Vendors)
	}
	if !services.VendorDocumentsHTTPDisabled {
		RegisterVendorDocumentRoutes(mux, services.VendorDocuments)
	}
	if !services.VendorVerificationHTTPDisabled {
		mux.Post("/api/v1/suppliers/{supplier_id}/documents/reconcile-approvals", vendorDocumentHandler{services.VendorDocuments}.reconcileApprovals)
		RegisterVendorVerificationRoutes(mux, services.VendorVerification)
		RegisterVendorReviewAssignments(mux, services.VendorReviewAssignments)
	}
	if !services.CatalogueHTTPDisabled {
		RegisterItemRoutes(mux, services.Items)
		RegisterManufacturerRoutes(mux, services.Manufacturers)
		RegisterCategoryRoutes(mux, services.Categories)
		RegisterTemplateRoutes(mux, services.Templates)
	}
	if !services.ReferencesHTTPDisabled {
		RegisterReferenceRoutes(mux, services.References)
	}
	if !services.ClientsHTTPDisabled {
		RegisterClientRoutes(mux, services.Clients)
	}
}
