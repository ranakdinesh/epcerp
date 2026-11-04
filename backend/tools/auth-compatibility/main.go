// Command auth-compatibility emits synthetic Go credentials for cross-runtime
// migration tests. It never connects to a database or reads deployment secrets.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/security"
	"golang.org/x/crypto/argon2"
)

const secret = "epcerp-synthetic-auth-compatibility-secret-only"
const userID = "10000000-0000-4000-8000-000000000001"

func main() {
	tokens, err := security.NewTokens(security.TokenConfig{Secret: secret, Issuer: "epcERP", Audience: "epcERP-api", AccessLifetime: 15 * time.Minute})
	if err != nil {
		panic(err)
	}
	mail, err := infrastructure.NewEmailCipher(strings.Repeat("11", 32))
	if err != nil {
		panic(err)
	}
	if len(os.Args) > 1 && os.Args[1] == "verify" {
		var in struct{ Token, Password, Hash, Payload, PayloadHash string }
		if err = json.NewDecoder(os.Stdin).Decode(&in); err != nil {
			panic(err)
		}
		id, e := tokens.Subject(in.Token)
		if e != nil || id != userID {
			panic("Next token is incompatible with Go")
		}
		p, e := security.NewPasswords()
		if e != nil || !p.Verify(context.Background(), in.Password, in.Hash) {
			panic("Next password hash is incompatible with Go")
		}
		payload, e := base64.StdEncoding.DecodeString(in.Payload)
		if e != nil {
			panic(e)
		}
		plain, e := mail.Open(payload, in.PayloadHash)
		if e != nil || string(plain) != "synthetic queued credential" {
			panic("Next mail envelope is incompatible with Go")
		}
		fmt.Println("Go accepted Next.js token and password hash")
		return
	}
	password := "Migration-Test1!"
	salt := []byte("synthetic-salt12")
	hashes := []string{}
	for _, variant := range []string{"argon2id", "argon2i"} {
		derive := argon2.IDKey
		if variant == "argon2i" {
			derive = argon2.Key
		}
		hash := derive([]byte(password), salt, 3, 65536, 4, 32)
		hashes = append(hashes, fmt.Sprintf("$%s$v=19$m=65536,t=3,p=4$%s$%s", variant, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)))
	}
	token, _, e := tokens.Issue(userID, time.Now().Add(time.Hour).Unix(), 0)
	if e != nil {
		panic(e)
	}
	payload, digest, e := mail.Seal([]byte("synthetic queued credential"))
	if e != nil {
		panic(e)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"secret": secret, "id": userID, "password": password, "hashes": hashes, "token": token, "mail_payload": base64.StdEncoding.EncodeToString(payload), "mail_hash": digest})
}
