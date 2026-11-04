// Synthetic file-format oracle; never reads deployment configuration or connects to PostgreSQL.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	p "github.com/ranakdinesh/epcERP/internal/modules/document_packages/adapters/outbound"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/domain"
	"io"
	"os"
)

func main() {
	var in struct {
		Operation, Root, Key, Object, Hash string
		Data                               []byte
		Size                               int64
		Files                              []domain.Declaration
	}
	if e := json.NewDecoder(os.Stdin).Decode(&in); e != nil {
		panic(e)
	}
	out := map[string]any{}
	if in.Operation == "fingerprint" {
		hash, e := (p.Values{}).Fingerprint(in.Files)
		if e != nil {
			panic(e)
		}
		out["hash"] = hash
	} else {
		cipher, e := infrastructure.NewDocumentCipher(in.Key)
		if e != nil {
			panic(e)
		}
		store, e := infrastructure.NewDocumentStreamStorage(in.Root, cipher)
		if e != nil {
			panic(e)
		}
		defer store.Close()
		if in.Operation == "write" {
			e = store.Upload(context.Background(), in.Object, bytes.NewReader(in.Data), in.Size, in.Hash)
		} else if in.Operation == "read" {
			e = store.WithPlain(context.Background(), in.Object, in.Size, in.Hash, func(f *os.File) error { b, err := io.ReadAll(f); out["data"] = b; return err })
		} else {
			panic("unsupported oracle operation")
		}
		if e != nil {
			out["error"] = e.Error()
		}
	}
	if e := json.NewEncoder(os.Stdout).Encode(out); e != nil {
		panic(fmt.Errorf("oracle output: %w", e))
	}
}
