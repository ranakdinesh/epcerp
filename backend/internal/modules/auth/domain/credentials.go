package domain

import "strings"

type CredentialEmail struct{ ID, UserID, Recipient, Subject, Message string }
type EmailJob struct {
	ID, LeaseID string
	Email       CredentialEmail
	Valid       bool
}

func CredentialMessage(username, password, loginURL string, reset bool) (string, string) {
	subject := "Welcome to epcERP — your account is ready"
	opening := "Your Vishal Technopower epcERP account has been created."
	if reset {
		subject = "Your epcERP temporary password"
		opening = "An authorised administrator has reset your epcERP password."
	}
	return subject, "Hello " + username + ",\n\n" + opening + "\n\nUsername: " + username + "\nTemporary password: " + password + "\n\n" + loginURL + "\n\nSign in within 24 hours. You must choose a new password before accessing your workspace. This temporary password replaces any earlier password.\n\nFor assistance, contact your administrator.\n\nRegards,\nVishal Technopower"
}
func ValidResetReason(s string) bool { n := len(strings.TrimSpace(s)); return n >= 10 && n <= 1000 }
func (u User) TemporaryValid(now int64) bool {
	return !u.MustChangePassword || (u.TemporaryExpiresAt != nil && u.TemporaryExpiresAt.Unix() > now)
}
