package security

import "crypto/rand"

type TemporaryPasswords struct{}

func (TemporaryPasswords) Generate() (string, error) { return "Ep9!" + rand.Text(), nil }
