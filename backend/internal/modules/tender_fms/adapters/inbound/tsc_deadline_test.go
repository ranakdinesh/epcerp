package inbound

import (
	"github.com/go-chi/chi/v5/middleware"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResearchResponseSurvivesShortGlobalWriteDeadline(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Production request logging wraps the response writer in this same adapter.
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		request, cancel, err := boundedTSCResearch(wrapped, r)
		if err != nil {
			t.Error(err)
			return
		}
		defer cancel()
		deadline, ok := request.Context().Deadline()
		if !ok || time.Until(deadline) > 90*time.Second {
			t.Error("research has no bounded context")
		}
		time.Sleep(40 * time.Millisecond)
		wrapped.Write([]byte("verified synthetic proposal"))
	}))
	server.Config.WriteTimeout = 10 * time.Millisecond
	server.Start()
	defer server.Close()
	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatal("research lost to global write timeout", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || string(body) != "verified synthetic proposal" {
		t.Fatal("research response lost", err)
	}
}
func TestResearchContextCancellationAndRecorder(t *testing.T) {
	r := httptest.NewRequest("POST", "/research", nil)
	bounded, cancel, e := boundedTSCResearch(httptest.NewRecorder(), r)
	if e != nil {
		t.Fatal(e)
	}
	cancel()
	select {
	case <-bounded.Context().Done():
	default:
		t.Fatal("research context did not cancel")
	}
}
