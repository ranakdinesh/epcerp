package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

type TSCFormats struct {
	unit   ports.TSCFormatUnit
	auth   ports.Authorizer
	values ports.Values
}

func NewTSCFormats(u ports.TSCFormatUnit, a ports.Authorizer, v ports.Values) *TSCFormats {
	return &TSCFormats{u, a, v}
}
func (s *TSCFormats) within(ctx context.Context, token string, write bool, f func(context.Context, ports.TSCFormatRepository, string) error) error {
	p, e := s.auth.Resolve(ctx, token)
	if e != nil {
		return e
	}
	if !p.Active || p.ID == "" {
		return domain.Unauthorized
	}
	return s.unit.Within(ctx, func(ctx context.Context, r ports.TSCFormatRepository) error {
		if e := r.Authorize(ctx, p.ID, write); e != nil {
			return e
		}
		return f(ctx, r, p.ID)
	})
}
func (s *TSCFormats) List(ctx context.Context, token string) (out []domain.TSCFormat, e error) {
	e = s.within(ctx, token, false, func(ctx context.Context, r ports.TSCFormatRepository, _ string) error {
		var e error
		out, e = r.List(ctx, false)
		return e
	})
	return
}
func (s *TSCFormats) Get(ctx context.Context, token, id string) (out domain.TSCFormat, e error) {
	if _, e = domain.ID(id); e != nil {
		return
	}
	e = s.within(ctx, token, false, func(ctx context.Context, r ports.TSCFormatRepository, _ string) error {
		var e error
		out, e = r.Get(ctx, id)
		return e
	})
	return
}
func (s *TSCFormats) Save(ctx context.Context, token, id string, in domain.TSCFormatInput) (out domain.TSCFormat, e error) {
	if id != "" {
		if _, e = domain.ID(id); e != nil {
			return
		}
		if in.Version < 1 {
			return out, domain.Invalid("The loaded format version is required")
		}
	} else if in.Version != 0 {
		return out, domain.Invalid("New formats must use version zero")
	}
	e = s.within(ctx, token, true, func(ctx context.Context, r ports.TSCFormatRepository, actor string) error {
		old := domain.TSCFormat{}
		if id == "" {
			// Copies may retain reviewed extraction identities only for unchanged uploaded questions.
			var err error
			old, err = r.Get(ctx, domain.DefaultTSCFormatID)
			if err != nil {
				return err
			}
		}
		if id != "" {
			var e error
			old, e = r.Get(ctx, id)
			if e != nil {
				return e
			}
			if old.Version != in.Version {
				return domain.CaptureFailure(409, "stale_format", "This format changed. Reload it before saving")
			}
		}
		for i := range in.Questions {
			q := &in.Questions[i]
			same := false
			for _, previous := range old.Questions {
				if previous.ID == q.ID && previous.Text == q.Text && previous.Section == q.Section && previous.ManualReview == q.ManualReview {
					same = true
					break
				}
			}
			// Legacy IDs drive reviewed extraction rules. Never retain one after changing its meaning.
			if !same {
				if _, err := domain.ID(q.ID); err != nil {
					q.ID = s.values.ID()
				}
				q.Row = 0
			}
		}
		if e := domain.ValidateTSCFormat(&in); e != nil {
			return e
		}
		if id == "" {
			id = s.values.ID()
		}
		var e error
		out, e = r.Save(ctx, id, actor, in)
		return e
	})
	return
}
