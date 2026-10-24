package domain

import (
	"testing"
)

func TestPermissionPolicies(t *testing.T) {
	user := User{IsActive: true, Roles: []Role{{IsActive: true, Permissions: []string{"admin.users.read", "admin.users.read"}}, {IsActive: false, Permissions: []string{"admin.users.write"}}}}
	if !HasPermission(user, "admin.users.read") || HasPermission(user, "admin.users.write") || len(EffectivePermissions(user)) != 7 {
		t.Fatal("inactive role or duplicate permission leak")
	}
	user.IsSuperuser = true
	if !HasPermission(user, "future.new.permission") || len(EffectivePermissions(user)) != 8 {
		t.Fatal("recovery override depends on role registry")
	}
	user.IsActive = false
	if HasPermission(user, "future.new.permission") {
		t.Fatal("inactive recovery identity authorized")
	}
}
