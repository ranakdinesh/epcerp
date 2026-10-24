package dtos

type ItemCreateRequest struct {
	ConfigurationID string            `json:"configuration_id"`
	ManufacturerID  string            `json:"manufacturer_id"`
	Specifications  map[string]string `json:"specifications"`
	Model           string            `json:"model"`
}
type ItemChangeRequest struct {
	Status  string `json:"status"`
	Version int32  `json:"version"`
	Reason  string `json:"reason"`
}
