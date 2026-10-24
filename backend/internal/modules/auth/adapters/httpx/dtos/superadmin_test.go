package dtos

import (
	"strings"
	"testing"
)

func TestCreateSuperadminBooleanAndRoleContract(t *testing.T) {
	base := `"first_name":"Fixture","last_name":"Admin","mobile":"9000000000","username":"fixture","email":"fixture@example.invalid"`
	for _, tc := range []struct {
		tail      string
		ok, super bool
	}{{`,"role_names":[],"is_superuser":true`, true, true}, {`,"role_names":["employee"]`, true, false}, {`,"role_names":[],"is_superuser":false`, false, false}, {`,"role_names":[],"is_superuser":"true"`, false, false}, {`,"role_names":[],"is_superuser":null`, false, false}, {`,"role_names":[],"is_superuser":1`, false, false}} {
		v, e := ParseCreateUser(strings.NewReader("{" + base + tc.tail + "}"))
		if (e == nil) != tc.ok || e == nil && v.IsSuperuser != tc.super {
			t.Fatalf("contract %s: %v", tc.tail, e)
		}
	}
}
