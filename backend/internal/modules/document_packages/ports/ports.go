package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/domain"
)

type Repository interface {
	ReadyDrafts(context.Context) ([]domain.Receipt, error)
	AddDisposition(context.Context, domain.Receipt, domain.Disposition) error
	Dispositions(context.Context, domain.Receipt) ([]domain.Disposition, error)
	MemberInfo(context.Context, domain.Receipt, string) (domain.Member, error)
	Guard(context.Context) error
	PurgeCandidates(context.Context, string) ([]domain.ObjectRef, error)
	FinishPurge(context.Context, domain.ObjectRef) error
	Actor(context.Context, string) (*domain.Actor, error)
	LockOperation(context.Context, domain.Operation) error
	Operation(context.Context, domain.Operation) (*domain.Operation, error)
	SaveOperation(context.Context, domain.Operation) error
	Create(context.Context, string, string, []domain.Declaration) error
	Get(context.Context, string) (domain.Receipt, error)
	File(context.Context, string, string) (domain.File, error)
	BeginUpload(context.Context, domain.File, string, string) error
	FinishUpload(context.Context, domain.File, string, string) error
	AbortUpload(context.Context, domain.File, string) error
	Seal(context.Context, domain.Receipt) error
	Claim(context.Context, string) (*domain.Receipt, error)
	AddMember(context.Context, domain.Receipt, domain.Member) error
	FinishVerification(context.Context, domain.Receipt, string) error
	RetainDraft(context.Context, string, string, string) error
	DraftReceipts(context.Context, string) ([]domain.Receipt, error)
	PublishDraft(context.Context, domain.Receipt, string) error
	Consume(context.Context, domain.Receipt, string, string) error
	MemberIDs(context.Context, domain.Receipt) ([]string, error)
	Member(context.Context, domain.Receipt, string) (domain.Member, error)
}
type Unit interface {
	Within(context.Context, func(context.Context, Repository) error) error
}
type Auth interface {
	Subject(context.Context, string) (string, error)
}
type Values interface {
	ID() string
	Fingerprint(any) (string, error)
}
type Objects interface {
	Original(context.Context, string, int64, string) ([]byte, error)
	Remove(context.Context, string) error
	Upload(context.Context, string, domain.ByteStream, int64, string) error
}
type Verifier interface {
	Verify(context.Context, domain.Verification, func(domain.Member) error) error
}

type RequestLimits interface{ Allow(string, string) bool }

type LeaseKeeper interface {
	KeepClaim(context.Context, domain.Receipt) (context.Context, func())
}

type ManifestVerifier interface {
	VerifyManifest(context.Context, domain.Verification, func(domain.Member) error, func(domain.Disposition) error) error
}
