// Parity oracle: offline rules, or read-only purchase gate in an explicitly isolated QA transaction.
package main

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	master "github.com/ranakdinesh/epcERP/internal/modules/master"
	dto "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	repo "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	d "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	"net/url"
	"os"
	"strings"
)

func main() {
	if len(os.Args) > 2 && os.Args[1] == "gate" {
		u, e := url.Parse(os.Getenv("EPCERP_DATABASE_URL"))
		if e != nil || !strings.HasPrefix(u.Path, "/epcerp_auth_qa_") {
			panic("QA database required")
		}
		ctx := context.Background()
		pool, e := pgxpool.New(ctx, u.String())
		if e != nil {
			panic(e)
		}
		defer pool.Close()
		cipher, e := infrastructure.NewDocumentCipher(os.Getenv("EPCERP_SUPPLIER_DOCUMENT_KEY"))
		if e != nil {
			panic(e)
		}
		objects, _, e := master.DocumentObjects()
		if e != nil {
			panic(e)
		}
		out := map[string]any{}
		e = database.NewManager(pool).Within(ctx, func(ctx context.Context, c database.Connection) error {
			g := repo.NewVendorVerificationGate(c, repo.NewVendorDirectory(c), repo.NewVendorDocumentEvidenceReader(c, cipher, infrastructure.NewDocumentInspector(15*1024*1024)).WithObjects(objects))
			source, e := g.Source(ctx, os.Args[2])
			if e != nil {
				return e
			}
			state, e := g.Check(ctx, os.Args[2])
			out["source"] = source
			out["eligibility"] = state
			return e
		})
		if e != nil {
			panic(e)
		}
		_ = json.NewEncoder(os.Stdout).Encode(out)
		return
	}
	var inputs []struct {
		Snapshot d.VendorVerificationSnapshot
		Review   *d.VendorVerificationReview
		Command  *d.VendorVerificationCommand
	}
	if e := json.NewDecoder(os.Stdin).Decode(&inputs); e != nil {
		panic(e)
	}
	out := []map[string]any{}
	for _, in := range inputs {
		hash := repo.VendorVerificationValues{}.Fingerprint(in.Snapshot)
		r := map[string]any{"fingerprint": hash, "value": dto.TransportValue(d.VendorVerificationEvaluate(in.Snapshot, in.Review, hash))}
		if in.Command != nil {
			if e := d.VendorVerificationValidate(in.Snapshot, *in.Command); e != nil {
				r["error"] = e.Error()
				if v, ok := e.(*d.Error); ok {
					r["field"] = v.Field
					r["error"] = v.Detail
				}
			}
		}
		out = append(out, r)
	}
	_ = json.NewEncoder(os.Stdout).Encode(out)
}
