package httpx

import (
	"github.com/go-chi/chi/v5"

	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
)

type Services struct {
	LOIDocuments          *application.LOIDocumentsService
	Awards                *application.AwardsService
	ClientRules           *application.ClientRulesService
	Quotations            *application.QuotationsService
	Acceptance            *application.AcceptanceService
	Advances              *application.AdvancesService
	BO                    *application.BOService
	ClientClearance       *application.ClientClearanceService
	Dispatch              *application.DispatchService
	EngineerConfirmations *application.EngineerConfirmationsService
	Workspace             *application.WorkspaceService
	GRNs                  *application.GRNsService
	Holds                 *application.HoldsService
	IndentDocuments       *application.IndentDocumentsService
	IndentGroups          *application.IndentGroupsService
	Indents               *application.IndentsService
	InspectionReports     *application.InspectionReportsService
	Inspections           *application.InspectionsService
	Orders                *application.OrdersService
	Quality               *application.QualityService
	SiteReceipts          *application.SiteReceiptsService
	Sourcing              *application.SourcingService
	Technical             *application.TechnicalService
	TechnicalEvidence     *application.TechnicalEvidenceService
	Workflow              *application.WorkflowService
	QuotationApprovals    *application.QuotationApprovalsService
}

func Register(router chi.Router, services Services) {
	LOIDocumentsRegister(router, services.LOIDocuments)
	AwardsRegister(router, services.Awards)
	ClientRulesRegister(router, services.ClientRules)
	QuotationsRegister(router, services.Quotations)
	AcceptanceRegister(router, services.Acceptance)
	AdvancesRegister(router, services.Advances)
	BORegister(router, services.BO)
	ClientClearanceRegister(router, services.ClientClearance)
	DispatchRegister(router, services.Dispatch)
	EngineerConfirmationsRegister(router, services.EngineerConfirmations)
	WorkspaceRegister(router, services.Workspace)
	GRNsRegister(router, services.GRNs)
	HoldsRegister(router, services.Holds)
	IndentDocumentsRegister(router, services.IndentDocuments)
	IndentGroupsRegister(router, services.IndentGroups)
	IndentsRegister(router, services.Indents)
	InspectionReportsRegister(router, services.InspectionReports)
	InspectionsRegister(router, services.Inspections)
	OrdersRegister(router, services.Orders)
	QualityRegister(router, services.Quality)
	SiteReceiptsRegister(router, services.SiteReceipts)
	SourcingRegister(router, services.Sourcing)
	TechnicalRegister(router, services.Technical, services.TechnicalEvidence)
	WorkflowRegister(router, services.Workflow)
	QuotationApprovalsRegister(router, services.QuotationApprovals)
}
func ClientRulesRegister(r chi.Router, s *application.ClientRulesService) {
	h := clientRulesHandler{s}
	r.Get("/api/v1/clients/{client}/purchase-rules", h.list)
	r.Get("/api/v1/clients/{client}/purchase-rules/{id}/history", h.history)
	r.Post("/api/v1/clients/{client}/purchase-rules", h.save)
	r.Patch("/api/v1/clients/{client}/purchase-rules/{id}", h.save)
}

func QuotationsRegister(r chi.Router, s *application.QuotationsService) {
	h := quotationsHandler{s}
	r.Get("/api/v1/projects/{project}/quotation-bo", h.boChoices)
	r.Get(quotationsBase, h.workspace)
	r.Post(quotationsBase, h.execute)
	r.Post(quotationsBase+"/{document}/attachments", h.upload)
	r.Get(quotationsBase+"/{document}/attachments/{file}/download", h.download)
}

func AcceptanceRegister(r chi.Router, s *application.AcceptanceService) {
	h := acceptanceHandler{s}
	r.Route("/api/v1/projects/{project}/purchase-workflow/{package}/acceptance", func(r chi.Router) {
		r.Get("/", h.workspace)
		r.Post("/", h.execute)
	})
}

func AdvancesRegister(r chi.Router, s *application.AdvancesService) {
	h := advancesHandler{s}
	r.Get("/api/v1/purchase-advances", h.queue)
	r.Route("/api/v1/projects/{project}/purchase-workflow/{package}/advances", func(r chi.Router) {
		r.Get("/", h.workspace)
		r.Post("/", h.execute)
	})
}

