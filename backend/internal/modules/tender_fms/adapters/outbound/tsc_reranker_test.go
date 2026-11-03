package outbound

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPrivateRerankerOnlySendsCandidateTextToFixedOrigin(t *testing.T) {
	p := NewTSCReranker(nil)
	calls := 0
	p.client.Transport = cloudTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "http://tsc-reranker:8080/rerank" || r.Header.Get("Authorization") != "" {
			t.Fatal("unsafe origin/credentials")
		}
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "secret-filename") || strings.Contains(string(raw), "privatehash") {
			t.Fatal("unneeded source metadata leaked")
		}
		body, _ := json.Marshal(map[string]any{"model": p.Model(), "scores": []domain.TSCRank{{ID: "s1", Score: 1}}})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})
	out, e := p.Rank(context.Background(), domain.TSCQuestion{Text: "EMD amount"}, []domain.TSCPassage{{ID: "s1", Text: "Earnest money INR 5000", Filename: "secret-filename", SHA256: "privatehash"}})
	if e != nil || len(out) != 1 || calls != 1 {
		t.Fatal("ranking failed", e)
	}
}
func TestRerankerRejectsWrongModelAndDoesNotRetry(t *testing.T) {
	p := NewTSCReranker(nil)
	calls := 0
	p.client.Transport = cloudTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"model":"unreviewed","scores":[]}`))}, nil
	})
	if _, e := p.Rank(context.Background(), domain.TSCQuestion{Text: "EMD"}, []domain.TSCPassage{{ID: "s1", Text: "EMD"}}); e == nil || calls != 1 {
		t.Fatal("model drift accepted or request retried")
	}
}

// Opt-in execution in an isolated Docker network against the real pinned model.
func TestTSCRerankerRealCPU(t *testing.T) {
	if os.Getenv("EPCERP_TSC_CPU_SMOKE") != "1" {
		t.Skip("requires the private local reranker")
	}
	q := domain.TSCQuestion{ID: "emd.q05", Section: "emd", Text: "What is the earnest money deposit amount?"}
	passages := []domain.TSCPassage{{ID: "emd", DocumentID: "true", Text: "Earnest money deposit of INR 500000 is required with the bid."}}
	for i := 0; i < 20; i++ {
		passages = append(passages, domain.TSCPassage{ID: fmt.Sprint(i), DocumentID: fmt.Sprint(i), Text: strings.Repeat("Tender document fee payment details. ", 35) + fmt.Sprintf("Clause %d: earnest money deposit is a separate requirement.", i)})
	}
	ranker := NewTSCReranker(nil)
	selected := application.NewTSCEvidence(ranker).Select(context.Background(), q, passages)
	found := false
	for _, p := range selected.Passages {
		if p.ID == "emd" {
			found = true
		}
	}
	if !found || selected.Diagnostic.Method != "bm25_minilm" || selected.Diagnostic.Fallback != "" || selected.Diagnostic.ContextGap || selected.Diagnostic.SelectedBytes >= selected.Diagnostic.BaselineBytes {
		t.Fatal("real local selection failed", selected.Diagnostic)
	}
	t.Logf("Synthetic CPU selection: candidates=%d baseline_bytes=%d selected_bytes=%d; not real-tender accuracy or token billing", selected.Diagnostic.Candidates, selected.Diagnostic.BaselineBytes, selected.Diagnostic.SelectedBytes)
}
func TestRankerDeadlineOpensCircuitAndAllowsBoundedRecovery(t *testing.T) {
	p := NewTSCReranker(nil)
	p.timeout = 10 * time.Millisecond
	now := time.Now()
	p.now = func() time.Time { return now }
	calls := 0
	p.client.Transport = cloudTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	q := domain.TSCQuestion{Text: "EMD amount"}
	passages := []domain.TSCPassage{{ID: "x", Text: "EMD INR 5000"}}
	start := time.Now()
	if _, e := p.Rank(context.Background(), q, passages); e == nil {
		t.Fatal("hung inference accepted")
	}
	for i := 0; i < 70; i++ {
		if _, e := p.Rank(context.Background(), q, passages); e == nil {
			t.Fatal("open circuit accepted inference")
		}
	}
	if calls != 1 || time.Since(start) > time.Second {
		t.Fatal("repeated per-question timeout", calls)
	}
	now = now.Add(61 * time.Second)
	p.client.Transport = cloudTransport(func(*http.Request) (*http.Response, error) {
		calls++
		raw := fmt.Sprintf(`{"model":%q,"scores":[{"id":"x","score":1}]}`, p.Model())
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(raw))}, nil
	})
	if _, e := p.Rank(context.Background(), q, passages); e != nil || calls != 2 {
		t.Fatal("circuit did not recover", e, calls)
	}
}
