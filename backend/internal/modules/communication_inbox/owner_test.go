package communication_inbox

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNativeInboxRetiresGoHTTP(t *testing.T) {
	t.Setenv("EPCERP_INBOX_OWNER", "nextjs")
	r := chi.NewRouter()
	if e := New(r, nil, nil); e != nil {
		t.Fatal(e)
	}
	for _, v := range []struct{ method, path string }{{"GET", "/api/v1/inbox"}, {"GET", "/api/v1/inbox/11111111-1111-4111-8111-111111111111"}, {"POST", "/api/v1/inbox/11111111-1111-4111-8111-111111111111/read"}, {"POST", "/api/v1/inbox/11111111-1111-4111-8111-111111111111/acknowledge"}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(v.method, v.path, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("Go Inbox route still registered: %s %d", v.path, w.Code)
		}
	}
}
func TestInvalidInboxOwnerFailsClosed(t *testing.T) {
	t.Setenv("EPCERP_INBOX_OWNER", "other")
	if e := New(chi.NewRouter(), nil, nil); e == nil {
		t.Fatal("invalid owner accepted")
	}
}
