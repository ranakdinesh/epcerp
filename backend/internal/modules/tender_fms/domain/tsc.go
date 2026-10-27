package domain

import (
	"strings"
	"time"
)

const TSCVersion = "tsc-v1"
const TSCProposalVersion = "tsc-structured-retrieval-v3"

// Only reviewed, non-sensitive categories cross the provider boundary.
type TSCProviderError string

func (e TSCProviderError) Error() string { return string(e) }

const (
	TSCProviderUnavailable      TSCProviderError = "provider_unavailable"
	TSCProviderTimeout          TSCProviderError = "provider_timeout"
	TSCProviderModelUnavailable TSCProviderError = "provider_model_unavailable"
	TSCProviderInvalidProposal  TSCProviderError = "invalid_provider_evidence"
)

func TSCProviderFailure(err error) string {
	switch err {
	case TSCProviderUnavailable, TSCProviderTimeout, TSCProviderModelUnavailable:
		return err.Error()
	default:
		return "provider_failed"
	}
}

type TSCQuestion struct {
	ID      string `json:"id"`
	Section string `json:"section"`
	Number  string `json:"printed_number"`
	Text    string `json:"question"`
	Row     int    `json:"row"`
}
type TSCPassage struct {
	ID           string   `json:"id"`
	DocumentID   string   `json:"document_id"`
	Revision     int32    `json:"revision"`
	SHA256       string   `json:"sha256"`
	Filename     string   `json:"filename"`
	Locator      string   `json:"locator"`
	Text         string   `json:"text"`
	SearchText   string   `json:"search_text,omitempty"`
	Heading      string   `json:"heading,omitempty"`
	Kind         string   `json:"kind,omitempty"`
	Method       string   `json:"method,omitempty"`
	Confidence   *float64 `json:"confidence,omitempty"`
	Segment      int      `json:"segment,omitempty"`
	Start        int      `json:"start,omitempty"`
	End          int      `json:"end,omitempty"`
	Alternative  bool     `json:"alternative,omitempty"`
	Suppressed   bool     `json:"suppressed,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	DocumentType string   `json:"document_type,omitempty"`
}

type TSCCoverage struct {
	Version       string `json:"version"`
	Documents     int    `json:"documents"`
	Segments      int    `json:"segments"`
	EmptySegments int    `json:"empty_segments"`
	Blocks        int    `json:"blocks"`
	Suppressed    int    `json:"suppressed"`
	LowConfidence int    `json:"low_confidence"`
}
type TSCCitation struct {
	SourceID string `json:"source_id"`
	Quote    string `json:"quote"`
}
type TSCAnswer struct {
	Accepted   bool          `json:"accepted"`
	QuestionID string        `json:"question_id"`
	Status     string        `json:"status"`
	Text       string        `json:"answer"`
	Citations  []TSCCitation `json:"citations"`
	Origin     string        `json:"origin"`
}
type TSCInput struct {
	Schema               string              `json:"schema"`
	ProposalVersion      string              `json:"proposal_version,omitempty"`
	Coverage             TSCCoverage         `json:"coverage"`
	TemplateHash         string              `json:"template_hash"`
	OriginalTemplateHash string              `json:"original_template_hash"`
	Name                 string              `json:"name"`
	Questions            []TSCQuestion       `json:"questions"`
	Passages             []TSCPassage        `json:"passages"`
	Warnings             []string            `json:"warnings"`
	Sources              []TSCSnapshotSource `json:"sources"`
	Parts                []TSCSourcePart     `json:"parts"`
}
type TSCSnapshotSource struct {
	BatchID string      `json:"batch_id"`
	Source  DraftSource `json:"source"`
}
type TSCSourcePart struct {
	DocumentID  string `json:"document_id"`
	Path        string `json:"path"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason"`
}

func ValidateTSCSources(d DocumentDraft) error {
	if len(d.Batches) == 0 {
		return Invalid("Upload and finish reading a source package first")
	}
	for _, b := range d.Batches {
		if b.State != "consumed" || len(b.Sources) == 0 {
			return Invalid("Every retained batch must finish verification and publish its readings first")
		}
		for _, s := range b.Sources {
			if s.Status != "ready" {
				return Invalid("Every published source must finish reading first")
			}
		}
	}
	return nil
}

