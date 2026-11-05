package project_access

import (
	"github.com/go-chi/chi/v5"
	"net/http/httptest"
	"testing"
)

func TestNativeOwnerRetiresTeamHTTP(t *testing.T) {
	t.Setenv("EPCERP_PROJECT_TEAM_OWNER", "nextjs")
	r := chi.NewRouter()
	if e := New(r, nil, nil); e != nil {
		t.Fatal(e)
	}
	for _, v := range []struct{ method, path string }{{"GET", ""}, {"POST", ""}, {"GET", "/options"}, {"PATCH", "/22222222-2222-4222-8222-222222222222"}, {"GET", "/22222222-2222-4222-8222-222222222222/history"}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(v.method, "/api/v1/projects/11111111-1111-4111-8111-111111111111/responsibilities"+v.path, nil))
		if w.Code != 404 {
			t.Fatalf("Go team route retained: %s %d", v.path, w.Code)
		}
	}
}
func TestInvalidTeamOwnerFailsClosed(t *testing.T) {
	t.Setenv("EPCERP_PROJECT_TEAM_OWNER", "invalid")
	if New(nil, nil, nil) == nil {
		t.Fatal("Invalid owner accepted")
	}
}
