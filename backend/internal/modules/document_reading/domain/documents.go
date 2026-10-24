package domain

import (
	"regexp"
	"strings"
	"time"
)

const MaxBytes = 32 << 20
const ReaderVersion = "epcerp-reading-v1"

type Error struct {
	Status int
	Detail string
}

func (e *Error) Error() string { return e.Detail }
func Invalid(s string) error   { return &Error{422, s} }

var Missing = &Error{404, "Document not found"}
var Conflict = &Error{409, "Reading state changed; reload before retrying"}

type Input struct {
	Module, EntityID, ActorID, Filename, Format, SHA256 string
	SizeBytes                                           int64
	Original                                            []byte
}
type Document struct {
	ID           string    `json:"id"`
	Module       string    `json:"-"`
	EntityID     string    `json:"-"`
	Filename     string    `json:"filename"`
	Format       string    `json:"format"`
	Size         int64     `json:"size_bytes"`
	SHA256       string    `json:"sha256"`
	Status       string    `json:"status"`
	Version      int32     `json:"version"`
	Attempts     int32     `json:"attempts"`
	Error        string    `json:"error"`
	SegmentCount int32     `json:"segment_count"`
	Warnings     []string  `json:"warnings"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ActorID      string    `json:"created_by_id"`
	Original     []byte    `json:"-"`
	ClaimID      string    `json:"-"`
}
type Word struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
}
type Cell struct {
	Address string `json:"address"`
	Value   string `json:"value"`
	Formula string `json:"formula,omitempty"`
}
type Segment struct {
	CoordinateUnit string   `json:"coordinate_unit,omitempty"`
	Kind           string   `json:"kind"`
	Label          string   `json:"label"`
	Text           string   `json:"text"`
	NativeText     string   `json:"native_text,omitempty"`
	Method         string   `json:"method"`
	Confidence     *float64 `json:"confidence,omitempty"`
	Words          []Word   `json:"words,omitempty"`
	Cells          []Cell   `json:"cells,omitempty"`
	Merges         []string `json:"merged_ranges,omitempty"`
}
type PartCoverage struct {
	Path        string `json:"path"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason"`
}
type Result struct {
	Parts         []PartCoverage `json:"parts,omitempty"`
	Schema        string         `json:"schema"`
	ReaderVersion string         `json:"reader_version"`
	SourceSHA256  string         `json:"source_sha256"`
	Segments      []Segment      `json:"segments"`
	Warnings      []string       `json:"warnings"`
}
type Event struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	Action     string    `json:"action"`
	Version    int32     `json:"version"`
	ActorID    string    `json:"actor_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

var idPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

func Filename(s string) (string, string, error) {
	s = strings.ReplaceAll(s, "\\", "/")
	parts := strings.Split(s, "/")
	s = parts[len(parts)-1]
	if s == "" || len(s) > 200 || strings.IndexFunc(s, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
		return "", "", Invalid("Choose a filename of at most 200 bytes")
	}
	dot := strings.LastIndex(s, ".")
	if dot < 0 {
		return "", "", Invalid("Choose PDF, Excel or Word documents")
	}
	ext := strings.ToLower(s[dot+1:])
	switch ext {
	case "pdf", "xlsx", "xls", "docx", "doc":
		return s, ext, nil
	}
	return "", "", Invalid("Allowed formats: PDF, XLSX, XLS, DOCX and DOC")
}
func Validate(in Input) error {
	if !idPattern.MatchString(in.EntityID) || !idPattern.MatchString(in.ActorID) || !regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`).MatchString(in.Module) {
		return Invalid("Invalid document context")
	}
	if len(in.Original) < 1 || len(in.Original) > MaxBytes {
		return Invalid("Document must be at most 32 MiB")
	}
	return nil
}
func Retry(d Document, version int32) error {
	if d.Version != version {
		return Conflict
	}
	if d.Status != "failed" || d.Attempts >= 3 {
		return Invalid("Only failed readings with fewer than three attempts may be retried")
	}
	return nil
}
func CheckResult(r Result) error {
	if r.Schema != ReaderVersion || r.ReaderVersion == "" || len(r.Segments) == 0 {
		return Invalid("No readable document content found")
	}
	found := false
	for _, s := range r.Segments {
		if strings.TrimSpace(s.Text) != "" {
			found = true
		}
	}
	if !found {
		return Invalid("No readable text found; inspect the original source")
	}
	return nil
}

// Prepared is internal encrypted source evidence, never a public request DTO.
type Prepared struct {
	Filename, Format, SHA256, CipherSHA256 string
	Size                                   int64
	Ciphertext                             []byte
}

func ValidPreparedContext(in Input, p Prepared) bool {
	name, format, e := Filename(p.Filename)
	return e == nil && name == p.Filename && format == p.Format && p.Size > 0 && p.Size <= MaxBytes && int64(len(p.Ciphertext)) == p.Size+28 &&
		idPattern.MatchString(in.EntityID) && idPattern.MatchString(in.ActorID) && regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`).MatchString(in.Module) &&
		regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(p.SHA256) && regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(p.CipherSHA256)
}
