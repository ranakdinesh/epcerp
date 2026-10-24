package repository

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	"io/fs"
	"testing"
)

type missingEvidenceObjects struct{ err error }

func (o missingEvidenceObjects) Get(context.Context, string, string) ([]byte, error) {
	return nil, o.err
}
func (o missingEvidenceObjects) Put(context.Context, string, string, []byte) error {
	return errors.New("unexpected write")
}
func TestMissingVendorEvidenceDoesNotAbortRead(t *testing.T) {
	row := db.SupplierDocument{ObjectKey: pgtype.Text{String: "fixture", Valid: true}, ObjectBucket: pgtype.Text{String: "fixture", Valid: true}}
	for _, err := range []error{fs.ErrNotExist, errors.New("storage access denied"), context.DeadlineExceeded} {
		r := VendorDocumentEvidenceReader{objects: missingEvidenceObjects{err}}
		preview, e := r.previewable(context.Background(), row)
		if preview {
			t.Fatal("unavailable content marked readable")
		}
		if errors.Is(err, fs.ErrNotExist) {
			if e != nil {
				t.Fatal(e)
			}
		} else if !errors.Is(e, err) {
			t.Fatal("storage failure was suppressed")
		}
	}
}
