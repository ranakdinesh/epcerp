package inbound

import (
	"bytes"
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/application"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/domain"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

type deniedIdentity struct{}

func (deniedIdentity) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{}, nil
}
func form(fields map[string]string, filename string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for name, value := range fields {
		_ = writer.WriteField(name, value)
	}
	part, _ := writer.CreateFormFile("file", filename)
	_, _ = part.Write([]byte("fixture"))
	_ = writer.Close()
	return body, writer.FormDataContentType()
}
func TestMultipartContractAndAnonymousDenial(t *testing.T) {
	fields := map[string]string{"entity_type": "supplier", "entity_id": "12345678-1234-4234-8234-123456789abc", "doc_type": "pan"}
	body, contentType := form(fields, "../../../secret.pdf")
	request := httptest.NewRequest("POST", "/", body)
	request.Header.Set("Content-Type", contentType)
	parsed, ok := parseUpload(httptest.NewRecorder(), request)
	if !ok || parsed.Filename != "../../../secret.pdf" || parsed.Title != parsed.Filename || parsed.DocType != "pan" || string(parsed.Data) != "fixture" {
		t.Fatal("multipart translation drift")
	}
	router := chi.NewRouter()
	Register(router, application.New(nil, deniedIdentity{}, nil, nil, nil, ""))
	body, contentType = form(fields, "secret.pdf")
	request = httptest.NewRequest("POST", "/api/v1/documents/upload", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != 401 || response.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatal("anonymous upload allowed")
	}
	for _, change := range []func(map[string]string){func(f map[string]string) { delete(f, "entity_id") }, func(f map[string]string) { f["entity_id"] = "not-an-id" }, func(f map[string]string) { f["unknown"] = "private-canary" }, func(f map[string]string) { f["title"] = strings.Repeat("x", 4097) }} {
		current := map[string]string{}
		for k, v := range fields {
			current[k] = v
		}
		change(current)
		body, contentType = form(current, "secret.pdf")
		request = httptest.NewRequest("POST", "/", body)
		request.Header.Set("Content-Type", contentType)
		response = httptest.NewRecorder()
		_, ok := parseUpload(response, request)
		if ok || (response.Code != 422 && response.Code != 413) || strings.Contains(response.Body.String(), "private-canary") {
			t.Fatal("invalid form accepted or echoed")
		}
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("POST", "/api/v1/documents", nil))
	if response.Code != 405 {
		t.Fatal("unexpected method allowed")
	}
}
