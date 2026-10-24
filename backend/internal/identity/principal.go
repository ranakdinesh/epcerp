// Package identity holds infrastructure-free authenticated identity values.
package identity

// Principal carries the authenticated actor and live permission grants.
type Principal struct {
	ID                string
	Active, Superuser bool
	Permissions       []string
}

func (p Principal) Allows(code string) bool {
	if !p.Active || p.ID == "" {
		return false
	}
	if p.Superuser {
		return true
	}
	for _, grant := range p.Permissions {
		if grant == code {
			return true
		}
	}
	return false
}

// Can is the purchase permission vocabulary for the same active-identity check.
func (p Principal) Can(code string) bool { return p.Allows(code) }
