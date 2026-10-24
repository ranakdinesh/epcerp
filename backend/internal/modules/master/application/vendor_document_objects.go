package application

import (
	"context"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

func (s *VendorDocumentService) WithObjects(o masterPorts.VendorDocumentObjects, bucket string) *VendorDocumentService {
	s.objects = o
	s.bucket = bucket
	return s
}
func (s *VendorDocumentService) store(ctx context.Context, supplier, id string, encrypted []byte) (string, string, error) {
	if s.objects == nil || s.bucket == "" {
		return "", "", masterDomain.VendorDocumentInvalid("Document storage unavailable")
	}
	key := "suppliers/" + supplier + "/" + id
	if err := s.objects.Put(ctx, s.bucket, key, encrypted); err != nil {
		return "", "", err
	}
	return s.bucket, key, nil
}
