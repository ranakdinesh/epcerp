package project_budgets

import (
	"github.com/go-chi/chi/v5"
	"net/http/httptest"
	"testing"
)

func TestNativeOwnerRetiresBudgetHTTP(t *testing.T) {
	t.Setenv("EPCERP_PROJECT_BUDGETS_OWNER", "nextjs")
	r := chi.NewRouter()
	if err := New(r, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "POST"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/api/v1/projects/11111111-1111-4111-8111-111111111111/budget", nil))
		if w.Code != 404 {
			t.Fatal(method, w.Code)
		}
	}
}
func TestInvalidOwnerFailsClosed(t *testing.T) {
	t.Setenv("EPCERP_PROJECT_BUDGETS_OWNER", "invalid")
	if New(chi.NewRouter(), nil, nil) == nil {
		t.Fatal("invalid owner accepted")
	}
}
