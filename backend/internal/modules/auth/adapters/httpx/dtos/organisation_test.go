package dtos

import (
	"strings"
	"testing"
)

func TestEmployeeIDsAndStrictHandover(t *testing.T) {
	employee, e := ParseEmployee(strings.NewReader(`{"first_name":"Test","last_name":"Employee","mobile":"+91 9000000000","department_id":"ABCDEF00-0000-4000-8000-000000000001","manager_id":"ABCDEF00-0000-4000-8000-000000000002","is_company_root":false,"version":0,"reason":"Initial reporting setup"}`))
	if e != nil || employee.ManagerID != "abcdef00-0000-4000-8000-000000000002" || employee.DepartmentID != "abcdef00-0000-4000-8000-000000000001" {
		t.Fatal("canonical reporting identifiers", e)
	}
	_, e = ParseHandover(strings.NewReader(`{"version":1,"reason":"Manager reviewed handover","selections":[{"source":"auth","kind":"reporting_manager","id":"ref","replacement_id":"","approved":true}]}`))
	if e == nil {
		t.Fatal("caller supplied approval accepted")
	}
	_, e = ParseHandover(strings.NewReader(`{"version":1,"reason":"Manager reviewed handover","actor_id":"forged"}`))
	if e == nil {
		t.Fatal("caller supplied actor accepted")
	}
}
