package application

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

// Registration owns the initial purchase assignment for an already-authorized project creation.
type WorkspaceRegistration struct {
	Repo   ports.WorkspaceRepository
	Users  ports.WorkspaceUsers
	Values ports.WorkspaceValues
}

func (r WorkspaceRegistration) Candidates(ctx context.Context) ([]domain.WorkspaceCandidate, error) {
	users, e := r.Users.List(ctx)
	out := []domain.WorkspaceCandidate{}
	for _, c := range users {
		if c.Eligible() {
			out = append(out, c)
		}
	}
	return out, e
}
func (r WorkspaceRegistration) Owner(ctx context.Context, id string) (string, error) {
	w, e := r.Repo.Get(ctx, id)
	if !w.Active {
		return "", e
	}
	return w.ManagerID, e
}
func (r WorkspaceRegistration) Initialize(ctx context.Context, projectID, userID, actorID string) error {
	c, e := r.Users.Get(ctx, userID)
	if e != nil {
		return e
	}
	if c == nil || !c.Eligible() {
		return domain.WorkspaceIneligible
	}
	w, e := r.Repo.Get(ctx, projectID)
	if e != nil {
		return e
	}
	if w.Version != 0 {
		return domain.WorkspaceStale
	}
	change := domain.WorkspaceChange{ManagerID: userID, Version: 0, Reason: "Initial Procurement Executive assignment at project creation"}
	if change, e = change.Validate(); e != nil {
		return e
	}
	w, e = r.Repo.Save(ctx, projectID, change, actorID)
	if e != nil {
		return e
	}
	return r.Repo.Record(ctx, domain.WorkspaceHistory{ID: r.Values.ID(), ProjectID: projectID, ManagerID: userID, ActorID: actorID, Reason: change.Reason, Version: w.Version, CreatedAt: w.CreatedAt})
}
