package dtos

type VendorDocumentArchiveRequest struct {
	Version int32  `json:"version"`
	Reason  string `json:"reason"`
}

type DocumentApprovalReconciliationDTO struct {
	Processed  int `json:"processed"`
	Unreadable int `json:"unreadable"`
	Replaced   int `json:"replaced"`
}
