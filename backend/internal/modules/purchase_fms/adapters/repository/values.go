package repository

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"golang.org/x/text/cases"
)

type Values struct{}

func (Values) ID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic("random source unavailable")
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
func (Values) Now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

func (Values) NameKey(s string) string { return cases.Fold().String(strings.TrimSpace(s)) }