func BORegister(r chi.Router, s *application.BOService) {
	h := bOHandler{s}
	r.Get("/api/v1/projects/{project}/purchase-bo", h.list)
	r.Get("/api/v1/projects/{project}/purchase-bo/pdf", h.pdf)
	r.Post("/api/v1/projects/{project}/purchase-bo/register-complete", h.registerChange)
	r.Post("/api/v1/projects/{project}/purchase-bo/register-reopen", h.registerChange)
	r.Get("/api/v1/projects/{project}/purchase-bo/{id}/history", h.history)
	r.Post("/api/v1/projects/{project}/purchase-bo/{id}", h.save)
	r.Post("/api/v1/projects/{project}/purchase-bo/{id}/complete", h.change)
	r.Post("/api/v1/projects/{project}/purchase-bo/{id}/reopen", h.change)
}

func ClientClearanceRegister(r chi.Router, s *application.ClientClearanceService) {
	h := clientClearanceHandler{s}
	r.Route("/api/v1/projects/{project}/purchase-workflow/{package}/client-clearance", func(r chi.Router) {
		r.Get("/", h.workspace)
		r.Post("/", h.execute)
	})
}

func DispatchRegister(r chi.Router, s *application.DispatchService) {
	h := dispatchHandler{s}
	r.Route("/api/v1/projects/{project}/purchase-workflow/{package}/dispatch", func(r chi.Router) {
		r.Get("/", h.workspace)
		r.Post("/", h.execute)
	})
}

func EngineerConfirmationsRegister(r chi.Router, s *application.EngineerConfirmationsService) {
	h := engineerConfirmationsHandler{s}
	r.Route("/api/v1/projects/{project}/purchase-workflow/{package}/engineer-confirmation", func(r chi.Router) {
		r.Get("/", h.workspace)
		r.Post("/", h.execute)
	})
}

func WorkspaceRegister(r chi.Router, s *application.WorkspaceService) {
	h := workspaceHandler{s}
	r.Get("/api/v1/purchase-fms", h.list)
	r.Get("/api/v1/purchase-fms/projects/{id}", h.context)
	r.Get("/api/v1/purchase-fms/projects/{id}/assignment-options", h.options)
	r.Patch("/api/v1/purchase-fms/projects/{id}/owner", h.assign)
}

func GRNsRegister(r chi.Router, s *application.GRNsService) {
	h := gRNsHandler{s}
	r.Route("/api/v1/projects/{project}/purchase-workflow/{package}/grn", func(r chi.Router) {
		r.Get("/", h.workspace)
		r.Post("/", h.execute)
	})
}

func HoldsRegister(r chi.Router, s *application.HoldsService) {
	h := holdsHandler{s}
	r.Get("/api/v1/projects/{project}/purchase-holds", h.list)
	r.Get("/api/v1/projects/{project}/purchase-holds/{id}/history", h.history)
	r.Post("/api/v1/projects/{project}/purchase-holds", h.create)
	r.Patch("/api/v1/projects/{project}/purchase-holds/{id}/release", h.release)
}

func IndentDocumentsRegister(r chi.Router, s *application.IndentDocumentsService) {
	h := indentDocumentsHandler{s}
	r.Get("/api/v1/projects/{project}/purchase-bo/{bo}/indent-documents", h.workspace)
	r.Post("/api/v1/projects/{project}/purchase-bo/{bo}/indent-documents", h.execute)
	r.Get("/api/v1/indent-dispatch-notifications", h.notifications)
}

func IndentGroupsRegister(r chi.Router, s *application.IndentGroupsService) {
	h := indentGroupsHandler{s}
	r.Get("/api/v1/projects/{project}/purchase-bo/{bo}/indent-groups", h.workspace)
	r.Post("/api/v1/projects/{project}/purchase-bo/{bo}/indent-groups", h.execute)
}

