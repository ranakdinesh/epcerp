package domain

import (
	"testing"
)

func TestDocumentPoliciesAndFilenameParity(t *testing.T) {
	for input, want := range map[string]string{"../../../secret.pdf": "secret.pdf", "safe_file-name.v1.pdf": "safe_file-name.v1.pdf", ".hidden": "document_hidden", "bad:name*with?illegal<>chars|.pdf": "bad_name_with_illegal__chars_.pdf", "": "document_", "/": "document_", "दस्तावेज़.pdf": "दस_त_व_ज_.pdf"} {
		if got := SanitizeFilename(input); got != want {
			t.Fatalf("filename parity: %q => %q want %q", input, got, want)
		}
	}
	valid := Upload{EntityType: "supplier", Title: "Fixture", Filename: "test.pdf", DocType: "pan", ContentType: "application/pdf", Data: []byte("payload")}
	if err := Validate(valid); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Upload){func(r *Upload) { r.Data = nil }, func(r *Upload) { r.Data = make([]byte, MaximumDocumentBytes+1) }, func(r *Upload) { r.DocType = "invalid" }, func(r *Upload) { r.ContentType = "application/x-msdownload" }, func(r *Upload) { r.EntityType = "" }} {
		request := valid
		change(&request)
		if Validate(request) == nil {
			t.Fatal("invalid upload accepted")
		}
	}
	if (Principal{Active: false, Superuser: true}).Allows("documents.storage.read") {
		t.Fatal("inactive recovery user allowed")
	}
	if !(Principal{Active: true, Permissions: []string{"documents.storage.read"}}).Allows("documents.storage.read") {
		t.Fatal("registered permission denied")
	}
}
