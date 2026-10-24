package domain

import "testing"

func TestUsernameFormatAndTakenMessage(t *testing.T) {
	for _, s := range []string{"ab", "has space", "user@example.invalid", "<name>", "नया"} {
		if _, e := Username(s); e == nil {
			t.Fatal("invalid username accepted")
		}
	}
	if s, e := Username("  user.name-1_2  "); e != nil || s != "user.name-1_2" {
		t.Fatal("valid username rejected", e)
	}
	if UsernameTaken("occupied").Error() != "occupied already exists. Please choose a different username." {
		t.Fatal("duplicate message mismatch")
	}
}
