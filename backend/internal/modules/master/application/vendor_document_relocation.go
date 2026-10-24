package application

import (
	"context"
	"errors"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

// VendorDocumentRelocation is invoked only by the offline database operator CLI.
// No HTTP route exposes it; original actors, timestamps, versions and review fingerprints survive.
type VendorDocumentRelocation struct {
	u      ports.VendorDocumentRelocationUnitOfWork
	o      ports.VendorDocumentObjects
	c      ports.VendorDocumentCipher
	v      ports.VendorDocumentValues
	bucket string
}

func NewVendorDocumentRelocation(u ports.VendorDocumentRelocationUnitOfWork, o ports.VendorDocumentObjects, c ports.VendorDocumentCipher, v ports.VendorDocumentValues, bucket string) *VendorDocumentRelocation {
	return &VendorDocumentRelocation{u, o, c, v, bucket}
}
func (s *VendorDocumentRelocation) Run(ctx context.Context) (int, error) {
	count := 0
	for {
		moved := false
		err := s.u.Within(ctx, func(ctx context.Context, r ports.VendorDocumentRelocationRepository) error {
			d, err := r.NextInline(ctx)
			if err != nil || d == nil {
				return err
			}
			if s.o == nil || s.bucket == "" {
				return errors.New("document storage unavailable")
			}
			if _, err = s.c.Open(d.Ciphertext, d.SHA256); err != nil {
				return domain.VendorDocumentIntegrity
			}
			key := "suppliers/" + d.SupplierID + "/" + d.ID
			if err = s.o.Put(ctx, s.bucket, key, d.Ciphertext); err != nil {
				return err
			}
			saved, err := s.o.Get(ctx, s.bucket, key)
			if err != nil {
				return err
			}
			if s.c.Hash(saved) != s.c.Hash(d.Ciphertext) {
				return domain.VendorDocumentIntegrity
			}
			if err = r.Relocate(ctx, *d, s.bucket, key, s.v.ID()); err != nil {
				return err
			}
			moved = true
			return nil
		})
		if err != nil {
			return count, err
		}
		if !moved {
			return count, nil
		}
		count++
	}
}
