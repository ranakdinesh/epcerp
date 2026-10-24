package outbound

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/domain"
	reading "github.com/ranakdinesh/epcERP/internal/modules/document_reading/contracts"
	"golang.org/x/text/unicode/norm"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Objects struct {
	Store *infrastructure.DocumentStreamStorage
}

func (o Objects) Upload(ctx context.Context, key string, body domain.ByteStream, size int64, hash string) error {
	e := o.Store.Upload(ctx, key, body, size, hash)
	if errors.Is(e, infrastructure.ErrDocumentIntegrity) {
		return domain.Fail(422, "checksum_mismatch", "Uploaded bytes do not match the declared size and checksum")
	}
	return e
}

type Scanner interface {
	Scan(context.Context, *os.File) (string, error)
}
type ClamdScanner struct{ Socket string }

func (s ClamdScanner) Scan(ctx context.Context, f *os.File) (string, error) {
	if s.Socket == "" {
		return "", domain.Unavailable
	}
	dial := net.Dialer{Timeout: 3 * time.Second}
	c, e := dial.DialContext(ctx, "unix", s.Socket)
	if e != nil {
		return "", domain.Unavailable
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(2 * time.Minute))
	if _, e = c.Write([]byte("zINSTREAM\x00")); e != nil {
		return "", domain.Unavailable
	}
	if _, e = f.Seek(0, io.SeekStart); e != nil {
		return "", e
	}
	buf := make([]byte, 64<<10)
	for {
		if e = ctx.Err(); e != nil {
			return "", e
		}
		n, err := f.Read(buf)
		if n > 0 {
			if e = binary.Write(c, binary.BigEndian, uint32(n)); e != nil {
				return "", domain.Unavailable
			}
			if _, e = c.Write(buf[:n]); e != nil {
				return "", domain.Unavailable
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	if e = binary.Write(c, binary.BigEndian, uint32(0)); e != nil {
		return "", domain.Unavailable
	}
	response := make([]byte, 4096)
	n, e := c.Read(response)
	if e != nil {
		return "", domain.Unavailable
	}
	result := strings.TrimRight(string(response[:n]), "\x00\r\n")
	if strings.HasSuffix(result, " FOUND") {
		return "", domain.Fail(422, "malware_detected", "The package was quarantined by malware inspection")
	}
	if result != "stream: OK" {
		return "", domain.Unavailable
	}
	version, e := dial.DialContext(ctx, "unix", s.Socket)
	if e != nil {
		return "", domain.Unavailable
	}
	defer version.Close()
	_ = version.SetDeadline(time.Now().Add(3 * time.Second))
	if _, e = version.Write([]byte("zVERSION\x00")); e != nil {
		return "", domain.Unavailable
	}
	n, e = version.Read(response[:256])
	if e != nil || n < 1 {
		return "", domain.Unavailable
	}
	revision := strings.TrimRight(string(response[:n]), "\x00\r\n")
	if !strings.HasPrefix(revision, "ClamAV ") {
		return "", domain.Unavailable
	}
	return revision, nil
}

type PageInspector interface {
	Pages(context.Context, *os.File) (int32, error)
}
type PDFInfo struct{}
type cappedBuffer struct {
	bytes.Buffer
	max int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.max-b.Len() {
		return 0, errors.New("bounded output exceeded")
	}
	return b.Buffer.Write(p)
}
func (PDFInfo) Pages(ctx context.Context, f *os.File) (int32, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	binary, e := exec.LookPath("pdfinfo")
	if e != nil {
		return 0, domain.Unavailable
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "linux" {
		limit, e := exec.LookPath("prlimit")
		if e != nil {
			return 0, domain.Unavailable
		}
		cmd = exec.CommandContext(ctx, limit, "--as=536870912", "--cpu=30", "--nofile=64", "--", binary, f.Name())
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", `ulimit -v 524288 && ulimit -t 30 && exec "$@"`, "bounded-pdfinfo", binary, f.Name())
	}
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C", "LC_ALL=C", "HOME=/tmp"}
	out := &cappedBuffer{max: 64 << 10}
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	if e = cmd.Run(); e != nil {
		return 0, domain.Fail(422, "invalid_pdf", "PDF is damaged, encrypted or exceeds inspection limits")
	}
	pages := regexp.MustCompile(`(?m)^Pages:\s+(\d+)\s*$`).FindStringSubmatch(out.String())
	if len(pages) != 2 {
		return 0, domain.Fail(422, "invalid_pdf", "PDF page count could not be verified")
	}
	n, e := strconv.Atoi(pages[1])
	if e != nil || n < 1 || n > 1000 {
		return 0, domain.Fail(413, "payload_too_large", "PDF exceeds the 1000-page limit")
	}
	if !regexp.MustCompile(`(?m)^Encrypted:\s+no\s*$`).MatchString(out.String()) {
		return 0, domain.Fail(422, "encrypted_document", "Encrypted PDFs are not supported")
	}
	return int32(n), nil
}

type Verifier struct {
	Store    *infrastructure.DocumentStreamStorage
	Scanner  Scanner
	Pages    PageInspector
	Preparer reading.Preparer
}

func safePath(p string) (string, error) {
	if !utf8.ValidString(p) || len(p) > 2048 || p == "" || strings.ContainsAny(p, "\\:") || strings.HasPrefix(p, "/") {
		return "", domain.Fail(422, "unsafe_archive_path", "Archive contains an unsafe path")
	}
	for _, r := range p {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", domain.Fail(422, "unsafe_archive_path", "Archive contains an unsafe path")
		}
	}
	trimmed := strings.TrimSuffix(p, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) > 10 {
		return "", domain.Fail(422, "archive_depth_limit", "Archive path is too deep")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || part != strings.TrimSpace(part) || strings.TrimRight(part, ". ") != part {
			return "", domain.Fail(422, "unsafe_archive_path", "Archive contains an ambiguous path")
		}
	}
	return strings.ToLower(norm.NFC.String(path.Clean(trimmed))), nil
}
func (v Verifier) Verify(ctx context.Context, in domain.Verification, emit func(domain.Member) error) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	seen := map[string]bool{}
	var count int
	var expanded int64
	var pages, sheets int32
	var cells int64
	leaf := func(f domain.File, original string, stream io.Reader, size int64, compressed uint64) error {
		if size < 1 || size > domain.LeafBytes || compressed == 0 || uint64(size) > compressed*100 {
			return domain.Fail(413, "archive_expansion_limit", "Archive member exceeds the bounded expansion ratio or size")
		}
		n, e := safePath(original)
		if e != nil {
			return e
		}
		if seen[n] {
			return domain.Fail(422, "archive_path_collision", "Archive member paths collide")
		}
		seen[n] = true
		count++
		if count > 100 {
			return domain.Fail(413, "member_limit", "Package has more than 100 document members")
		}
		name := path.Base(original)
		if e = domain.Filename(name); e != nil {
			return e
		}
		ext := strings.ToLower(path.Ext(name))
		kind := strings.TrimPrefix(ext, ".")
		if kind != "pdf" && kind != "docx" && kind != "xlsx" {
			return domain.Fail(415, "unsupported_media", "Package contains an unsupported, legacy or nested document; upload PDF, DOCX or XLSX")
		}
		data, e := io.ReadAll(io.LimitReader(stream, domain.LeafBytes+1))
		if e != nil {
			return domain.Fail(422, "archive_integrity_failed", "Archive member integrity failed")
		}
		if int64(len(data)) != size {
			return domain.Fail(422, "archive_integrity_failed", "Archive member size does not match its manifest")
		}
		temp, e := os.CreateTemp("", "epcerp-package-leaf-")
		if e != nil {
			return e
		}
		defer func() { _ = temp.Close(); _ = os.Remove(temp.Name()) }()
		if _, e = temp.Write(data); e != nil {
			return e
		}
		revision, e := v.Scanner.Scan(ctx, temp)
		if e != nil {
			return e
		}
		m := domain.Member{FileID: f.ID, Path: original, NormalizedPath: n, Filename: name, Kind: kind, Size: size, ExpandedSize: size, ScannerRevision: revision}
		if kind == "pdf" {
			if !bytes.HasPrefix(data, []byte("%PDF-")) {
				return domain.Fail(415, "unsupported_media", "PDF signature does not match")
			}
			m.Pages, e = v.Pages.Pages(ctx, temp)
			if e != nil {
				return e
			}
		} else {
			m.ExpandedSize, m.Sheets, m.Cells, e = inspectOffice(ctx, data, kind)
			if e != nil {
				return e
			}
		}
		expanded += m.ExpandedSize
		pages += m.Pages
		sheets += m.Sheets
		cells += m.Cells
		if expanded > domain.PackageBytes || pages > 3000 || sheets > 200 || cells > 250000 {
			return domain.Fail(413, "payload_too_large", "Package exceeds cumulative page, sheet, cell or expansion bounds")
		}
		prepared, e := v.Preparer.Prepare(name, data)
		if e != nil {
			return domain.Fail(422, "document_validation_failed", "Source document failed reader format checks")
		}
		m.Payload = prepared.Ciphertext
		m.PayloadSHA256 = prepared.CipherSHA256
		m.SHA256 = prepared.SHA256
		return emit(m)
	}
	for _, f := range in.Files {
		if e := v.Store.WithPlain(ctx, f.ObjectKey, f.Size, f.SHA256, func(source *os.File) error {
			if _, e := v.Scanner.Scan(ctx, source); e != nil {
				return e
			}
			if _, e := source.Seek(0, io.SeekStart); e != nil {
				return e
			}
			if f.Kind != "zip" {
				return leaf(f, f.Filename, source, f.Size, uint64(f.Size))
			}
			z, e := zip.NewReader(source, f.Size)
			if e != nil {
				return domain.Fail(422, "invalid_archive", "ZIP directory is damaged")
			}
			if len(z.File) == 0 || len(z.File) > 200 {
				return domain.Fail(413, "member_limit", "ZIP has too many entries or is empty")
			}
			paths := map[string]bool{}
			for _, entry := range z.File {
				n, e := safePath(entry.Name)
				if e != nil {
					return e
				}
				if paths[n] {
					return domain.Fail(422, "archive_path_collision", "ZIP entry paths collide")
				}
				paths[n] = true
				if entry.Flags&1 != 0 || (!entry.Mode().IsRegular() && !entry.Mode().IsDir()) || (entry.Method != zip.Store && entry.Method != zip.Deflate) {
					return domain.Fail(422, "unsafe_archive_entry", "Encrypted archives, links and special entries are not supported")
				}
				if entry.Mode().IsDir() {
					continue
				}
				if entry.UncompressedSize64 > uint64(domain.LeafBytes) || entry.CompressedSize64 == 0 || entry.UncompressedSize64 > entry.CompressedSize64*100 {
					return domain.Fail(413, "archive_expansion_limit", "ZIP expansion exceeds bounds")
				}
				member, e := entry.Open()
				if e != nil {
					return domain.Fail(422, "invalid_archive", "Cannot read ZIP member")
				}
				e = leaf(f, f.Filename+"/"+entry.Name, member, int64(entry.UncompressedSize64), entry.CompressedSize64)
				_ = member.Close()
				if e != nil {
					return e
				}
			}
			return nil
		}); e != nil {
			return e
		}
	}
	if count == 0 {
		return domain.Fail(422, "empty_package", "Package contains no supported documents")
	}
	return nil
}
func inspectOffice(ctx context.Context, data []byte, kind string) (expanded int64, sheets int32, cells int64, e error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, 0, 0, domain.Fail(422, "invalid_office", "Office container is damaged")
	}
	if len(z.File) > 10000 {
		return 0, 0, 0, domain.Fail(413, "office_expansion_limit", "Office container has too many parts")
	}
	seen := map[string]bool{}
	found := false
	for _, f := range z.File {
		if err = ctx.Err(); err != nil {
			return 0, 0, 0, err
		}
		n, err := safeOfficePath(f.Name)
		if err != nil {
			return 0, 0, 0, err
		}
		if seen[n] {
			return 0, 0, 0, domain.Fail(422, "office_path_collision", "Office part paths collide")
		}
		seen[n] = true
		lower := strings.ToLower(f.Name)
		if f.Flags&1 != 0 || (!f.Mode().IsRegular() && !f.Mode().IsDir()) || strings.Contains(lower, "vbaproject") || strings.Contains(lower, "activex") || strings.Contains(lower, "embeddings/") {
			return 0, 0, 0, domain.Fail(422, "active_office_content", "Active or embedded Office content is not supported")
		}
		if f.Mode().IsDir() {
			continue
		}
		if f.UncompressedSize64 > 64<<20 || f.CompressedSize64 == 0 || f.UncompressedSize64 > f.CompressedSize64*100 {
			return 0, 0, 0, domain.Fail(413, "office_expansion_limit", "Office part exceeds expansion bounds")
		}
		if kind == "docx" && f.Name == "word/document.xml" || kind == "xlsx" && f.Name == "xl/workbook.xml" {
			found = true
		}
		sheet := kind == "xlsx" && strings.HasPrefix(f.Name, "xl/worksheets/") && strings.HasSuffix(f.Name, ".xml")
		if sheet {
			sheets++
		}
		rc, err := f.Open()
		if err != nil {
			return 0, 0, 0, domain.Fail(422, "invalid_office", "Office part cannot be read")
		}
		counter := &countReader{r: io.LimitReader(rc, (64<<20)+1)}
		if strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".rels") {
			d := xml.NewDecoder(counter)
			depth, tokens := 0, 0
			for {
				token, err := d.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					e = domain.Fail(422, "invalid_office", "Office XML is invalid or exceeds bounds")
					break
				}
				tokens++
				if tokens > 1000000 {
					e = domain.Fail(413, "office_expansion_limit", "Office XML token limit exceeded")
					break
				}
				switch token := token.(type) {
				case xml.StartElement:
					depth++
					if sheet && token.Name.Local == "c" {
						cells++
					}
					for _, a := range token.Attr {
						if a.Name.Local == "TargetMode" && strings.EqualFold(a.Value, "External") {
							e = domain.Fail(422, "external_office_reference", "External Office relationships require manual review")
						}
					}
				case xml.EndElement:
					depth--
				case xml.Directive:
					e = domain.Fail(422, "active_office_content", "Office XML declarations are not supported")
				}
				if depth > 100 || cells > 100000 || sheets > 100 {
					e = domain.Fail(413, "office_expansion_limit", "Office structure exceeds bounds")
				}
				if e != nil {
					break
				}
			}
		} else {
			_, err = io.Copy(io.Discard, counter)
			if err != nil {
				e = domain.Fail(422, "invalid_office", "Office part integrity failed")
			}
		}
		_ = rc.Close()
		expanded += counter.n
		if e != nil {
			return 0, 0, 0, e
		}
		if counter.n != int64(f.UncompressedSize64) || expanded > 64<<20 {
			return 0, 0, 0, domain.Fail(413, "office_expansion_limit", "Office expansion exceeds bounds")
		}
	}
	if !found {
		return 0, 0, 0, domain.Fail(422, "invalid_office", fmt.Sprintf("Required %s package part is missing", kind))
	}
	if int64(len(data)) > expanded {
		expanded = int64(len(data))
	}
	return
}
func safeOfficePath(s string) (string, error) { return safePath(s) }

type countReader struct {
	r io.Reader
	n int64
}

func (r *countReader) Read(b []byte) (int, error) { n, e := r.r.Read(b); r.n += int64(n); return n, e }

func (o Objects) Remove(ctx context.Context, key string) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	return o.Store.Remove(key)
}
