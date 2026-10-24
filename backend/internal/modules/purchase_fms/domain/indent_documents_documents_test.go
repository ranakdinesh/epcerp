package domain

import (
	"testing"
	"time"
)

func TestIndentDocumentsDispatchDatesUseGenerationCalendarAndRetainEvidence(t *testing.T) {
	generated := time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC)
	now := time.Date(2026, 10, 10, 18, 0, 0, 0, time.UTC)
	d := IndentDocumentsData{GeneratedAt: generated}
	for _, day := range []string{"2026-10-09", "2026-10-12", "2026-02-30", "2026-1-01"} {
		if IndentDocumentsValidateDispatch(d, day, now) == nil {
			t.Fatalf("accepted invalid dispatch %s", day)
		}
	}
	if e := IndentDocumentsValidateDispatch(d, "2026-10-10", now); e != nil {
		t.Fatal(e)
	}
	sent := "2026-10-10"
	d.SentDate = &sent
	if IndentDocumentsValidateDispatch(d, sent, now) == nil {
		t.Fatal("dispatch rewritten")
	}
}
func TestIndentDocumentsDocumentCommandRejectsMissingTargetsAndDispatchEvidence(t *testing.T) {
	id := "12345678-1234-4234-8234-123456789012"
	for _, c := range []IndentDocumentsCommand{{Action: "generate"}, {Action: "print"}, {Action: "send", DocumentID: &id}, {Action: "unexpected"}, {Action: "print", DocumentID: &id, Version: -1}} {
		if _, e := c.Validate(); e == nil {
			t.Fatalf("invalid command accepted: %s", c.Action)
		}
	}
	if IndentDocumentsNumber(id) != "IND-12345678123442348234123456789012" {
		t.Fatal("source numbering changed")
	}
}
