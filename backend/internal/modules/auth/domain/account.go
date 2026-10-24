package domain

import (
	"unicode"
	"unicode/utf8"
)

func StrongPassword(s string) bool {
	n := utf8.RuneCountInString(s)
	if n < 8 || n > 128 {
		return false
	}
	lower, upper, digit, symbol := false, false, false, false
	for _, r := range s {
		lower = lower || unicode.IsLower(r)
		upper = upper || unicode.IsUpper(r)
		digit = digit || unicode.IsDigit(r)
		symbol = symbol || (!unicode.IsLetter(r) && !unicode.IsNumber(r))
	}
	return lower && upper && digit && symbol
}
