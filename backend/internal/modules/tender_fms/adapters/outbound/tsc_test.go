package outbound

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTSCExportPreservesAllQuestionsAndCitesEvidence(t *testing.T) {
	in, e := TSCRegistry()
	if e != nil {
		t.Fatal(e)
	}
	in.Name = "SYNTHETIC TSC <fixture>"
	in.Passages = []domain.TSCPassage{{ID: "source-1", DocumentID: "fixture-doc", Filename: "fixture.pdf", Locator: "page 1", Text: "Synthetic evidence only", SHA256: strings.Repeat("a", 64), Revision: 1}}
	job := domain.TSCJob{ID: "fixture", DraftVersion: 1, Version: 3, Input: in, Result: domain.TSCResult{Schema: domain.TSCVersion, Warnings: in.Warnings}}
	for _, q := range in.Questions {
		job.Result.Answers = append(job.Result.Answers, domain.EmptyTSCAnswer(q))
	}
	job.Result.Answers[0] = domain.TSCAnswer{QuestionID: in.Questions[0].ID, Status: "answered", Text: "Synthetic evidence only", Citations: []domain.TSCCitation{{SourceID: "source-1", Quote: "Synthetic evidence only"}}}
	data, e := (TSCExporter{}).Export(job)
	if e != nil {
		t.Fatal(e)
	}
	reader, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		t.Fatal(e)
	}
	var document string
	for _, f := range reader.File {
		if f.Name == "word/document.xml" {
			r, e := f.Open()
			if e != nil {
				t.Fatal(e)
			}
			raw, e := io.ReadAll(r)
			r.Close()
			if e != nil {
				t.Fatal(e)
			}
			document = string(raw)
			decoder := xml.NewDecoder(bytes.NewReader(raw))
			for {
				_, e := decoder.Token()
				if e == io.EOF {
					break
				}
				if e != nil {
					t.Fatal("invalid generated XML", e)
				}
			}
		}
	}
	rows := tscRows.FindAllString(document, -1)
	for i, q := range in.Questions {
		if q.Row >= len(rows) || !strings.Contains(rows[q.Row], tscParagraph(tscChecklistAnswer(job.Result.Answers[i]))) {
			t.Fatal("question answer omitted", q.ID)
		}
	}
	for _, s := range []string{"DRAFT", "Synthetic evidence only"} {
		if !strings.Contains(document, s) {
			t.Fatal("missing export evidence", s)
		}
	}
	for _, s := range []string{"Not found", "Needs review", "Conflicting information", "answered:", "Draft evidence appendix", "fixture.pdf", "page 1", "[particulars.q01]", "SHA256"} {
		if strings.Contains(document, s) {
			t.Fatal("review metadata leaked into checklist", s)
		}
	}
	if dir := os.Getenv("EPCERP_TSC_EXPORT_PROOF"); dir != "" {
		if e = os.WriteFile(filepath.Join(dir, "synthetic-tsc-draft.docx"), data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	job.Input.TemplateHash = "changed"
	if _, e = (TSCExporter{}).Export(job); e == nil {
		t.Fatal("wrong template accepted")
	}
}

func TestChecklistAnswerPresentation(t *testing.T) {
	for _, tc := range []struct {
		answer domain.TSCAnswer
		want   string
	}{
		{domain.TSCAnswer{Status: "answered", Text: "Answered: 90 days"}, "90 days"},
		{domain.TSCAnswer{Status: "answered", Text: "answered"}, ""},
		{domain.TSCAnswer{Status: "answered", Text: "Answered:"}, ""},
		{domain.TSCAnswer{Status: "answered", Text: "Yes"}, "Yes"},
		{domain.TSCAnswer{Status: "not_found", Text: "Long retrieval explanation and source area"}, ""},
		{domain.TSCAnswer{Status: "needs_review", Text: "Unreviewed decision"}, ""},
		{domain.TSCAnswer{Status: "conflicting", Text: "Two incompatible source values"}, ""},
	} {
		if got := tscChecklistAnswer(tc.answer); got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
	}
}
func TestTSCProviderRejectsExternalOriginsAndRetrievalIsBounded(t *testing.T) {
	for _, origin := range []string{"https://api.example.com", "http://example.com", "http://user:password@localhost:11434", "http://localhost:11434/path", "http://localhost:11434?token=x"} {
		if _, e := NewTSCOllama(origin, "fixture"); e == nil {
			t.Fatal("external/noncanonical provider accepted")
		}
	}
	rows := []domain.TSCPassage{}
	for i := 0; i < 20; i++ {
		rows = append(rows, domain.TSCPassage{Text: "Earnest money deposit surety exemption"})
	}
	out := TSCRetrieve(domain.TSCQuestion{Section: "emd", Text: "EMD exemption"}, rows)
	if len(out) != 1 {
		t.Fatal("retrieval bound", len(out))
	}
}

func TestTSCOllamaOnlySendsEvidenceToPinnedLocalWeights(t *testing.T) {
	digest := strings.Repeat("a", 64)
	var remote atomic.Bool
	var chats atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/tags" {
			host := ""
			if remote.Load() {
				host = "https://ollama.com"
			}
			json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{{"name": "fixture:local", "digest": digest, "size": 2000000, "remote_host": host}}})
			return
		}
		if r.URL.Path != "/api/chat" {
			t.Error("unexpected provider path")
			w.WriteHeader(404)
			return
		}
		chats.Add(1)
		raw, _ := io.ReadAll(r.Body)
		if !bytes.Contains(raw, []byte("Synthetic tender evidence")) {
			t.Error("missing selected source")
		}
		answer := `{"status":"answered","answer":"Synthetic tender evidence","citations":[{"source_id":"S1"}]}`
		json.NewEncoder(w).Encode(map[string]any{"done": true, "done_reason": "stop", "message": map[string]string{"content": answer}})
	}))
	defer server.Close()
	provider, e := NewTSCOllama(server.URL, "fixture:local", digest)
	if e != nil {
		t.Fatal(e)
	}
	q := domain.TSCQuestion{ID: "particulars.q01", Text: "Tender evidence department", Section: "particulars"}
	passages := []domain.TSCPassage{{ID: "s1", Text: "Synthetic tender evidence"}}
	if a, e := provider.Propose(context.Background(), q, passages); e != nil || a.Status != "answered" || chats.Load() != 1 || len(a.Citations) != 1 || a.Citations[0].SourceID != "s1" || a.Citations[0].Quote != passages[0].Text {
		t.Fatal("valid local model", a, e, chats.Load())
	}
	remote.Store(true)
	if _, e = provider.Propose(context.Background(), q, passages); e == nil || chats.Load() != 1 {
		t.Fatal("remote-routed model received evidence", e, chats.Load())
	}
	provider.digest = strings.Repeat("b", 64)
	remote.Store(false)
	if _, e = provider.Propose(context.Background(), q, passages); e == nil || chats.Load() != 1 {
		t.Fatal("changed weights received evidence", e, chats.Load())
	}
}

