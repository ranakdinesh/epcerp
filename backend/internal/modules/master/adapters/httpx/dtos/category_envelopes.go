package dtos

type CategorySourceRequest struct {
	CategorySourceDTO
	Active *bool `json:"is_active"`
}
type CategoryImportRowsRequest struct {
	Records []CategorySourceRequest `json:"records"`
}
