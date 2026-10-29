package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/ports"
)

type Worker struct {
	u     ports.UnitOfWork
	c     ports.Cipher
	v     ports.Values
	e     ports.Engine
	codec ports.Codec
}

func NewWorker(u ports.UnitOfWork, c ports.Cipher, v ports.Values, e ports.Engine, codec ports.Codec) *Worker {
	return &Worker{u, c, v, e, codec}
}
func (w *Worker) Once(ctx context.Context) (bool, error) {
	var job *domain.Document
	e := w.u.Within(ctx, func(ctx context.Context, r ports.Repository) error {
		var err error
		job, err = r.Claim(ctx, w.v.ID())
		if err != nil || job == nil {
			return err
		}
		return r.Event(ctx, *job, job.Status, "")
	})
	if e != nil || job == nil {
		return job != nil, e
	}
	if job.Status == "failed" {
		return true, nil
	}
	parentCtx := ctx
	stopClaim := func() {}
	if keeper, ok := w.u.(ports.LeaseKeeper); ok {
		ctx, stopClaim = keeper.KeepClaim(ctx, *job)
		defer stopClaim()
	}
	data, e := w.c.Open(job.Original, job.SHA256)
	result := domain.Result{}
	if e == nil {
		result, e = w.e.Read(ctx, data, job.Format, job.SHA256)
	}
	if e == nil {
		e = domain.CheckResult(result)
		if e == nil && result.SourceSHA256 != job.SHA256 {
			e = domain.Invalid("Reader result does not match the original document")
		}
	}
	if ctx.Err() != nil {
		return true, ctx.Err()
	}
	failure := ""
	var raw, encrypted []byte
	hash := ""
	if e == nil {
		raw, e = w.codec.Encode(result)
	}
	if e == nil {
		encrypted, hash, e = w.c.Seal(raw)
	}
	if e != nil {
		failure = "Document could not be read. Check for damage, password protection, scan quality or processing limits."
		if d, ok := e.(*domain.Error); ok {
			failure = d.Detail
		}
	}
	stopClaim()
	ctx = parentCtx
	e = w.u.Within(ctx, func(ctx context.Context, r ports.Repository) error {
		out, err := r.Finish(ctx, *job, encrypted, hash, result, failure)
		if err != nil {
			return err
		}
		return r.Event(ctx, out, out.Status, "")
	})
	return true, e
}
