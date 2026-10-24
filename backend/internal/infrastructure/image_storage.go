package infrastructure

import (
	"context"
	"errors"
	"path"
	"strings"
)

type ImageObjects interface {
	Put(context.Context, string, string, []byte) error
	Get(context.Context, string, string) ([]byte, error)
}

// ImageStorage persists encrypted immutable objects; application repositories own
// the authorized metadata reference and its transactional audit history.
type ImageStorage struct {
	objects      ImageObjects
	cipher       *DocumentCipher
	bucket       string
	maximumBytes int
}

func NewImageStorage(o ImageObjects, c *DocumentCipher, bucket string, maximumBytes int) (*ImageStorage, error) {
	if o == nil || c == nil || bucket == "" || maximumBytes < 1 {
		return nil, errors.New("invalid image storage configuration")
	}
	return &ImageStorage{o, c, bucket, maximumBytes}, nil
}
func validImageKey(key string) bool {
	return len(key) <= 512 && strings.HasPrefix(key, "images/") && path.Clean(key) == key && !strings.ContainsAny(key, "\\\x00")
}
func (s *ImageStorage) Store(ctx context.Context, key string, data []byte) (string, error) {
	if !validImageKey(key) || len(data) == 0 || len(data) > s.maximumBytes {
		return "", errors.New("invalid stored image")
	}
	encrypted, hash, err := s.cipher.Seal(data)
	if err != nil {
		return "", err
	}
	if err = s.objects.Put(ctx, s.bucket, key, encrypted); err != nil {
		return "", err
	}
	return hash, nil
}
func (s *ImageStorage) Retrieve(ctx context.Context, key, hash string) ([]byte, error) {
	if !validImageKey(key) {
		return nil, errors.New("invalid stored image")
	}
	encrypted, err := s.objects.Get(ctx, s.bucket, key)
	if err != nil {
		return nil, err
	}
	if len(encrypted) > s.maximumBytes+28 {
		return nil, errors.New("stored image exceeds bounded payload")
	}
	data, err := s.cipher.Open(encrypted, hash)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > s.maximumBytes {
		return nil, errors.New("invalid stored image")
	}
	return data, nil
}
