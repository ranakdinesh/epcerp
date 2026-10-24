package domain

import (
	"testing"
)

func TestDocumentFilenameAndRetryPolicy(t *testing.T) {
	name, format, e := Filename(`C:\fakepath\tender.XLSX`)
	if e != nil || name != "tender.XLSX" || format != "xlsx" {
		t.Fatal("filename normalization", e)
	}
	for _, name := range []string{"macro.xlsm", "image.exe", "bad\n.pdf"} {
		if _, _, e = Filename(name); e == nil {
			t.Fatal("unsupported name", name)
		}
	}
	if e = Retry(Document{Version: 2, Status: "failed", Attempts: 2}, 1); e != Conflict {
		t.Fatal("stale version")
	}
	if e = Retry(Document{Version: 2, Status: "failed", Attempts: 3}, 2); e == nil {
		t.Fatal("exhausted retry")
	}
	if e = Retry(Document{Version: 2, Status: "failed", Attempts: 2}, 2); e != nil {
		t.Fatal(e)
	}
}
