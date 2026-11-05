package outbound

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/domain"
)

type metadataScanner struct{ calls int }

func (s *metadataScanner) Scan(context.Context, *os.File) (string, error) {
	s.calls++
	return "test-scanner", nil
}

type metadataPages struct{}

func (metadataPages) Pages(context.Context, *os.File) (int32, error) { return 1, nil }

func TestFinderMetadataIgnoredWithoutWeakeningArchiveValidation(t *testing.T) {
	for _, tc := range []struct {
		name, extra string
		valid       bool
		documents   int
	}{
		{"finder", "__MACOSX/._notice.pdf", true, 1},
		{"appledouble", "docs/._notice.pdf", true, 1},
		{"finder_store", "docs/.DS_Store", true, 1},
		{"unsafe_metadata", "__MACOSX/../._notice.pdf", false, 0},
		{"unsupported_document", "notes.txt", false, 0},
		{"fake_pdf", "fake.pdf", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cipher, e := infrastructure.NewDocumentCipher(strings.Repeat("ec", 32))
			if e != nil {
				t.Fatal(e)
			}
			store, e := infrastructure.NewDocumentStreamStorage(filepath.Join(t.TempDir(), "private"), cipher)
			if e != nil {
				t.Fatal(e)
			}
			defer store.Close()
			var b bytes.Buffer
			z := zip.NewWriter(&b)
			f, _ := z.Create("notice.pdf")
			f.Write([]byte("%PDF-1.4 synthetic"))
			f, _ = z.Create(tc.extra)
			f.Write([]byte("Finder packaging data, not a PDF"))
			if e = z.Close(); e != nil {
				t.Fatal(e)
			}
			data := b.Bytes()
			sum := sha256.Sum256(data)
			hash := hex.EncodeToString(sum[:])
			key := Values{}.ID()
			if e = store.Upload(context.Background(), key, bytes.NewReader(data), int64(len(data)), hash); e != nil {
				t.Fatal(e)
			}
			scanner := &metadataScanner{}
			v := Verifier{Store: store, Scanner: scanner, Pages: metadataPages{}, Preparer: runtimePreparer{cipher}}
			members, ignored := 0, 0
			e = v.VerifyManifest(context.Background(), domain.Verification{Files: []domain.File{{ID: Values{}.ID(), Filename: "Archive.zip", Kind: "zip", ObjectKey: key, Size: int64(len(data)), SHA256: hash}}}, func(m domain.Member) error { members++; return nil }, func(d domain.Disposition) error {
				if d.State == "ignored" {
					ignored++
					if d.Code != "archive_metadata" {
						t.Fatal(d)
					}
				}
				return nil
			})
			if tc.valid {
				if e != nil || members != tc.documents || ignored != 1 || scanner.calls != 2 {
					t.Fatalf("members=%d ignored=%d scans=%d err=%v", members, ignored, scanner.calls, e)
				}
			} else if e == nil {
				t.Fatal("unsafe or unsupported member accepted")
			}
		})
	}
}
