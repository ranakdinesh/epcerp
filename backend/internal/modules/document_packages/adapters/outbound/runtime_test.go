package outbound

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/domain"
	reading "github.com/ranakdinesh/epcERP/internal/modules/document_reading/contracts"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type runtimePreparer struct {
	c *infrastructure.DocumentCipher
}

func (p runtimePreparer) Prepare(name string, b []byte) (reading.Prepared, error) {
	sealed, hash, e := p.c.Seal(b)
	sum := sha256.Sum256(sealed)
	return reading.Prepared{Filename: name, Format: strings.TrimPrefix(filepath.Ext(name), "."), Size: int64(len(b)), SHA256: hash, Ciphertext: sealed, CipherSHA256: hex.EncodeToString(sum[:])}, e
}
func TestRuntimeScannerAndPackageManifest(t *testing.T) {
	if os.Getenv("EPCERP_SCANNER_TOOLS_TEST") != "1" {
		t.Skip("requires isolated runtime scanner test")
	}
	ctx := context.Background()
	cipher, e := infrastructure.NewDocumentCipher(strings.Repeat("ec", 32))
	if e != nil {
		t.Fatal(e)
	}
	store, e := infrastructure.NewDocumentStreamStorage(filepath.Join(t.TempDir(), "private"), cipher)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	pdf, e := os.ReadFile("/testdata/native.pdf")
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name string
		bad  bool
	}{{"supported", false}, {"unsupported", true}, {"unsafe_path", true}} {
		t.Run(tc.name, func(t *testing.T) {
			var buffer bytes.Buffer
			z := zip.NewWriter(&buffer)
			f, _ := z.Create("notice.pdf")
			f.Write(pdf)
			name := "schedule.pdf"
			body := pdf
			if tc.name == "unsupported" {
				name = "readme.txt"
				body = []byte("Unsupported source fixture")
			}
			if tc.name == "unsafe_path" {
				name = "../notice.pdf"
			}
			f, _ = z.Create(name)
			f.Write(body)
			z.Close()
			data := buffer.Bytes()
			key := Values{}.ID()
			sum := sha256.Sum256(data)
			hash := hex.EncodeToString(sum[:])
			if e := store.Upload(ctx, key, bytes.NewReader(data), int64(len(data)), hash); e != nil {
				t.Fatal(e)
			}
			v := Verifier{Store: store, Scanner: ClamdScanner{Socket: os.Getenv("EPCERP_CLAMD_SOCKET")}, Pages: PDFInfo{}, Preparer: runtimePreparer{cipher}}
			entries := []domain.Disposition{}
			members := 0
			e := v.VerifyManifest(ctx, domain.Verification{Files: []domain.File{{ID: Values{}.ID(), Filename: "sources.zip", Kind: "zip", ObjectKey: key, Size: int64(len(data)), SHA256: hash}}}, func(m domain.Member) error {
				members++
				if m.ScannerRevision == "" || m.Pages != 1 {
					t.Fatal("missing scanner/page evidence", m.Pages)
				}
				return nil
			}, func(d domain.Disposition) error { entries = append(entries, d); return nil })
			if !tc.bad && (e != nil || members != 2) {
				t.Fatal("supported package", members, e)
			}
			if tc.bad && e == nil {
				t.Fatal("unsafe package accepted")
			}
			listed, blocked := 0, 0
			for _, d := range entries {
				if d.State == "listed" {
					listed++
				}
				if d.State == "blocked" {
					blocked++
				}
			}
			if listed != 3 || tc.bad && blocked != 1 {
				t.Fatal("manifest omitted archive entries/disposition", entries)
			}
		})
	}
	file, e := os.CreateTemp("", "scanner-outage-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.Remove(file.Name())
	defer file.Close()
	file.Write(pdf)
	if _, e = (ClamdScanner{Socket: "tcp://127.0.0.1:1"}).Scan(ctx, file); !errors.Is(e, domain.Unavailable) {
		t.Fatal("scanner outage did not block verification", e)
	}
}

func TestRuntimeStorageCapacity(t *testing.T) {
	if os.Getenv("EPCERP_STORAGE_CAPACITY_TEST") != "1" {
		t.Skip("run with /tmp tmpfs smaller than the 256 MiB reserve")
	}
	cipher, e := infrastructure.NewDocumentCipher(strings.Repeat("ed", 32))
	if e != nil {
		t.Fatal(e)
	}
	store, e := infrastructure.NewDocumentStreamStorage(filepath.Join(t.TempDir(), "private"), cipher)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	e = (Objects{Store: store}).Upload(context.Background(), Values{}.ID(), bytes.NewReader([]byte("x")), 1, strings.Repeat("a", 64))
	var failure *domain.Error
	if !errors.As(e, &failure) || failure.Status != 507 {
		t.Fatal("storage admission did not reject safely", e)
	}
}
