package dtos

import (
	"strings"
	"testing"
)

func TestUsernameChangeIsStrictAndSelfScoped(t *testing.T) {
	for _, body := range []string{`{"username":"newname"}`, `{"username":"newname","expected_username":"oldname","user_id":"someone"}`, `{"username":"newname","expected_username":"oldname","is_superuser":true}`, `{"username":"bad@name","expected_username":"oldname"}`} {
		if _, e := ParseUsernameChange(strings.NewReader(body)); e == nil {
			t.Fatal("invalid username request accepted")
		}
	}
	if _, e := ParseUsernameChange(strings.NewReader(`{"username":"newname","expected_username":"oldname"}`)); e != nil {
		t.Fatal(e)
	}
}
