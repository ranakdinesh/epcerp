// Synthetic parity oracle. Database access is restricted to an explicitly isolated QA database.
package main

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	ar "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	aa "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	inbox "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox"
	er "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	ea "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	tender "github.com/ranakdinesh/epcERP/internal/modules/tender_fms"
	tr "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound"
	td "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"net/url"
	"os"
	"strings"
)

func main() {
	var in struct {
		Operation, Actor, Tender, Action string
		Capture                          td.CaptureInput
		Patch                            td.Patch
		Change                           td.ChangeManager
	}
	if e := json.NewDecoder(os.Stdin).Decode(&in); e != nil {
		panic(e)
	}
	out := map[string]any{}
	if in.Operation == "fingerprint" {
		a, e := (tr.Values{}).Fingerprint(in.Capture)
		if e != nil {
			panic(e)
		}
		out["capture"] = a
		a, e = (tr.Values{}).Fingerprint(in.Patch)
		if e != nil {
			panic(e)
		}
		out["patch"] = a
		a, e = (tr.Values{}).Fingerprint(in.Change)
		if e != nil {
			panic(e)
		}
		out["manager"] = a
	} else {
		u, e := url.Parse(os.Getenv("EPCERP_DATABASE_URL"))
		if e != nil || !strings.HasPrefix(u.Path, "/epcerp_auth_qa_register_") {
			panic("isolated register QA database required")
		}
		ctx := context.Background()
		p, e := pgxpool.New(ctx, u.String())
		if e != nil {
			panic(e)
		}
		defer p.Close()
		e = database.NewManager(p).Within(ctx, func(ctx context.Context, c database.Connection) error {
			users := aa.NewAssignmentDirectory(ar.NewAssignmentReader(c))
			bundles := ar.NewBundleDirectory(c)
			journal := ea.NewRecorder(er.NewJournal(c))
			records := tender.Records(c, users, bundles, strings.Split(os.Getenv("EPCERP_TENDER_MANAGER_BUNDLE_IDS"), ","), journal)
			switch in.Operation {
			case "read":
				r, e := records.Read(ctx, in.Actor, in.Tender)
				out["record"] = r
				return e
			case "options":
				r, e := records.ManagerOptions(ctx, in.Actor)
				out["items"] = r
				return e
			case "notice":
				r, e := inbox.TenderWriter(c, journal, tender.AssignmentActions(c, journal)).TenderAssignment(ctx, in.Action)
				out["notice"] = r
				return e
			}
			panic("unsupported operation")
		})
		if e != nil {
			out["error"] = e.Error()
		}
	}
	if e := json.NewEncoder(os.Stdout).Encode(out); e != nil {
		panic(e)
	}
}
