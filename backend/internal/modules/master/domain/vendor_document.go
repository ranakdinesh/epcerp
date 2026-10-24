package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const VendorDocumentMaximumBytes = 15 * 1024 * 1024

type VendorDocumentSource struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}
type VendorDocumentInput struct {
	ReplacesID  string
	Source      VendorDocumentSource
	DocType     string
	Title       string
	Filename    string
	ContentType string
	Content     []byte
	Active      *bool
}
type VendorDocumentDocument struct {
	ApprovalState, ApprovalComment string
	Version                        int32
	ID                             string
	SupplierID                     string
	DocType                        string
	Title                          string
	Filename                       string
	ContentType                    string
	ByteSize                       int32
	SHA256                         string
	Active                         bool
	Encrypted                      bool
	CreatedAt                      time.Time
	UpdatedAt                      time.Time
	CreatedBy                      *string
	UpdatedBy                      *string
	ObjectBucket                   string
	ObjectKey                      string
	Ciphertext                     []byte
	Source                         VendorDocumentSource
}

var VendorDocumentUnauthorized = &Error{Kind: Unauthenticated, Detail: "Invalid or expired credentials"}
var VendorDocumentForbidden = &Error{Kind: AccessDenied, Detail: "Insufficient permission"}
var VendorDocumentMissing = &Error{Kind: NotFound, Detail: "Supplier document not found"}
var VendorDocumentConflict = &Error{Kind: Conflict, Detail: "Source document conflicts with an existing import"}
var VendorDocumentIntegrity = &Error{Kind: Conflict, Detail: "Document integrity verification failed"}

func VendorDocumentInvalid(s string) error { return &Error{Kind: InvalidInput, Detail: s} }

var vendorDocumentUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func VendorDocumentValidID(s string) bool { return vendorDocumentUuid.MatchString(s) }
func VendorDocumentValidate(in VendorDocumentInput) error {
	if !VendorDocumentValidID(in.Source.ID) || in.Source.CreatedAt.IsZero() || in.Source.UpdatedAt.IsZero() || in.Active == nil {
		return VendorDocumentInvalid("Document source and status are required")
	}
	return VendorDocumentValidateUpload(in)
}
func VendorDocumentValidateUpload(in VendorDocumentInput) error {
	if in.ReplacesID != "" && !VendorDocumentValidID(in.ReplacesID) {
		return VendorDocumentFieldError("file", "Invalid replacement document")
	}
	if len(in.Content) == 0 || len(in.Content) > VendorDocumentMaximumBytes {
		return VendorDocumentFieldError("file", "Choose a nonempty file up to 15 MiB")
	}
	switch in.DocType {
	case "aadhar", "pan", "gst", "cheque", "other":
	default:
		return VendorDocumentFieldError("doc_type", "Choose a document type")
	}
	for _, field := range []struct {
		s   string
		max int
	}{{in.Title, 255}, {in.Filename, 255}, {in.ContentType, 120}} {
		if utf8.RuneCountInString(field.s) > field.max || strings.ContainsAny(field.s, "\x00\r\n") {
			return VendorDocumentInvalid("Invalid document metadata")
		}
	}
	if strings.TrimSpace(in.Filename) == "" {
		return VendorDocumentInvalid("Filename is required")
	}
	return nil
}

func VendorDocumentValidReason(s string) bool { return strings.TrimSpace(s) != "" && len(s) <= 2000 }

func VendorDocumentFieldError(field, message string) error {
	return &Error{Kind: InvalidInput, Detail: message, Field: field}
}
