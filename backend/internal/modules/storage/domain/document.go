package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const MaximumDocumentBytes = 50 * 1024 * 1024

type Document struct {
	ID          string     `json:"id"`
	EntityType  string     `json:"entity_type"`
	EntityID    string     `json:"entity_id"`
	DocType     string     `json:"doc_type"`
	Title       string     `json:"title"`
	Filename    string     `json:"original_filename"`
	ContentType string     `json:"content_type"`
	ByteSize    int        `json:"byte_size"`
	Bucket      string     `json:"storage_bucket"`
	Key         string     `json:"storage_key"`
	SHA256      string     `json:"sha256_hash"`
	Encrypted   bool       `json:"is_encrypted"`
	Algorithm   string     `json:"encryption_algorithm"`
	Active      bool       `json:"is_active"`
	CreatedAt   *time.Time `json:"created_at"`
	CreatedBy   *string    `json:"created_by_id"`
	UpdatedAt   *time.Time `json:"updated_at"`
	UpdatedBy   *string    `json:"updated_by_id"`
}
type Principal struct {
	ID                string
	Active, Superuser bool
	Permissions       []string
}

func (p Principal) Allows(code string) bool {
	if !p.Active {
		return false
	}
	if p.Superuser {
		return true
	}
	for _, permission := range p.Permissions {
		if permission == code {
			return true
		}
	}
	return false
}

type Upload struct {
	EntityType, EntityID, DocType, Title, Filename, ContentType string
	Data                                                        []byte
}
type Event struct {
	ID, DocumentID, Action, ActorID string
	Snapshot                        map[string]any
}
type Error struct {
	Code   int
	Detail string
}

func (e *Error) Error() string { return e.Detail }

var ErrUnauthorized = &Error{401, "Invalid or expired credentials"}
var ErrInactive = &Error{404, "Document not found or inactive"}
var ErrMissing = &Error{404, "Document not found"}
var ErrIntegrity = &Error{400, "Document integrity verification failed (SHA256 mismatch). Document payload may have been tampered with or corrupted."}
var ErrCiphertext = &Error{400, "Document encryption authentication failed"}

func Denied(code string) error    { return &Error{403, "Insufficient permission: required " + code} }
func Invalid(detail string) error { return &Error{400, detail} }
func At(nanos int64) *time.Time   { value := time.Unix(0, nanos).UTC(); return &value }

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}$`)

func CanonicalID(raw string) (string, error) {
	if !uuidPattern.MatchString(raw) {
		return "", Invalid("Invalid UUID")
	}
	s := strings.ToLower(strings.ReplaceAll(raw, "-", ""))
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:], nil
}
func SanitizeFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	if i := strings.LastIndex(filename, "/"); i >= 0 {
		filename = filename[i+1:]
	}
	cleaned := []rune{}
	for _, r := range filename {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || strings.ContainsRune("_.-()", r) {
			cleaned = append(cleaned, r)
		} else {
			cleaned = append(cleaned, '_')
		}
	}
	value := string(cleaned)
	if value == "" || strings.HasPrefix(value, ".") {
		value = "document_" + strings.TrimLeft(value, ".")
	}
	chars := []rune(value)
	if len(chars) > 200 {
		chars = chars[:200]
	}
	return string(chars)
}

var AllowedContentTypes = []string{
	"application/pdf", "image/png", "image/jpeg", "image/webp", "text/csv", "text/plain",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/vnd.ms-excel",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/zip",
	"application/x-zip-compressed", "application/octet-stream",
}
var DocumentTypes = []string{
	"aadhar", "pan", "gst", "cancelled_cheque", "msme_certificate", "vendor_profile", "bank_mandate",
	"po_signed_copy", "advance_request", "call_notice", "stage_inspection", "irn_note", "tc_test_certificate",
	"tr_test_report", "mtc_mill_test_certificate", "rtc_routine_test_certificate", "warranty_certificate",
	"mdcc_clearance", "transporter_lr", "eway_bill", "tax_invoice", "site_verification", "grn_voucher",
	"mbook_entry", "jv_settlement", "contract_evidence", "project_document", "technical_datasheet", "drawing", "other",
}

func member(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
func Validate(request Upload) error {
	if len(request.Data) == 0 {
		return Invalid("Document cannot be empty.")
	}
	if len(request.Data) > MaximumDocumentBytes {
		return Invalid("Document exceeds maximum allowable size of 50MB.")
	}
	if !member(DocumentTypes, strings.ToLower(strings.TrimSpace(request.DocType))) {
		return Invalid("Invalid document type")
	}
	if !member(AllowedContentTypes, strings.ToLower(strings.TrimSpace(request.ContentType))) {
		return Invalid("Disallowed content type")
	}
	for _, field := range []struct {
		name, value string
		maximum     int
	}{{"entity_type", request.EntityType, 60}, {"title", request.Title, 255}} {
		if utf8.RuneCountInString(field.value) > field.maximum {
			return Invalid(fmt.Sprintf("%s exceeds maximum length", field.name))
		}
	}
	if strings.TrimSpace(request.EntityType) == "" {
		return Invalid("Entity type cannot be empty")
	}
	return nil
}
func Normalize(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func UploadTitle(title, filename string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return filename
	}
	return title
}
func StorageKey(prefix, entityID, documentID, filename string) (string, error) {
	key := prefix + "/" + entityID + "/" + documentID + "_" + filename + ".enc"
	if utf8.RuneCountInString(key) > 500 {
		return "", Invalid("Storage key exceeds maximum length")
	}
	return key, nil
}
