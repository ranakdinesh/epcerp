package infrastructure

import (
	"errors"
	"github.com/minio/minio-go/v7"
	"io/fs"
	"testing"
)

func TestObjectReadMissingClassification(t *testing.T) {
	for _, code := range []string{"NoSuchBucket", "NoSuchKey", "NoSuchObject", "AccessDenied", "InternalError"} {
		t.Run(code, func(t *testing.T) {
			original := minio.ErrorResponse{Code: code}
			err := objectReadError(original)
			want := code == "NoSuchBucket" || code == "NoSuchKey" || code == "NoSuchObject"
			if errors.Is(err, fs.ErrNotExist) != want {
				t.Fatalf("missing classification: %v", err)
			}
			if !errors.Is(err, original) {
				t.Fatal("original storage error lost")
			}
		})
	}
	original := errors.New("connection failed")
	if objectReadError(original) != original {
		t.Fatal("connection error changed")
	}
}
