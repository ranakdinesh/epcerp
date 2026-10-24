package domain

import (
	"testing"
)

func TestClientContactCoverage(t *testing.T) {
	contacts := []ClientContact{{Type: "EIC", Active: true}, {Type: "EIC", Active: true}, {Type: "HOD", Active: true}, {Type: "SITE_CONTACT", Active: false}, {Active: true}}
	if got := ClientMissingContactTypes(contacts); len(got) != 1 || got[0] != "SITE_CONTACT" {
		t.Fatal(got)
	}
	contacts = append(contacts, ClientContact{Type: "SITE_CONTACT", Active: true})
	if len(ClientMissingContactTypes(contacts)) != 0 {
		t.Fatal("complete coverage flagged")
	}
	if len(ClientMissingContactTypes(nil)) != 3 {
		t.Fatal("missing coverage not flagged")
	}
}

func TestClientContactsDoNotBlockCreation(t *testing.T) {
	for _, contacts := range [][]ClientContact{nil, {{Type: "EIC"}, {Type: "EIC"}}, {{Type: ""}}} {
		if e := ClientPrepare(ClientInput{Contacts: contacts}, &Client{}); e != nil {
			t.Fatal(e)
		}
	}
	if ClientPrepare(ClientInput{Contacts: []ClientContact{{Type: "DIRECTOR"}}}, &Client{}) == nil {
		t.Fatal("unknown type accepted")
	}
}
