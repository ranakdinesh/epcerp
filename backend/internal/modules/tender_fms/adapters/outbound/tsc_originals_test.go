package outbound

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"io"
	"net/http"
	"strings"
	"testing"
)

func originalFixture(id string) domain.TSCOriginal {
	b := []byte("%PDF-synthetic-" + id)
	h := sha256.Sum256(b)
	return domain.TSCOriginal{ID: id, Filename: id + ".pdf", Pages: 1, Bytes: len(b), SHA256: hex.EncodeToString(h[:]), Data: b}
}
func originalsResponse(alias string, page int, quote string) string {
	a, _ := json.Marshal(map[string]any{"status": "answered", "answer": "INR 2500", "citations": []map[string]any{{"document": alias, "page": page, "quote": quote}}})
	out, _ := json.Marshal(map[string]any{"candidates": []map[string]any{{"finishReason": "STOP", "content": map[string]any{"parts": []map[string]string{{"text": string(a)}}}}}})
	return string(out)
}
func TestOriginalPDFsAreCompleteBoundedAndSourceVerified(t *testing.T) {
	p, _ := NewTSCCloud("gemini", "synthetic-api-key")
	calls := 0
	p.client.Transport = cloudTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		raw, _ := io.ReadAll(r.Body)
		if r.URL.Host != "generativelanguage.googleapis.com" || r.URL.RawQuery != "" || r.Header.Get("x-goog-api-key") != "synthetic-api-key" || strings.Contains(string(raw), "synthetic-api-key") {
			t.Fatal("unsafe origin/credential placement")
		}
		var body map[string]any
		if json.Unmarshal(raw, &body) != nil {
			t.Fatal("bad body")
		}
		if calls == 1 {
			req := body["generateContentRequest"].(map[string]any)
			contents := req["contents"].([]any)
			parts := contents[0].(map[string]any)["parts"].([]any)
			pdfs := 0
			for _, v := range parts {
				if v.(map[string]any)["inlineData"] != nil {
					pdfs++
				}
			}
			if pdfs != 2 {
				t.Fatal("attachment silently omitted", pdfs)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"totalTokens":1000}`))}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(originalsResponse("D2", 1, "Earnest money INR 2500")))}, nil
	})
	q := domain.TSCQuestion{ID: "emd.q05", Text: "EMD amount"}
	a, e := p.ProposeOriginals(context.Background(), q, []domain.TSCOriginal{originalFixture("first"), originalFixture("second")}, []domain.TSCPassage{{ID: "actual", DocumentID: "second", Segment: 0, Text: "Earnest money INR 2500 payable online."}})
	if e != nil || calls != 2 || len(a.Citations) != 1 || a.Citations[0].SourceID != "actual" || a.Accepted {
		t.Fatal("unverified answer", e, a, calls)
	}
}
func TestOriginalsRejectBadHashBeforeNetwork(t *testing.T) {
	p, _ := NewTSCCloud("gemini", "synthetic-key")
	doc := originalFixture("x")
	doc.SHA256 = strings.Repeat("0", 64)
	p.client.Transport = cloudTransport(func(*http.Request) (*http.Response, error) { t.Fatal("bad manifest sent"); return nil, nil })
	if _, e := p.ProposeOriginals(context.Background(), domain.TSCQuestion{}, []domain.TSCOriginal{doc}, nil); e == nil {
		t.Fatal("bad checksum accepted")
	}
}
func TestOriginalTokenLimitPreventsGeneration(t *testing.T) {
	p, _ := NewTSCCloud("gemini", "synthetic-key")
	calls := 0
	p.client.Transport = cloudTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if !strings.HasSuffix(r.URL.Path, ":countTokens") {
			t.Fatal("over-limit generation sent")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"totalTokens":999999}`))}, nil
	})
	if _, e := p.ProposeOriginals(context.Background(), domain.TSCQuestion{}, []domain.TSCOriginal{originalFixture("x")}, nil); e == nil || calls != 1 {
		t.Fatal("token bound ignored", e, calls)
	}
}
func TestOriginalsRejectInventedDocumentsPagesAndQuotes(t *testing.T) {
	for _, c := range []struct {
		alias string
		page  int
		quote string
	}{{"D9", 1, "Earnest money INR 2500"}, {"D1", 99, "Earnest money INR 2500"}, {"D1", 1, "Invented private fact"}} {
		t.Run(c.alias, func(t *testing.T) {
			p, _ := NewTSCCloud("gemini", "synthetic-key")
			n := 0
			p.client.Transport = cloudTransport(func(*http.Request) (*http.Response, error) {
				n++
				s := `{"totalTokens":100}`
				if n == 2 {
					s = originalsResponse(c.alias, c.page, c.quote)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(s))}, nil
			})
			if _, e := p.ProposeOriginals(context.Background(), domain.TSCQuestion{ID: "emd.q05"}, []domain.TSCOriginal{originalFixture("x")}, []domain.TSCPassage{{ID: "s", DocumentID: "x", Text: "Earnest money INR 2500"}}); e == nil {
				t.Fatal("fabricated source accepted")
			}
		})
	}
}
