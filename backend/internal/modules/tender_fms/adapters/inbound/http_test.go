package inbound

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"net/http"
	"net/http/httptest"
	"testing"
)

type deniedIdentity struct{}

func (deniedIdentity) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{}, nil
}

type unreadBody struct{ read bool }

func (b *unreadBody) Read(_ []byte) (int, error) {
	b.read = true
	panic("unauthorized body must not be read")
}
func TestUploadGuardAuthorizesBeforeBuffering(t *testing.T) {
	s := application.New(nil, deniedIdentity{}, nil)
	b := &unreadBody{}
	request := httptest.NewRequest("POST", "/api/v1/tender-fms/upload", b)
	response := httptest.NewRecorder()
	called := false
	UploadGuard(s)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(response, request)
	if called || b.read || response.Code != 401 {
		t.Fatal("unauthorized body processed")
	}
}
