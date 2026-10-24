package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
)

// DocumentStreamStorage stores private immutable, chunk-authenticated objects.
// Overall plaintext hash/size and strict framing detect truncation/reordering.
// Plaintext staging exists only while the bounded verifier callback is executing.
type DocumentStreamStorage struct {
	root   *os.Root
	path   string
	cipher *DocumentCipher
}

func NewDocumentStreamStorage(dir string, c *DocumentCipher) (*DocumentStreamStorage, error) {
	if c == nil || dir == "" {
		return nil, errors.New("document storage configuration required")
	}
	absolute, e := filepath.Abs(dir)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(absolute, 0700); e != nil {
		return nil, e
	}
	info, e := os.Lstat(absolute)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("document storage directory must be private")
	}
	root, e := os.OpenRoot(absolute)
	if e != nil {
		return nil, e
	}
	return &DocumentStreamStorage{root, absolute, c}, nil
}

var ErrDocumentStorageFull = errors.New("insufficient document storage capacity")
var documentObjectID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func (s *DocumentStreamStorage) Close() error { return s.root.Close() }
func (s *DocumentStreamStorage) Upload(ctx context.Context, key string, body io.Reader, size int64, hash string) error {
	if !documentObjectID.MatchString(key) || size < 1 || size > 64<<20 {
		return errors.New("invalid source object")
	}
	var capacity syscall.Statfs_t
	if e := syscall.Statfs(s.path, &capacity); e != nil {
		return e
	}
	if uint64(capacity.Bavail)*uint64(capacity.Bsize) < uint64(size)+(256<<20) {
		return ErrDocumentStorageFull
	}
	name := key + ".blob"
	f, e := s.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	success := false
	defer func() {
		_ = f.Close()
		if !success {
			_ = s.root.Remove(name)
		}
	}()
	if _, e = f.Write([]byte("EPCPKG01")); e != nil {
		return e
	}
	h := sha256.New()
	var actual int64
	source := io.LimitReader(body, size+1)
	buf := make([]byte, 1<<20)
	for {
		if e = ctx.Err(); e != nil {
			return e
		}
		n, readErr := io.ReadFull(source, buf)
		if n > 0 {
			actual += int64(n)
			if actual > size {
				return errors.New("source size mismatch")
			}
			_, _ = h.Write(buf[:n])
			sealed, _, err := s.cipher.Seal(buf[:n])
			if err != nil {
				return err
			}
			if e = binary.Write(f, binary.BigEndian, uint32(len(sealed))); e != nil {
				return e
			}
			if _, e = f.Write(sealed); e != nil {
				return e
			}
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if actual != size || hex.EncodeToString(h.Sum(nil)) != hash {
		return ErrDocumentIntegrity
	}
	if e = binary.Write(f, binary.BigEndian, uint32(0)); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	success = true
	return nil
}
func (s *DocumentStreamStorage) WithPlain(ctx context.Context, key string, size int64, hash string, work func(*os.File) error) error {
	if !documentObjectID.MatchString(key) || size < 1 || size > 64<<20 {
		return ErrDocumentIntegrity
	}
	in, e := s.root.OpenFile(key+".blob", os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return e
	}
	defer in.Close()
	info, e := in.Stat()
	if e != nil || !info.Mode().IsRegular() {
		return ErrDocumentIntegrity
	}
	temp, e := os.CreateTemp(s.path, "plain-")
	if e != nil {
		return e
	}
	defer func() { _ = temp.Close(); _ = os.Remove(temp.Name()) }()
	magic := make([]byte, 8)
	if _, e = io.ReadFull(in, magic); e != nil || string(magic) != "EPCPKG01" {
		return ErrDocumentIntegrity
	}
	h := sha256.New()
	var actual int64
	for {
		if e = ctx.Err(); e != nil {
			return e
		}
		var length uint32
		if e = binary.Read(in, binary.BigEndian, &length); e != nil {
			return ErrDocumentIntegrity
		}
		if length == 0 {
			break
		}
		if length < 29 || length > (1<<20)+28 {
			return ErrDocumentIntegrity
		}
		sealed := make([]byte, int(length))
		if _, e = io.ReadFull(in, sealed); e != nil {
			return ErrDocumentIntegrity
		}
		// Per-chunk GCM authentication; total hash is checked before callback exposure.
		plain, e := s.cipher.aead.Open(nil, nil, sealed, nil)
		if e != nil {
			return ErrDocumentIntegrity
		}
		actual += int64(len(plain))
		if actual > size {
			return ErrDocumentIntegrity
		}
		_, _ = h.Write(plain)
		if _, e = temp.Write(plain); e != nil {
			return e
		}
	}
	tail := make([]byte, 1)
	n, err := in.Read(tail)
	if n != 0 || err != io.EOF || actual != size || hex.EncodeToString(h.Sum(nil)) != hash {
		return ErrDocumentIntegrity
	}
	if _, e = temp.Seek(0, io.SeekStart); e != nil {
		return e
	}
	return work(temp)
}
func (s *DocumentStreamStorage) Remove(key string) error {
	if !documentObjectID.MatchString(key) {
		return errors.New("invalid object identity")
	}
	e := s.root.Remove(key + ".blob")
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	return e
}
