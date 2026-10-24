package contracts

import "context"

type VendorReviewTask struct {
	ID, DocumentID, Title, Kind, Outcome         string
	Completed                                    bool
	SupplierID, Code, Name, State, Message, Link string
}
type VendorReviewTasks interface {
	Own(context.Context, string) ([]VendorReviewTask, error)
}
