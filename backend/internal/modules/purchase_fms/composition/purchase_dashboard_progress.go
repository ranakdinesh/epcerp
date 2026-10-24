package composition

import (
	"context"
	documentInfra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	accountsOut "github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/adapters/outbound"
	masterModule "github.com/ranakdinesh/epcERP/internal/modules/master"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func purchaseLaterProgressProviders(c database.Connection, p purchaseRepository.IndentsProviders, cipher *documentInfra.DocumentCipher) purchaseRepository.LaterProgressProviders {
	tp := purchaseTechnicalProviders(c, p)
	tp.Clearances = purchaseRepository.NewQuotationProjectClearance(c, quotationApprovalProviders(c, cipher))
	orders := purchaseRepository.NewOrdersReleasedReader(c)
	acceptance := purchaseRepository.AcceptanceProviders{Projects: p.Projects, Purchase: p.Scope, Access: tp.Access, Users: p.Users, Work: tp.Work, Orders: orders}
	gate := masterRepository.NewVendorVerificationGate(c, masterRepository.NewVendorDirectory(c), masterModule.DocumentEvidence(c, cipher, documentInfra.NewDocumentInspector(masterDomain.VendorDocumentMaximumBytes)))
	advances := purchaseRepository.AdvancesProviders{Projects: p.Projects, Purchase: p.Scope, Access: tp.Access, Users: p.Users, Work: tp.Work, Orders: orders, Accepted: purchaseRepository.NewAcceptanceAcceptedReader(c, acceptance), Verification: gate, Verified: gate}
	return purchaseRepository.LaterProgressProviders{
		Workflow: purchaseWorkflowProviders(c, p), Sourcing: purchaseRepository.NewSourcingContext(purchaseSourcingProviders(c, p, cipher)), Technical: purchaseRepository.NewTechnicalContext(tp), Acceptance: purchaseRepository.NewAcceptanceContext(acceptance), Advances: purchaseRepository.NewAdvancesContext(advances),
		Inspections:           purchaseRepository.NewInspectionsContext(purchaseInspectionProviders(c, p)),
		InspectionReports:     purchaseRepository.NewInspectionReportsContext(purchaseReportProviders(c, p)),
		Quality:               purchaseRepository.NewQualityContext(purchaseQualityProviders(c, p)),
		ClientClearance:       purchaseRepository.NewClientClearanceContext(purchaseClientClearanceProviders(c, p)),
		Dispatch:              purchaseRepository.NewDispatchContext(purchaseDispatchProviders(c, p)),
		SiteReceipts:          purchaseRepository.NewSiteReceiptsContext(purchaseSiteReceiptProviders(c, p)),
		GRNs:                  purchaseRepository.NewGRNsContext(purchaseGRNProviders(c, p)),
		EngineerConfirmations: purchaseRepository.NewEngineerConfirmationsContext(purchaseEngineerProviders(c, p)),
		Accounts: func(ctx context.Context, project, pkg string) (domain.ProgressEvidence, error) {
			source, e := accountsOut.NewContext(purchaseAccountsProviders(c, p)).Source(ctx, project, pkg)
			if e != nil {
				return domain.ProgressEvidence{}, e
			}
			fact, e := accountsOut.NewProgressReader(c).Progress(ctx, project, pkg, source.SourceFacts, source.Guard() == nil)
			return domain.ProgressEvidence{Number: 20, State: fact.State, Details: fact.Details}, e
		},
	}
}
