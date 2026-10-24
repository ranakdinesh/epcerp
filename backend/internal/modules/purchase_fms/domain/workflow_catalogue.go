package domain

// Source catalogue v1: preparation definitions, never automatic completion.
func WorkflowDefinitions() []WorkflowStep {
	return []WorkflowStep{
		{Number: 1, Title: "Bought-out (BO) item list", Phase: "WO2I", Evidence: "BO list and reviewed project material requirements", State: "not_started"},
		{Number: 2, Title: "Purchase Indent", Phase: "WO2I", Evidence: "Full-kit indent, delivery allocations and budget review", State: "not_started"},
		{Number: 3, Title: "Invite Quotations from Vendors", Phase: "I2LOI", Evidence: "Eligible supplier list and issued RFQ reference", State: "not_started"},
		{Number: 4, Title: "Price Comparative Table", Phase: "I2LOI", Evidence: "Validated quotations, comparison and selection reason", State: "not_started"},
		{Number: 5, Title: "Vendor award & LOI", Phase: "I2LOI", Evidence: "Existing approved vendor selection and released LOI", State: "not_started"},
		{Number: 6, Title: "Technical Documents from Vendor", Phase: "LOI2PO", Evidence: "GA drawing, TDS, QAP and applicable test documents", State: "not_started"},
		{Number: 7, Title: "Technical review & submission", Phase: "LOI2PO", Evidence: "Document checking and project-team submission reference", State: "not_started"},
		{Number: 8, Title: "Client technical approval", Phase: "LOI2PO", Evidence: "Client stamped approval and approved QAP", State: "not_started"},
		{Number: 9, Title: "Draft PO, client sign-off & release", Phase: "LOI2PO", Evidence: "Finance approval, approved technical revision and client-signed current PO", State: "not_started"},
		{Number: 10, Title: "PO Acceptance", Phase: "PO2INS", Evidence: "Supplier acknowledgement of PO terms", State: "not_started"},
		{Number: 11, Title: "Advance Payment Request", Phase: "PO2INS", Evidence: "Advance request to Accounts, or reason not applicable", State: "not_started"},
		{Number: 12, Title: "Material Inspection", Phase: "PO2INS", Evidence: "Inspection report with findings and acceptance decision", State: "not_started"},
		{Number: 13, Title: "IR / IRN", Phase: "PO2INS", Evidence: "Inspection report and inspection release note", State: "not_started"},
		{Number: 14, Title: "Quality Certificates", Phase: "PO2INS", Evidence: "Applicable TC / TR / MTC / RTC certificates", State: "not_started"},
		{Number: 15, Title: "Dispatch Clearance (Client)", Phase: "INS2D", Evidence: "Client dispatch clearance", State: "not_started"},
		{Number: 16, Title: "Material Dispatch", Phase: "INS2D", Evidence: "LR, invoice, e-way bill and dispatch reference", State: "not_started"},
		{Number: 17, Title: "Received at Site", Phase: "INS2D", Evidence: "Receipt, quantity/condition record and POD / gate entry", State: "not_started"},
		{Number: 18, Title: "Warranty / Guarantee Letter", Phase: "INS2D", Evidence: "Vendor warranty / guarantee letter", State: "not_started"},
		{Number: 19, Title: "PE Site Confirmation", Phase: "INS2D", Evidence: "Project engineer site-inspection confirmation", State: "not_started"},
		{Number: 20, Title: "Accounts Acceptance Confirmation", Phase: "INS2D", Evidence: "Accounts acknowledgement of material acceptance", State: "not_started"},
	}
}
