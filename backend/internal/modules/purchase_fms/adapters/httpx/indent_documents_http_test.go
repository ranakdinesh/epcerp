package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func TestIndentDocumentsDocumentCommandStrictShapeAndRequestBound(t *testing.T) {
	for _, body := range []string{`null`, `[]`, `{"action":"print","version":null}`, `{"action":"send","channel":null}`, `{"action":"generate","invitation_line_ids":[]}`, `{"action":"print"} {}`, `{"action":"print","unexpected":true}`} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		var c domain.IndentDocumentsCommand
		if indentDocumentsDecode(w, r, &c) || w.Code != 422 {
			t.Fatalf("invalid document request accepted: %s (%d)", body, w.Code)
		}
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat(" ", 65<<10)))
	var c domain.IndentDocumentsCommand
	if indentDocumentsDecode(w, r, &c) || w.Code != 413 {
		t.Fatal("oversize document request accepted")
	}
}
