package dtos

import (
	"strings"
	"testing"
)

func TestUpdateUserContact(t *testing.T) {
	v, err := ParseUpdateUser(strings.NewReader(`{"first_name":"  Asha ","last_name":" Shah ","mobile":" +91 90000 00000 ","email":"ASHA@example.invalid","role_names":["employee"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if *v.FirstName != "Asha" || *v.LastName != "Shah" || *v.Mobile != "+91 90000 00000" || *v.Email != "asha@example.invalid" {
		t.Fatalf("contact was not normalized: %+v", v)
	}
	for _, body := range []string{`{}`, `{"email":"a@example.invalid"}`, `{"role_names":["employee"]}`} {
		v, err := ParseUpdateUser(strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if v.FirstName != nil || v.LastName != nil || v.Mobile != nil {
			t.Fatalf("omitted contact must stay unchanged: %s", body)
		}
	}
}

func TestUpdateUserRejectsInvalidContactAndAccountChanges(t *testing.T) {
	for _, body := range []string{
		`{"first_name":" "}`, `{"last_name":null}`, `{"mobile":null}`, `{"first_name":42}`,
		`{"last_name":"bad\nname"}`, `{"first_name":"` + strings.Repeat("a", 101) + `"}`,
		`{"mobile":"1234567"}`, `{"mobile":"1234567890123456"}`, `{"mobile":"90000abc00"}`,
		`{"is_active":false}`, `{"is_active":true}`, `{"is_superuser":true}`, `{"username":"another"}`,
	} {
		t.Run(body, func(t *testing.T) {
			if _, err := ParseUpdateUser(strings.NewReader(body)); err == nil {
				t.Fatal("invalid update accepted")
			}
		})
	}
}

func TestUserStatusChangeContract(t *testing.T) {
	for _, body := range []string{`{"is_active":false,"reason":"Access revoked for departure"}`, `{"is_active":true,"reason":"Approved account reactivation"}`} {
		if _, err := ParseUserStatus(strings.NewReader(body)); err != nil {
			t.Fatal(err)
		}
	}
	for _, body := range []string{`{}`, `{"is_active":null,"reason":"Valid reason here"}`, `{"is_active":true}`, `{"is_active":false,"reason":"short"}`, `{"is_active":true,"reason":"Valid reason here","role_names":[]}`} {
		if _, err := ParseUserStatus(strings.NewReader(body)); err == nil {
			t.Fatalf("invalid status accepted: %s", body)
		}
	}
}
