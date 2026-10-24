package identity

import "testing"

func TestPrincipalPermissionAuthority(t *testing.T) {
	for _, check := range []struct {
		name      string
		principal Principal
		code      string
		want      bool
	}{
		{"explicit permission", Principal{ID: "actor", Active: true, Permissions: []string{"settings.clients.write"}}, "settings.clients.write", true},
		{"missing permission", Principal{ID: "actor", Active: true, Permissions: []string{"settings.clients.read"}}, "settings.clients.write", false},
		{"recovery administrator", Principal{ID: "actor", Active: true, Superuser: true}, "future.permission", true},
		{"inactive administrator", Principal{ID: "actor", Superuser: true}, "settings.clients.write", false},
		{"missing actor", Principal{Active: true, Superuser: true}, "settings.clients.write", false},
	} {
		t.Run(check.name, func(t *testing.T) {
			if got := check.principal.Allows(check.code); got != check.want {
				t.Fatal("unexpected permission authority", got)
			}
		})
	}
}