func TestTSCRetrievalPrioritizesQuestionOverSectionKeywords(t *testing.T) {
	rows := []domain.TSCPassage{}
	for i := 0; i < 10; i++ {
		rows = append(rows, domain.TSCPassage{ID: "broad", Text: "tender employer location completion portal auction deadline submission validity"})
	}
	rows = append(rows, domain.TSCPassage{ID: "specific", Text: "Department: Synthetic Works"})
	selected := TSCRetrieve(domain.TSCQuestion{Section: "particulars", Text: "Name of Department:"}, rows)
	if len(selected) != 2 || selected[0].ID != "specific" {
		t.Fatal("broad section keywords displaced question evidence")
	}
}

func TestTSCOllamaRejectsUnknownReferencesAndGeneratedQuotes(t *testing.T) {
	for _, content := range []string{
		`{"status":"answered","answer":"Invented answer","citations":[{"source_id":"S99"}]}`,
		`{"status":"answered","answer":"Invented answer","citations":[{"source_id":"S1","quote":"Invented quotation"}]}`,
		`{"status":"answered","answer":"Unsupported answer","citations":[]}`,
	} {
		t.Run(content, func(t *testing.T) {
			digest := strings.Repeat("a", 64)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/tags" {
					json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{{"name": "fixture:local", "digest": digest, "size": 2000000}}})
					return
				}
				json.NewEncoder(w).Encode(map[string]any{"done": true, "done_reason": "stop", "message": map[string]string{"content": content}})
			}))
			defer server.Close()
			provider, err := NewTSCOllama(server.URL, "fixture:local", digest)
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.Propose(context.Background(), domain.TSCQuestion{ID: "particulars.q01", Text: "Synthetic department", Section: "particulars"}, []domain.TSCPassage{{ID: "original-id", Text: "Synthetic department evidence"}})
			if err != domain.TSCProviderInvalidProposal {
				t.Fatalf("unexpected error category: %v", err)
			}
		})
	}
}
