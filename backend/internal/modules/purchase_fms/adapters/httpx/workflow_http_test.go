package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func TestWorkflowStrictPackageCommands(t *testing.T) {
	for _, body := range []string{`null`, `{"version":1,"reason":"r","rows":[],"actor_id":"forged"}`, `{"version":1,"reason":"r","rows":[{"number":5,"actor_id":"forged"}]}`, `{} {}`, strings.Repeat(" ", 64<<10) + `{}`} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		w := httptest.NewRecorder()
		var q domain.WorkflowAssign
		if workflowDecode(w, r, &q) {
			t.Fatal("unsafe package command accepted")
		}
		if w.Code != 422 && w.Code != 413 {
			t.Fatal(w.Code)
		}
	}
}
