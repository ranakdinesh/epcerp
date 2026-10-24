package domain

// Empty means unclassified, including previously imported contacts.
func ClientValidContactType(value string) bool {
	switch value {
	case "", "EIC", "HOD", "SITE_CONTACT":
		return true
	}
	return false
}

// Coverage counts active contacts, independently of primary-contact designation.
func ClientMissingContactTypes(contacts []ClientContact) []string {
	found := map[string]bool{}
	for _, contact := range contacts {
		if contact.Active {
			found[contact.Type] = true
		}
	}
	missing := []string{}
	for _, kind := range []string{"EIC", "HOD", "SITE_CONTACT"} {
		if !found[kind] {
			missing = append(missing, kind)
		}
	}
	return missing
}
