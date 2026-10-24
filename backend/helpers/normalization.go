// Package helpers contains pure text helpers shared by the Go backend.
package helpers

import (
	"strings"
	"unicode"
)

// Fold uses Go Unicode simple-fold equivalence without multi-character expansions.
// In particular, ß remains distinct from ss. Importers must use this same name key.
func Fold(s string) string {
	return strings.Map(func(r rune) rune {
		canonical := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < canonical {
				canonical = next
			}
		}
		return unicode.ToLower(canonical)
	}, s)
}
func Lower(s string) string { return strings.ToLower(s) }
func Upper(s string) string { return strings.ToUpper(s) }
