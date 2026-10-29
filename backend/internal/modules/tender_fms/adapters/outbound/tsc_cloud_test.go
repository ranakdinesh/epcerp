package outbound

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"io"
	"net/http"
	"strings"
	"testing"
)

type cloudTransport func(*http.Request) (*http.Response, error)

func (f cloudTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCloudProvidersUseBoundedFixedOriginsAndExactEvidence(t *testing.T) {
	for _, mode := range []string{"gemini", "openai"} {
		t.Run(mode, func(t *testing.T) {
			p, _ := NewTSCCloud(mode, "synthetic-unusable-key")
			calls := 0
			p.client.Transport = cloudTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Scheme != "https" || r.URL.RawQuery != "" {
					t.Fatal("unsafe origin/key query")
				}
				if mode == "gemini" && r.URL.Host != "generativelanguage.googleapis.com" || mode == "openai" && r.URL.Host != "api.openai.com" {
					t.Fatal("wrong origin")
				}
				body, _ := io.ReadAll(r.Body)
				if len(body) > 32<<10 || !strings.Contains(string(body), "2048") || strings.Contains(string(body), `"tools":`) {
					t.Fatal("unbounded/agentic request")
				}
				answer := `{\"status\":\"answered\",\"answer\":\"EMD INR 5000\",\"citations\":[{\"source_id\":\"S1\"}]}`
				raw := `{"choices":[{"finish_reason":"stop","message":{"content":"` + answer + `"}}]}`
				if mode == "gemini" {
					raw = `{"candidates":[{"finishReason":"STOP","content":{"parts":[{"text":"` + answer + `"}]}}]}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(raw))}, nil
			})
			q := domain.TSCQuestion{ID: "emd.q05", Section: "emd", Text: "EMD amount"}
			passages := []domain.TSCPassage{{ID: "actual-source", Text: "EMD INR 5000 payable online", Filename: "source.pdf", Locator: "Page 1"}}
			a, e := p.Propose(context.Background(), q, passages)
			if e != nil || a.Status != "answered" || len(a.Citations) != 1 || a.Citations[0].SourceID != "actual-source" || a.Citations[0].Quote != passages[0].Text || calls != 1 {
				t.Fatal("invalid response", a, e)
			}
		})
	}
}
func TestCloudRejectsUnknownEvidenceAndNeverRetries(t *testing.T) {
	p, _ := NewTSCCloud("gemini", "synthetic-key")
	calls := 0
	p.client.Transport = cloudTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader(`{"secret":"must not escape"}`))}, nil
	})
	_, e := p.Propose(context.Background(), domain.TSCQuestion{ID: "emd.q05", Section: "emd", Text: "EMD amount"}, []domain.TSCPassage{{Text: "EMD amount 5000"}})
	if e != domain.TSCProviderUnavailable || calls != 1 {
		t.Fatal("unsafe provider retry", e, calls)
	}
	if p, e := NewTSCCloud("arbitrary", "key"); e == nil || p != nil {
		t.Fatal("arbitrary provider accepted")
	}
	if p, _ := NewTSCCloud("openai", ""); p.Model() != "" {
		t.Fatal("unconfigured provider available")
	}
}
