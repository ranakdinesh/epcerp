package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
)

type Input = domain.Input
type Document = domain.Document
type Event = domain.Event
type Error = domain.Error
type Validator interface {
	Validate(string, []byte) (string, string, error)
}
type Reader interface {
	Submit(context.Context, Input) (Document, error)
	Get(context.Context, string, string, string) (Document, error)
	List(context.Context, string, string) ([]Document, error)
	Artifact(context.Context, string, string, string, string) ([]byte, error)
	Retry(context.Context, string, string, string, int32, string) (Document, error)
	History(context.Context, string, string, string) ([]Event, error)
}

type Prepared = domain.Prepared
type Preparer interface {
	Prepare(string, []byte) (Prepared, error)
}
type PreparedReader interface {
	SubmitPrepared(context.Context, Input, Prepared) (Document, error)
}
