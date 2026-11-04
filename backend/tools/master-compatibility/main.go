// Command master-compatibility exports Go's persisted name-key semantics for
// migration tests. It reads no credentials and never connects to a database.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"unicode"

	"github.com/ranakdinesh/epcERP/helpers"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "validate" {
		var inputs []json.RawMessage
		if err := json.NewDecoder(os.Stdin).Decode(&inputs); err != nil {
			panic(err)
		}
		results := []map[string]any{}
		for _, raw := range inputs {
			in, field, err := dtos.ParseClient(bytes.NewReader(raw))
			out := map[string]any{"valid": err == nil, "field": field}
			if err == nil {
				var c domain.Client
				if err = domain.ClientPrepare(in, &c); err != nil {
					out["valid"] = false
					out["field"] = "contacts"
				} else {
					out["legal_name"] = c.LegalName
					out["display_name"] = c.DisplayName
					out["normalized_name"] = c.NormalizedName
					out["website"] = c.Website
					out["gstin"] = c.Gstin
					out["pan"] = c.Pan
					out["country"] = c.Country
					if len(in.Contacts) > 0 {
						out["email"] = in.Contacts[0].Email
						out["contact_type"] = in.Contacts[0].Type
					}
				}
			}
			results = append(results, out)
		}
		_ = json.NewEncoder(os.Stdout).Encode(results)
		return
	}
	pairs := [][4]int32{}
	for r := rune(0); r <= unicode.MaxRune; r++ {
		folded := []rune(helpers.Fold(string(r)))
		if len(folded) == 1 && (folded[0] != r || unicode.ToLower(r) != r || unicode.ToUpper(r) != r) {
			pairs = append(pairs, [4]int32{r, folded[0], unicode.ToLower(r), unicode.ToUpper(r)})
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(pairs)
}
