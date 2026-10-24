package outbound

import (
	"context"
	"errors"
	reading "github.com/ranakdinesh/epcERP/internal/modules/document_reading/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"mime"
	"path/filepath"
)

type Documents struct {
	r reading.Reader
	v reading.Validator
}

func NewDocuments(r reading.Reader, v reading.Validator) *Documents { return &Documents{r, v} }
func translate(e error) error {
	var d *reading.Error
	if errors.As(e, &d) {
		return &domain.Error{Status: d.Status, Detail: d.Detail}
	}
	return e
}
func source(v reading.Document) domain.Source {
	return domain.Source{ID: v.ID, Filename: v.Filename, Format: v.Format, Size: v.Size, SHA256: v.SHA256, Status: v.Status, Version: v.Version, Attempts: v.Attempts, Error: v.Error, SegmentCount: v.SegmentCount, Warnings: v.Warnings}
}
func (d *Documents) Validate(name string, b []byte) (string, string, error) {
	n, h, e := d.v.Validate(name, b)
	return n, h, translate(e)
}
func (d *Documents) Submit(ctx context.Context, entity, actor, name string, b []byte) (domain.Source, error) {
	v, e := d.r.Submit(ctx, reading.Input{Module: "tender_fms", EntityID: entity, ActorID: actor, Filename: name, Original: b})
	return source(v), translate(e)
}
func (d *Documents) Get(ctx context.Context, entity, id string) (domain.Source, error) {
	v, e := d.r.Get(ctx, "tender_fms", entity, id)
	return source(v), translate(e)
}
func (d *Documents) List(ctx context.Context, entity string) ([]domain.Source, error) {
	rows, e := d.r.List(ctx, "tender_fms", entity)
	out := []domain.Source{}
	for _, v := range rows {
		out = append(out, source(v))
	}
	return out, translate(e)
}
func (d *Documents) Artifact(ctx context.Context, entity, id, kind string) (domain.Artifact, error) {
	v, e := d.r.Get(ctx, "tender_fms", entity, id)
	if e != nil {
		return domain.Artifact{}, translate(e)
	}
	b, e := d.r.Artifact(ctx, "tender_fms", entity, id, kind)
	name := v.Filename
	content := mime.TypeByExtension(filepath.Ext(name))
	if kind == "text" {
		name += ".txt"
		content = "text/plain; charset=utf-8"
	}
	if kind == "result" {
		name += ".reading.json"
		content = "application/json"
	}
	return domain.Artifact{Data: b, Name: name, ContentType: content}, translate(e)
}
func (d *Documents) Retry(ctx context.Context, entity, id string, version int32, actor string) (domain.Source, error) {
	v, e := d.r.Retry(ctx, "tender_fms", entity, id, version, actor)
	return source(v), translate(e)
}
func (d *Documents) History(ctx context.Context, entity, id string) ([]domain.Event, error) {
	rows, e := d.r.History(ctx, "tender_fms", entity, id)
	out := []domain.Event{}
	for _, v := range rows {
		out = append(out, domain.Event{ID: v.ID, DocumentID: v.DocumentID, Action: "reading_" + v.Action, Version: v.Version, ActorID: v.ActorID, CreatedAt: v.CreatedAt})
	}
	return out, translate(e)
}
