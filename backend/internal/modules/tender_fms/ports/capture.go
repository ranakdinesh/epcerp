package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
)

type RecordFoundation interface {
	CreateNamed(context.Context, string, domain.NamedInput) (domain.Record, error)
	Read(context.Context, string, string) (domain.Record, error)
}
type CaptureSources interface {
	Consume(context.Context, string, string, int64, string) (domain.PackageRef, error)
	MemberIDs(context.Context, domain.PackageRef) ([]string, error)
	Queue(context.Context, domain.PackageRef, string, string, string) (string, error)
}
type AssignmentNotices interface {
	Deliver(context.Context, domain.AssignmentAction) (string, error)
}
type CaptureRepository interface {
	Guard(context.Context) error
	LockOperation(context.Context, domain.CaptureOperation) error
	Operation(context.Context, domain.CaptureOperation) (*domain.CaptureOperation, error)
	SaveOperation(context.Context, domain.CaptureOperation) error
	Bind(context.Context, domain.Record, domain.CaptureInput, string) error
	LinkSource(context.Context, string, string, string, string) error
	Projection(context.Context, domain.Record) (domain.Captured, error)
	ManagerWork(context.Context, domain.Record, string) (domain.AssignmentAction, error)
	LinkNotice(context.Context, string, string) error
	ChangeManager(context.Context, domain.Record, domain.ChangeManager, string) (domain.Record, error)
	Patch(context.Context, domain.Record, domain.Patch, string) (domain.Record, error)
	List(context.Context, string, bool, int32, int32, string, string, bool) ([]string, int64, error)
}
type CaptureUnit interface {
	Within(context.Context, func(context.Context, CaptureRepository, RecordFoundation, CaptureSources, EligibleUsers, AssignmentNotices) error) error
}
type CommandValues interface {
	Values
	Fingerprint(any) (string, error)
}

type CommandLimits interface{ Allow(string, string) bool }
