package domain

type OrdersFile struct {
	ID            string          `json:"id"`
	OrderID       string          `json:"order_id"`
	DraftRevision int32           `json:"draft_revision"`
	Name          string          `json:"name"`
	MediaType     string          `json:"media_type"`
	Size          int64           `json:"size"`
	SHA256        string          `json:"-"`
	Bucket        string          `json:"-"`
	ObjectKey     string          `json:"-"`
	CreatedAt     OrdersTimestamp `json:"created_at"`
	CreatedBy     string          `json:"created_by_id"`
}
type OrdersFileEvent struct {
	ID, OrderID, FileID, ActorID, Action string
	At                                   OrdersTimestamp
}
