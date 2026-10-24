package domain

import (
	"strings"
	"testing"
)

func TestAccountPasswordPolicy(t *testing.T) {
	if !StrongPassword("Abcd123!") {
		t.Fatal("eight-character password rejected")
	}
	if StrongPassword("Abc123!") {
		t.Fatal("seven-character password accepted")
	}
	if !StrongPassword(strings.Repeat("å", 110) + "Aa1!") {
		t.Fatal("valid Unicode password rejected by byte count")
	}
	for _, value := range []string{strings.Repeat("a", 20), "Aa1!", strings.Repeat("a", 125) + "Aa1!"} {
		if StrongPassword(value) {
			t.Fatal("invalid password accepted")
		}
	}
}
