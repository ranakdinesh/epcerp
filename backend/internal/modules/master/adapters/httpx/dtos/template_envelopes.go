package dtos

type TemplatePreviewRequest struct {
	Values map[string]string `json:"values"`
}
type TemplateImportRowsRequest struct {
	Templates []TemplateDefinitionDTO `json:"templates"`
}
type TemplatePublishRequest struct {
	Definition       TemplateDefinitionDTO `json:"definition"`
	ExpectedRevision *int32                `json:"expected_revision"`
	Reason           string                `json:"reason"`
}
