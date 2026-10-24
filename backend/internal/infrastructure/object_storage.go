package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"io/fs"
	"time"
)

type ObjectStoreConfig struct {
	Endpoint, AccessKey, SecretKey string
	Secure                         bool
}

func (c ObjectStoreConfig) String() string   { return "ObjectStoreConfig{values omitted}" }
func (c ObjectStoreConfig) GoString() string { return c.String() }

type ObjectStore struct {
	client       *minio.Client
	maximumBytes int
}

func NewObjectStore(config ObjectStoreConfig, maximumBytes int) (*ObjectStore, error) {
	if maximumBytes < 1 {
		return nil, errors.New("invalid object size limit")
	}
	transport, err := minio.DefaultTransport(config.Secure)
	if err != nil {
		return nil, err
	}
	transport.ResponseHeaderTimeout = 30 * time.Second
	client, err := minio.New(config.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""), Secure: config.Secure, Transport: transport})
	if err != nil {
		return nil, err
	}
	return &ObjectStore{client, maximumBytes}, nil
}
func (o *ObjectStore) Put(ctx context.Context, bucket, key string, data []byte) error {
	if len(data) > o.maximumBytes {
		return errors.New("object exceeds bounded payload")
	}
	exists, err := o.client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err = o.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			code := minio.ToErrorResponse(err).Code
			if code != "BucketAlreadyOwnedByYou" {
				return err
			}
		}
	}
	_, err = o.client.PutObject(ctx, bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: "application/octet-stream"})
	return err
}
func (o *ObjectStore) Get(ctx context.Context, bucket, key string) ([]byte, error) {
	object, err := o.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, objectReadError(err)
	}
	defer object.Close()
	data, err := io.ReadAll(io.LimitReader(object, int64(o.maximumBytes)+1))
	if err != nil {
		return nil, objectReadError(err)
	}
	if len(data) > o.maximumBytes {
		return nil, errors.New("stored object exceeds bounded payload")
	}
	return data, nil
}

// A missing bucket and a missing key both mean the referenced bytes are absent.
// Preserve all other storage errors so an outage or denied access is never
// mistaken for missing evidence.
func objectReadError(err error) error {
	switch minio.ToErrorResponse(err).Code {
	case "NoSuchBucket", "NoSuchKey", "NoSuchObject":
		return fmt.Errorf("%w: %w", fs.ErrNotExist, err)
	default:
		return err
	}
}

func (o *ObjectStore) Remove(ctx context.Context, bucket, key string) error {
	return o.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}
