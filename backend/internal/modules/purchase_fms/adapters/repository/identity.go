package repository

import (
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
)

type AuthBridge = infrastructure.AuthBridge

func NewAuthBridge(r auth.Resolver) *infrastructure.AuthBridge {
	return infrastructure.NewAuthBridge(r)
}
