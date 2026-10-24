// Package document_reading exposes reusable document-reading composition.
package document_reading

import (
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/adapters/outbound"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/application"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

type Module struct {
	database *pgxpool.Pool
	cipher   *outbound.Cipher
	events   func(database.Connection) events.Writer
}

func New(pool *pgxpool.Pool, writers func(database.Connection) events.Writer) (*Module, error) {
	cipher, err := outbound.NewCipher(os.Getenv("EPCERP_DOCUMENT_READING_KEY"))
	if err != nil {
		return nil, err
	}
	return &Module{database: pool, cipher: cipher, events: writers}, nil
}

func (m *Module) Reader(connection database.Connection) contracts.Reader {
	return application.New(outbound.NewRepository(connection, m.events(connection)), m.cipher, outbound.Values{}, outbound.Inspector{}, outbound.Codec{})
}

func (*Module) Validator() contracts.Validator { return outbound.Inspector{} }

func (m *Module) Preparer() contracts.Preparer {
	return application.New(nil, m.cipher, outbound.Values{}, outbound.Inspector{}, outbound.Codec{})
}
func (m *Module) PreparedReader(c database.Connection) contracts.PreparedReader {
	return application.New(outbound.NewRepository(c, m.events(c)), m.cipher, outbound.Values{}, outbound.Inspector{}, outbound.Codec{})
}
