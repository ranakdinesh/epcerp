package outbound

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
	"time"
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
func (Values) Now() time.Time { return time.Now().UTC() }

type AuthBridge struct{ r auth.Resolver }

func NewAuthBridge(r auth.Resolver) *AuthBridge { return &AuthBridge{r} }
func (a *AuthBridge) Resolve(ctx context.Context, t string) (domain.Actor, error) {
	p, e := a.r.Resolve(ctx, t)
	return domain.Actor{ID: p.ID, Active: p.Active, Superuser: p.Superuser, Permissions: p.Permissions}, e
}
