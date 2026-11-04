package personal_tasks

import (
	"github.com/go-chi/chi/v5"
	"net/http/httptest"
	"testing"
)

func TestNativeOwnerRetiresGoTasks(t *testing.T) {
	t.Setenv("EPCERP_PERSONAL_TASKS_OWNER", "nextjs")
	r := chi.NewRouter()
	if e := New(Dependencies{Router: r}); e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/my-tasks", nil))
	if w.Code != 404 {
		t.Fatal("Go task route still active", w.Code)
	}
}
func TestInvalidOwnerFailsClosed(t *testing.T) {
	t.Setenv("EPCERP_PERSONAL_TASKS_OWNER", "invalid")
	if e := New(Dependencies{}); e == nil {
		t.Fatal("invalid owner accepted")
	}
}
