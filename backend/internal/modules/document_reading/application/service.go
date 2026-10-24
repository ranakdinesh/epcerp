package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/ports"
)

type Service struct {
	r     ports.Repository
	c     ports.Cipher
	v     ports.Values
	i     ports.Inspector
	codec ports.Codec
}

func New(r ports.Repository, c ports.Cipher, v ports.Values, i ports.Inspector, codec ports.Codec) *Service {
	return &Service{r, c, v, i, codec}
}
func (s *Service) Submit(ctx context.Context, in domain.Input) (domain.Document, error) {
	name, format, e := domain.Filename(in.Filename)
	if e != nil {
		return domain.Document{}, e
	}
	in.Filename = name
	in.Format = format
	if e = domain.Validate(in); e != nil {
		return domain.Document{}, e
	}
	if e = s.i.Inspect(in.Original, format); e != nil {
		return domain.Document{}, e
	}
	sealed, hash, e := s.c.Seal(in.Original)
	if e != nil {
		return domain.Document{}, e
	}
	in.SHA256 = hash
	return s.r.Submit(ctx, in, sealed, s.v.ID())
}
func (s *Service) Get(ctx context.Context, module, entity, id string) (domain.Document, error) {
	return s.r.Get(ctx, module, entity, id)
}
func (s *Service) List(ctx context.Context, module, entity string) ([]domain.Document, error) {
	return s.r.List(ctx, module, entity)
}
func (s *Service) Artifact(ctx context.Context, module, entity, id, kind string) ([]byte, error) {
	d, e := s.r.Get(ctx, module, entity, id)
	if e != nil {
		return nil, e
	}
	if kind == "original" {
		return s.c.Open(d.Original, d.SHA256)
	}
	if kind != "result" && kind != "text" {
		return nil, domain.Missing
	}
	if d.Status != "ready" {
		return nil, domain.Conflict
	}
	cipher, hash, e := s.r.Result(ctx, id)
	if e != nil {
		return nil, e
	}
	plain, e := s.c.Open(cipher, hash)
	if e != nil {
		return nil, e
	}
	if kind == "text" {
		return s.codec.Text(plain)
	}
	return plain, nil
}
func (s *Service) Retry(ctx context.Context, module, entity, id string, version int32, actor string) (domain.Document, error) {
	d, e := s.r.Get(ctx, module, entity, id)
	if e != nil {
		return d, e
	}
	if e = domain.Retry(d, version); e != nil {
		return d, e
	}
	out, e := s.r.Retry(ctx, id, version, actor)
	if e != nil {
		return out, e
	}
	return out, s.r.Event(ctx, out, "retry", actor)
}
func (s *Service) History(ctx context.Context, module, entity, id string) ([]domain.Event, error) {
	if _, e := s.r.Get(ctx, module, entity, id); e != nil {
		return nil, e
	}
	return s.r.History(ctx, id)
}

func (s *Service) Prepare(filename string, data []byte) (domain.Prepared, error) {
	name, format, e := domain.Filename(filename)
	if e != nil {
		return domain.Prepared{}, e
	}
	if len(data) < 1 || len(data) > domain.MaxBytes {
		return domain.Prepared{}, domain.Invalid("Source exceeds supported bounds")
	}
	if e = s.i.Inspect(data, format); e != nil {
		return domain.Prepared{}, e
	}
	sealed, hash, e := s.c.Seal(data)
	if e != nil {
		return domain.Prepared{}, e
	}
	return domain.Prepared{Filename: name, Format: format, SHA256: hash, CipherSHA256: s.c.Hash(sealed), Size: int64(len(data)), Ciphertext: sealed}, nil
}
func (s *Service) SubmitPrepared(ctx context.Context, in domain.Input, p domain.Prepared) (domain.Document, error) {
	if !domain.ValidPreparedContext(in, p) || s.c.Hash(p.Ciphertext) != p.CipherSHA256 {
		return domain.Document{}, domain.Invalid("Prepared source context or integrity is invalid")
	}
	in.Filename = p.Filename
	in.Format = p.Format
	in.SHA256 = p.SHA256
	in.SizeBytes = p.Size
	return s.r.Submit(ctx, in, p.Ciphertext, s.v.ID())
}
