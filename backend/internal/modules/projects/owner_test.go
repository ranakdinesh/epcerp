package projects

import (
	"github.com/go-chi/chi/v5"
	"net/http/httptest"
	"testing"
)

func TestNativeProjectOwnerRetiresGoHTTP(t *testing.T) {
	t.Setenv("EPCERP_PROJECTS_OWNER", "nextjs")
	r := chi.NewRouter()
	if _, e := New(Dependencies{Router: r}); e != nil {
		t.Fatal(e)
	}
	for _, v := range []struct{ method, path string }{{"GET", ""}, {"POST", ""}, {"POST", "/schedule-preview"}, {"GET", "/assignment-options"}, {"GET", "/11111111-1111-4111-8111-111111111111"}, {"PATCH", "/11111111-1111-4111-8111-111111111111"}, {"PATCH", "/11111111-1111-4111-8111-111111111111/manager"}, {"GET", "/11111111-1111-4111-8111-111111111111/metadata-history"}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(v.method, "/api/v1/projects"+v.path, nil))
		if w.Code != 404 {
			t.Fatalf("Go project route registered: %s %d", v.path, w.Code)
		}
	}
}
func TestInvalidProjectOwnerFailsClosed(t *testing.T) {
	t.Setenv("EPCERP_PROJECTS_OWNER", "invalid")
	if _, e := New(Dependencies{}); e == nil {
		t.Fatal("invalid owner accepted")
	}
}
