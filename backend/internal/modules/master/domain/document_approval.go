package domain

const DocumentAutoApprovePermission = "accounts.vendor_verification.auto_approve"

type DocumentApproval struct {
	DocumentID, SupplierID, SHA256, State, Comment string
	DocumentVersion, Version                       int32
}

type DocumentApprovalReconciliation struct{ Processed, Unreadable, Replaced int }
