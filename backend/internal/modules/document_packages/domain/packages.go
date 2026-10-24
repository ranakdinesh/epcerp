package domain

import (
	"github.com/ranakdinesh/epcERP/internal/identity"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const LeafBytes int64 = 32 << 20
const ZipBytes int64 = 64 << 20
const PackageBytes int64 = 250 << 20
const MaxMembers = 100
const SafeVersion int64 = 9007199254740991

type Error struct {
	Status       int
	Code, Detail string
}

func (e *Error) Error() string                   { return e.Detail }
func Fail(status int, code, detail string) error { return &Error{status, code, detail} }

var Missing = Fail(404, "not_found", "Document receipt not found")
var Stale = Fail(409, "stale_version", "Receipt or file changed; refresh before continuing")
var Forbidden = Fail(403, "forbidden", "Document receipt permission denied")
var Unauthorized = Fail(401, "unauthenticated", "Invalid or expired credentials")
var Unavailable = Fail(503, "dependency_unavailable", "Document verification dependency is unavailable")
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func UUID(s string) error {
	if !uuidPattern.MatchString(s) {
		return Fail(422, "validation_failed", "Use a canonical UUID")
	}
	return nil
}
func Hash(s string) bool { return hashPattern.MatchString(s) }

type Actor struct {
	Principal identity.Principal
	Ready     bool
}

func (a Actor) Authorize(write bool) error {
	if !a.Principal.Active || a.Principal.ID == "" {
		return Unauthorized
	}
	if !a.Ready {
		return Fail(403, "account_not_ready", "Complete account setup before continuing")
	}
	codes := []string{"tenders.register.view", "tenders.register.read"}
	if write {
		codes = append(codes, "tenders.register.write", "tenders.processing.write")
	}
	for _, code := range codes {
		if !a.Principal.Allows(code) {
			return Forbidden
		}
	}
	return nil
}

type Declaration struct {
	ClientKey, Filename, Kind, SHA256 string
	Size                              int64
}

func Filename(s string) error {
	if len(s) > 200 || !utf8.ValidString(s) || strings.TrimSpace(s) != s || s == "" || s == "." || s == ".." || strings.ContainsAny(s, "/\\") {
		return Fail(422, "validation_failed", "Use a safe leaf filename of at most 200 bytes")
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return Fail(422, "validation_failed", "Invalid filename")
		}
	}
	return nil
}
func Validate(files []Declaration) error {
	if len(files) < 1 || len(files) > 100 {
		return Fail(422, "validation_failed", "Declare between 1 and 100 files")
	}
	keys, names := map[string]bool{}, map[string]bool{}
	var total int64
	for _, f := range files {
		if e := UUID(f.ClientKey); e != nil {
			return e
		}
		if e := Filename(f.Filename); e != nil {
			return e
		}
		switch f.Kind {
		case "pdf", "doc", "docx", "xlsx", "xls", "zip":
		default:
			return Fail(415, "unsupported_media", "Unsupported document kind")
		}
		if !strings.HasSuffix(strings.ToLower(f.Filename), "."+f.Kind) || !Hash(f.SHA256) {
			return Fail(422, "validation_failed", "Declared kind or checksum does not match the filename")
		}
		limit := LeafBytes
		if f.Kind == "zip" {
			limit = ZipBytes
		}
		if f.Size < 1 || f.Size > limit {
			return Fail(413, "payload_too_large", "File exceeds its supported size")
		}
		total += f.Size
		n := strings.ToLower(f.Filename)
		if keys[f.ClientKey] || names[n] {
			return Fail(422, "validation_failed", "Duplicate file keys or ambiguous filenames")
		}
		keys[f.ClientKey] = true
		names[n] = true
	}
	if total > PackageBytes {
		return Fail(413, "payload_too_large", "Receipt exceeds its cumulative transport quota")
	}
	return nil
}

type Receipt struct {
	ID, OwnerID, State, ErrorCode, ConsumedBy, ClaimID string
	Version, LimitRevision                             int64
	ExpiresAt, LeaseUntil, CreatedAt                   time.Time
	Attempts                                           int32
	Files                                              []File
	MemberCount, Unresolved                            int64
}
type File struct {
	ID, ReceiptID, ClientKey, Filename, Kind, SHA256, State, ObjectKey, UploadKey, ClaimID, ErrorCode string
	Size, Version, UploadVersion                                                                      int64
	LeaseUntil                                                                                        time.Time
}
type Member struct {
	ID, ReceiptID, FileID, Path, NormalizedPath, Filename, Kind, SHA256, PayloadSHA256, ClaimID, ScannerRevision string
	Size, ExpandedSize                                                                                           int64
	Payload                                                                                                      []byte
	Pages, Sheets                                                                                                int32
	Cells                                                                                                        int64
}
type Operation struct{ Actor, Command, Context, Key, Fingerprint, ReceiptID string }
type ByteStream interface{ Read([]byte) (int, error) }
type Verification struct {
	Receipt Receipt
	Files   []File
}

type ObjectRef struct{ ID, ReceiptID, FileID, ClaimID string }
