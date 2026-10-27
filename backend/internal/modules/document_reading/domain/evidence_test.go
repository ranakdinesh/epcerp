package domain

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestEvidenceRetainsSourceOffsetsAndSuppressesOnlyFurniture(t *testing.T) {
	r := Result{}
	for i := 0; i < 4; i++ {
		r.Segments = append(r.Segments, Segment{Kind: "page", Label: "Page", Method: "native", Text: "SYNTHETIC TENDER\n\n1. Eligibility\nEMD: INR 10000. Except eligible bidders under clause 9.\n\n2. Payment\nPayment  90%  on delivery\nRetention  10%  after acceptance\n\nPage 1"})
	}
	e := BuildEvidence(r)
	if e.Segments != 4 || e.Suppressed < 3 {
		t.Fatalf("furniture not recognised: %+v", e)
	}
	foundTable := false
	foundException := false
	for _, b := range e.Blocks {
		if b.Text != r.Segments[b.Segment].Text[b.Start:b.End] || len(b.Text) > 1800 || !utf8.ValidString(b.Text) {
			t.Fatal("source span lost")
		}
		if b.Kind == "table" {
			foundTable = true
		}
		if strings.Contains(b.Text, "Except") {
			foundException = true
			if b.Suppressed {
				t.Fatal("business condition suppressed")
			}
		}
	}
	if !foundTable || !foundException {
		t.Fatal("table or condition lost")
	}
}

func TestEvidencePreservesUTF8AndNativeAlternative(t *testing.T) {
	low := 35.0
	text := strings.Repeat("विद्युत कार्य ₹1000; no exemption. ", 120)
	r := Result{Segments: []Segment{{Kind: "page", Label: "Page 1", Text: text, NativeText: "Employer: Synthetic Works Department", Method: "ocr", Confidence: &low}}}
	e := BuildEvidence(r)
	var original strings.Builder
	alternative := false
	for _, b := range e.Blocks {
		if !utf8.ValidString(b.Text) || len(b.Text) > 1800 {
			t.Fatal("invalid UTF8 split")
		}
		if b.Alternative {
			alternative = true
			if b.Method != "native_alternative" {
				t.Fatal("alternative method lost")
			}
		} else {
			original.WriteString(b.Text)
		}
	}
	if original.String() != text || !alternative || e.LowConfidence != 1 {
		t.Fatal("content or alternative dropped")
	}
}

func TestNativeChoiceRequiresSubstantialCleanTextAndWeakOCR(t *testing.T) {
	low, high := 30.0, 95.0
	native := strings.Repeat("Synthetic native tender clause with valid conditions. ", 8)
	if !PreferNativeText(native, native, &low) || PreferNativeText("Page 1", native, &low) || PreferNativeText(native, native, &high) {
		t.Fatal("native/OCR routing incorrect")
	}
}

func TestStandaloneAmountAtPageMarginIsNotPageFurniture(t *testing.T) {
	e := BuildEvidence(Result{Segments: []Segment{{Kind: "page", Text: "10000\n\nEMD amount stated above."}}})
	for _, b := range e.Blocks {
		if strings.Contains(b.Text, "10000") && b.Suppressed {
			t.Fatal("amount treated as a page number")
		}
	}
}
