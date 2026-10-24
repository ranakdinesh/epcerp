package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type IndentGroupsRecord = domain.IndentGroupsRecord
type IndentGroupsReader interface {
	Get(context.Context, string, string) (*IndentGroupsRecord, error)
}

type IndentGroupsActor = domain.Principal
type IndentGroupsCommand = domain.IndentGroupsCommand
type IndentGroupsWorkspace = domain.IndentGroupsWorkspace

// Commander may only run on its consumer's active shared transaction.
type IndentGroupsCommander interface {
	Execute(context.Context, IndentGroupsActor, string, string, IndentGroupsCommand) (IndentGroupsWorkspace, error)
}

type IndentGroupsError = domain.IndentGroupsError
