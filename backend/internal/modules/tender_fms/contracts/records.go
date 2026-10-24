// Package contracts publishes transaction-bound Tender foundations, not a capture HTTP API.
package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
)

type Record = domain.Record
type NamedInput = domain.NamedInput
type Candidate = domain.Candidate
type ParticipantGrant = domain.ParticipantGrant

// Records must join the caller's shared transaction. CreateNamed is one step of a
// future atomic capture orchestrator (receipt, operation ledger, provisioning and
// notices); it must never be exposed as an independently successful HTTP capture.
type Records interface {
	CreateNamed(context.Context, string, NamedInput) (Record, error)
	Read(context.Context, string, string) (Record, error)
	GrantParticipant(context.Context, string, string, ParticipantGrant) (int64, error)
	ManagerOptions(context.Context, string) ([]Candidate, error)
}

type AssignmentAction = domain.AssignmentAction
type AssignmentActions interface {
	ReadAction(context.Context, string) (AssignmentAction, error)
}
