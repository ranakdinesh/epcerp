package domain

type QuotationApprovalsQueueRow struct {
	AttemptID     string                      `json:"attempt_id"`
	GroupID       string                      `json:"group_id"`
	GroupName     string                      `json:"group_name"`
	State         string                      `json:"state"`
	Amount        string                      `json:"amount"`
	Currency      string                      `json:"currency"`
	SubmittedAt   QuotationApprovalsTimestamp `json:"submitted_at"`
	ProjectID     string                      `json:"project_id"`
	ProjectName   string                      `json:"project_name"`
	ProjectNumber string                      `json:"project_number"`
	BOID          string                      `json:"bo_id"`
	Version       int32                       `json:"version"`
	UpdatedAt     QuotationApprovalsTimestamp `json:"updated_at"`
}
type QuotationApprovalsQueuePage struct {
	Rows []QuotationApprovalsQueueRow `json:"rows"`
	Next *int32                       `json:"next_offset"`
}
