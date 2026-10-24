package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

// CompleteReview confirms preparation of one immutable PDF, never external dispatch.
func (s *LOIDocumentsService) CompleteReview(ctx context.Context, token, p, attempt, id string, q domain.LOIReviewCommand) (out *domain.LOIReview, e error) {
	a, e := s.actor(ctx, token, p, attempt)
	if e != nil {
		return nil, e
	}
	if e = domain.WorkflowID(id); e != nil {
		return nil, e
	}
	if e = q.Validate(); e != nil {
		return nil, e
	}
	e = s.u.WithinAwards(ctx, func(ctx context.Context, x ports.AwardsSession) error {
		if e := x.Context.Authorize(ctx, a, p, true); e != nil {
			return e
		}
		if e := x.Awards.Lock(ctx, p); e != nil {
			return e
		}
		docs, e := x.Documents.List(ctx, p, attempt)
		if e != nil {
			return e
		}
		if len(docs) == 0 || docs[0].ID != id {
			return domain.WorkflowInvalid("document", "A newer PDF is available. Review the latest version before completing")
		}
		d := docs[0]
		candidates, e := x.Context.Candidates(ctx, p)
		if e != nil {
			return e
		}
		current := false
		for _, c := range candidates {
			if c.Attempt.ID == attempt {
				hash, e := s.fingerprint.Approval(c.Attempt)
				if e != nil {
					return e
				}
				current = hash == d.Content.SourceHash
			}
		}
		if !current {
			return domain.WorkflowInvalid("approval", "The approved source changed. Refresh the award and generate its current LOI")
		}
		if d.Review != nil {
			out = d.Review
			return nil
		}
		v := domain.LOIReview{LOIReviewCommand: q, ID: s.v.ID(), DocumentID: id, CreatedBy: a.ID, CreatedAt: s.v.Now()}
		if e := x.Documents.CompleteReview(ctx, p, v); e != nil {
			return e
		}
		out, e = x.Documents.Review(ctx, id)
		return e
	})
	if e != nil {
		out = nil
	}
	return
}