func IndentsRegister(r chi.Router, s *application.IndentsService) {
	h := indentsHandler{s}
	r.Get("/api/v1/projects/{project}/purchase-indents", h.list)
	r.Get("/api/v1/projects/{project}/purchase-indents/workspace", h.workspace)
	r.Get("/api/v1/projects/{project}/purchase-indents/{id}/history", h.history)
	r.Post("/api/v1/projects/{project}/purchase-indents", h.save)
	r.Patch("/api/v1/projects/{project}/purchase-indents/{id}", h.save)
	r.Post("/api/v1/projects/{project}/purchase-indents/{id}/actions", h.action)
}

func InspectionReportsRegister(r chi.Router, s *application.InspectionReportsService) {
	h := inspectionReportsHandler{s}
	r.Route("/api/v1/projects/{project}/purchase-workflow/{package}/inspection-report", func(r chi.Router) {
		r.Get("/", h.workspace)
		r.Post("/", h.execute)
	})
}

func InspectionsRegister(r chi.Router, s *application.InspectionsService) {
	h := inspectionsHandler{s}
	r.Route("/api/v1/projects/{project}/purchase-workflow/{package}/inspection", func(r chi.Router) {
		r.Get("/", h.workspace)
		r.Post("/", h.execute)
	})
}

func OrdersRegister(r chi.Router, s *application.OrdersService) {
	if s == nil {
		return
	}
	h := ordersHandler{s}
	r.Route("/api/v1/projects/{project}/purchase-workflow/{package}/purchase-orders", func(r chi.Router) {
		r.Post("/uploads/{version}", h.upload)
		r.Get("/uploads/{version}", h.uploadAuthority)
		r.Get("/files/{file}", h.file)
		r.Get("/", h.workspace)
		r.Post("/", h.execute)
		r.Get("/document", h.document)
	})
}

func QualityRegister(r chi.Router, s *application.QualityService) {
	h := qualityHandler{s}
	r.Route("/api/v1/projects/{project}/purchase-workflow/{package}/quality", func(r chi.Router) {
		r.Get("/", h.workspace)
		r.Post("/", h.execute)
	})
}

func SiteReceiptsRegister(r chi.Router, s *application.SiteReceiptsService) {
	h := siteReceiptsHandler{s}
	r.Route("/api/v1/projects/{project}/purchase-workflow/{package}/site-receipt", func(r chi.Router) {
		r.Get("/", h.workspace)
		r.Post("/", h.execute)
	})
}

func SourcingRegister(r chi.Router, s *application.SourcingService) {
	h := sourcingHandler{s}
	r.Route("/api/v1/projects/{project}/purchase-workflow/{package}/sourcing", func(r chi.Router) { r.Get("/", h.workspace); r.Post("/", h.execute) })
}

func TechnicalRegister(r chi.Router, s *application.TechnicalService, files *application.TechnicalEvidenceService) {
	h := technicalHandler{s, files}
	r.Route("/api/v1/projects/{project}/purchase-workflow/{package}/technical", func(r chi.Router) {
		r.Get("/", h.workspace)
		r.Post("/", h.execute)
		r.Get("/uploads/{purpose}/{version}", h.authority)
		r.Post("/uploads/{purpose}/{version}", h.upload)
		r.Get("/files/{file}", h.download)
	})
}

func WorkflowRegister(r chi.Router, s *application.WorkflowService) {
	h := workflowHandler{s}
	r.Route("/api/v1/projects/{project}/purchase-workflow", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/{package}", h.workspace)
		r.Post("/{package}/assignments", h.assign)
		r.Post("/{package}/steps/{step}", h.Command)
	})
}

func QuotationApprovalsRegister(r chi.Router, s *application.QuotationApprovalsService) {
	h := quotationApprovalsHandler{s}
	r.Get("/api/v1/quotation-approvals", h.queue)
	r.Get("/api/v1/projects/{project}/quotation-status", h.projectStatus)
	r.Route("/api/v1/projects/{project}/purchase-bo/{bo}/quotation-approvals", func(r chi.Router) {
		r.Get("/", h.workspace)
		r.Get("/budget-preview", h.preview)
		r.Get("/print", h.print)
		r.Post("/print", h.print)
		r.Post("/draft", h.draft)
		r.Post("/submit", h.submit)
		r.Post("/review", h.review)
	})
}
func IsUploadPath(path string) bool {
	return QuotationsIsUploadPath(path) || TechnicalIsUploadPath(path) || OrdersIsUploadPath(path)
}
