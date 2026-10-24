package domain

// ErrorKind describes a business failure without an HTTP status.
type ErrorKind string

const (
	InternalFailure ErrorKind = "internal_failure"
	Unauthenticated ErrorKind = "unauthenticated"
	AccessDenied    ErrorKind = "access_denied"
	NotFound        ErrorKind = "not_found"
	Conflict        ErrorKind = "conflict"
	InvalidInput    ErrorKind = "invalid_input"
)

type Error struct {
	Kind          ErrorKind
	Detail, Field string
}

func (e *Error) Error() string { return e.Detail }
