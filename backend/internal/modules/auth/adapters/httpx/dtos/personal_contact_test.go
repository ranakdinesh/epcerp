package dtos

import (
	"strings"
	"testing"
)

func TestPersonalContactRequestCannotTargetPrivileges(t *testing.T) {
	for _, input := range []string{
		`{"display_name":"Name","user_id":"another"}`,
		`{"display_name":"Name","contact":{"first_name":"A","last_name":"B","mobile":"9876543210","is_superuser":true},"expected_contact":{},"version":0}`,
		`{"display_name":"Name","contact":{"first_name":"A","last_name":"B","mobile":"9876543210"}}`,
		`{"display_name":"Name","is_active":false}`,
		`{"display_name":"Name","department_id":"another"}`,
	} {
		if _, e := ParseProfileUpdate(strings.NewReader(input)); e == nil {
			t.Fatal("unsafe or incomplete request accepted")
		}
	}
}
