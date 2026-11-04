// Offline compatibility oracle. It only processes supplied synthetic bytes.
package main

import (
	"encoding/json"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"os"
)

func main() {
	var in struct {
		Operation string
		Content   []byte
		Hash      string
	}
	if e := json.NewDecoder(os.Stdin).Decode(&in); e != nil {
		panic(e)
	}
	cipher, e := infrastructure.NewDocumentCipher(os.Getenv("EPCERP_SUPPLIER_DOCUMENT_KEY"))
	if e != nil {
		panic(e)
	}
	out := map[string]any{}
	switch in.Operation {
	case "seal":
		encrypted, hash, e := cipher.Seal(in.Content)
		if e != nil {
			panic(e)
		}
		out["content"] = encrypted
		out["hash"] = hash
	case "open":
		plain, e := cipher.Open(in.Content, in.Hash)
		if e != nil {
			out["error"] = e.Error()
		} else {
			out["content"] = plain
		}
	case "inspect":
		kind, e := infrastructure.NewDocumentInspector(15 * 1024 * 1024).Inspect(in.Content)
		out["kind"] = kind
		if e != nil {
			out["error"] = e.Error()
		}
	}
	if e := json.NewEncoder(os.Stdout).Encode(out); e != nil {
		panic(e)
	}
}