type TSCResult struct {
	Schema      string                  `json:"schema"`
	Answers     []TSCAnswer             `json:"answers"`
	Warnings    []string                `json:"warnings"`
	Diagnostics []TSCQuestionDiagnostic `json:"diagnostics,omitempty"`
}

type TSCQuestionDiagnostic struct {
	QuestionID string `json:"question_id"`
	Candidates int    `json:"candidates"`
	Documents  int    `json:"documents"`
	Outcome    string `json:"outcome"`
}
type TSCJob struct {
	ID           string    `json:"id"`
	DraftID      string    `json:"draft_id"`
	DraftVersion int64     `json:"draft_version"`
	ActorID      string    `json:"-"`
	ClaimID      string    `json:"-"`
	Mode         string    `json:"mode"`
	Model        string    `json:"model"`
	State        string    `json:"state"`
	Error        string    `json:"error"`
	Version      int64     `json:"version"`
	Attempts     int32     `json:"attempts"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Input        TSCInput  `json:"input"`
	Result       TSCResult `json:"result"`
	Stale        bool      `json:"stale"`
}

func ValidateTSCAnswer(a TSCAnswer, in TSCInput) error {
	found := false
	for _, q := range in.Questions {
		if q.ID == a.QuestionID {
			found = true
		}
	}
	if !found {
		return Invalid("Unknown TSC question")
	}
	if a.Status != "answered" && a.Status != "not_found" && a.Status != "needs_review" && a.Status != "conflicting" {
		return Invalid("Invalid draft answer status")
	}
	if len(a.Text) > 8000 || (a.Status == "answered" && TSCPlaceholder(a.Text)) || len(a.Citations) > 8 {
		return Invalid("Provide a bounded answer and at most eight citations")
	}
	if a.Status == "answered" && len(a.Citations) == 0 {
		return Invalid("An answered question requires source evidence")
	}
	if a.Accepted && a.Status != "answered" {
		return Invalid("Only a supported answer can be accepted; resolve this question first")
	}
	if (strings.HasPrefix(a.QuestionID, "conclusion.") || a.QuestionID == "work_location.q01" || a.QuestionID == "risk.q01" || a.QuestionID == "risk.q02" || a.QuestionID == "risk.q03") && a.Status != "needs_review" {
		return Invalid("This question requires reviewed company/risk evidence and an authorised human decision")
	}
	for _, c := range a.Citations {
		if len(c.Quote) < 5 || len(c.Quote) > 2000 {
			return Invalid("Citations require an exact excerpt of 5 to 2000 bytes")
		}
		supported := false
		for _, p := range in.Passages {
			if p.ID == c.SourceID && strings.Contains(p.Text, c.Quote) {
				supported = true
				break
			}
		}
		if !supported {
			return Invalid("Citation does not match the snapshotted source text")
		}
	}
	return nil
}

func EmptyTSCAnswer(q TSCQuestion) TSCAnswer {
	a := TSCAnswer{QuestionID: q.ID, Status: "needs_review", Text: "", Citations: []TSCCitation{}, Origin: "manual_draft"}
	if TSCRequiresHumanReview(q) {
		a.Status = "needs_review"
		a.Text = ""
	}
	return a
}

func SameTSCAnswerContent(a, b TSCAnswer) bool {
	if a.QuestionID != b.QuestionID || a.Status != b.Status || a.Text != b.Text || len(a.Citations) != len(b.Citations) {
		return false
	}
	for i, c := range a.Citations {
		if c != b.Citations[i] {
			return false
		}
	}
	return true
}

func TSCRequiresHumanReview(q TSCQuestion) bool {
	return strings.HasPrefix(q.ID, "conclusion.") || q.Section == "work_location" || q.ID == "work_location.q01" || q.ID == "risk.q01" || q.ID == "risk.q02" || q.ID == "risk.q03"
}
