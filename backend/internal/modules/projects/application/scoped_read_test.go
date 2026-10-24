package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/ports"
	"testing"
)

type scopedReadStub struct {
	ports.Repository
	allow bool
}

func (s scopedReadStub) ListReadable(context.Context, domain.Filter, domain.Principal) (domain.Page, error) {
	return domain.Page{}, nil
}
func (s scopedReadStub) CanReadAssigned(context.Context, domain.Principal, string) (bool, error) {
	return s.allow, nil
}
func TestAssignedReadDoesNotCreateOwnershipOrWriteAuthority(t *testing.T) {
	a := domain.Principal{ID: "finance", Active: true, Permissions: []string{"projects.master.read"}}
	p := domain.Project{ID: "project", ManagerID: "another"}
	for _, allow := range []bool{false, true} {
		got, e := canReadProject(t.Context(), scopedReadStub{allow: allow}, a, p)
		if e != nil || got != allow {
			t.Fatal(got, e)
		}
	}
	if domain.CanRead(a, p) || a.Allows("projects.master.write") {
		t.Fatal("specialist became manager/writer")
	}
}
