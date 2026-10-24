package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

type OffboardingService struct {
	unit   ports.UnitOfWork[ports.OffboardingRepository]
	values ports.Tokens
}

func NewOffboarding(u ports.UnitOfWork[ports.OffboardingRepository], v ports.Tokens) *OffboardingService {
	return &OffboardingService{u, v}
}
func (s *OffboardingService) actor(ctx context.Context, r ports.OffboardingRepository, token string, write bool) (*domain.User, error) {
	if write {
		if e := r.Lock(ctx, "auth.administration"); e != nil {
			return nil, e
		}
	}
	return resolve(ctx, r, s.values, token)
}
func (s *OffboardingService) Cases(ctx context.Context, token string) ([]domain.OffboardingCase, error) {
	out := []domain.OffboardingCase{}
	e := s.unit.Within(ctx, func(ctx context.Context, r ports.OffboardingRepository) error {
		a, e := s.actor(ctx, r, token, false)
		if e != nil {
			return e
		}
		admin := domain.HasPermission(*a, "admin.offboarding.read")
		if !admin && !domain.HasPermission(*a, "account.offboarding.read") {
			return domain.ErrForbidden
		}
		out, e = r.Cases(ctx, a.ID, admin)
		return e
	})
	return out, e
}
func liveEmployee(ctx context.Context, r ports.OrganisationRepository, id string) (domain.Employee, error) {
	all, e := r.Employees(ctx)
	if e != nil {
		return domain.Employee{}, e
	}
	for _, v := range all {
		if v.ID == id {
			return v, nil
		}
	}
	return domain.Employee{}, domain.ErrUserMissing
}
func (s *OffboardingService) scope(ctx context.Context, r ports.OffboardingRepository, a domain.User, c domain.OffboardingCase, managerOnly bool) error {
	profile, e := liveEmployee(ctx, r, c.UserID)
	if e != nil {
		return e
	}
	if managerOnly {
		if a.ID != profile.ManagerID || a.ID != c.ManagerID || a.ID == c.UserID || !domain.HasPermission(a, "account.offboarding.write") {
			return domain.ErrForbidden
		}
	} else if !domain.HasPermission(a, "admin.offboarding.read") && !(a.ID == profile.ManagerID && a.ID == c.ManagerID && domain.HasPermission(a, "account.offboarding.read")) {
		return domain.ErrForbidden
	}
	return nil
}
func (s *OffboardingService) Get(ctx context.Context, token, id string) (*domain.OffboardingCase, error) {
	var out *domain.OffboardingCase
	e := s.unit.Within(ctx, func(ctx context.Context, r ports.OffboardingRepository) error {
		a, e := s.actor(ctx, r, token, false)
		if e != nil {
			return e
		}
		out, e = r.Case(ctx, id)
		if e != nil {
			return e
		}
		if out == nil {
			return &domain.Error{Kind: domain.Missing, Detail: "Offboarding case not found."}
		}
		return s.scope(ctx, r, *a, *out, false)
	})
	if e != nil {
		return nil, e
	}
	return out, nil
}
func (s *OffboardingService) Create(ctx context.Context, token, userID, reason string) (domain.OffboardingCase, error) {
	var out domain.OffboardingCase
	if !domain.ValidResetReason(reason) {
		return out, &domain.Error{Kind: domain.Invalid, Detail: "Provide a departure reason of 10 to 1000 characters."}
	}
	e := s.unit.Within(ctx, func(ctx context.Context, r ports.OffboardingRepository) error {
		a, e := s.actor(ctx, r, token, true)
		if e != nil {
			return e
		}
		if !domain.HasPermission(*a, "admin.offboarding.write") {
			return domain.ErrForbidden
		}
		if a.ID == userID {
			return domain.ErrSelfDeactivation
		}
		u, e := r.User(ctx, userID, true)
		if e != nil {
			return e
		}
		if u == nil {
			return domain.ErrUserMissing
		}
		if !u.IsActive {
			return &domain.Error{Kind: domain.Conflict, Detail: "This account is already inactive."}
		}
		profile, e := liveEmployee(ctx, r, userID)
		if e != nil {
			return e
		}
		if profile.Version == 0 || profile.ManagerID == "" {
			return &domain.Error{Kind: domain.Conflict, Detail: "Set up the employee's reporting manager before starting offboarding. A company root needs an authorised successor reporting arrangement."}
		}
		manager, e := r.User(ctx, profile.ManagerID, true)
		if e != nil {
			return e
		}
		if manager == nil || !manager.IsActive || manager.MustChangePassword {
			return &domain.Error{Kind: domain.Conflict, Detail: "The reporting manager must have an active, ready account."}
		}
		work, e := r.WorkInventory(ctx, userID)
		if e != nil {
			return e
		}
		out = domain.OffboardingCase{ID: s.values.ID(), UserID: userID, ManagerID: profile.ManagerID, ProfileVersion: profile.Version, State: "planning", Reason: reason, Items: []domain.HandoverItem{}}
		for _, w := range work {
			out.Items = append(out.Items, domain.HandoverItem{Responsibility: w})
		}
		out, e = r.SaveCase(ctx, out, a.ID)
		if e != nil {
			return e
		}
		return r.RecordOffboarding(ctx, out, s.values.ID(), a.ID, "opened", reason)
	})
	return out, e
}
func (s *OffboardingService) Candidates(ctx context.Context, token, id string) ([]domain.User, error) {
	out := []domain.User{}
	e := s.unit.Within(ctx, func(ctx context.Context, r ports.OffboardingRepository) error {
		a, e := s.actor(ctx, r, token, false)
		if e != nil {
			return e
		}
		c, e := r.Case(ctx, id)
		if e != nil {
			return e
		}
		if c == nil {
			return &domain.Error{Kind: domain.Missing, Detail: "Offboarding case not found."}
		}
		if e = s.scope(ctx, r, *a, *c, false); e != nil {
			return e
		}
		all, e := r.Employees(ctx)
		if e != nil {
			return e
		}
		for _, p := range all {
			if !p.IsActive || p.ID == c.UserID {
				continue
			}
			u, e := r.User(ctx, p.ID, false)
			if e != nil {
				return e
			}
			if u != nil && !u.MustChangePassword {
				out = append(out, *u)
			}
		}
		return nil
	})
	return out, e
}
func (s *OffboardingService) Change(ctx context.Context, token, id, action string, in domain.HandoverPlan) (domain.OffboardingCase, error) {
	var out domain.OffboardingCase
	if !domain.ValidResetReason(in.Reason) || in.Version < 1 {
		return out, &domain.Error{Kind: domain.Invalid, Detail: "Provide the current case version and a reason of 10 to 1000 characters."}
	}
	e := s.unit.Within(ctx, func(ctx context.Context, r ports.OffboardingRepository) error {
		a, e := s.actor(ctx, r, token, true)
		if e != nil {
			return e
		}
		c, e := r.Case(ctx, id)
		if e != nil {
			return e
		}
		if c == nil {
			return &domain.Error{Kind: domain.Missing, Detail: "Offboarding case not found."}
		}
		if !c.Open() || c.Version != in.Version {
			return domain.ErrOffboardingStale
		}
		profile, e := liveEmployee(ctx, r, c.UserID)
		if e != nil {
			return e
		}
		if action == "refresh" || action == "cancel" || action == "deactivate" {
			if !domain.HasPermission(*a, "admin.offboarding.write") {
				return domain.ErrForbidden
			}
		} else if e = s.scope(ctx, r, *a, *c, true); e != nil {
			return e
		}
		switch action {
		case "cancel":
			c.State = "cancelled"
		case "refresh":
			if profile.ManagerID == "" {
				return &domain.Error{Kind: domain.Conflict, Detail: "Assign a live reporting manager before refreshing this case."}
			}
			work, e := r.WorkInventory(ctx, c.UserID)
			if e != nil {
				return e
			}
			for _, w := range work {
				found := false
				for i, item := range c.Items {
					if item.Responsibility.Same(w) {
						c.Items[i].Responsibility = w
						found = true
					}
				}
				if !found {
					c.Items = append(c.Items, domain.HandoverItem{Responsibility: w})
				}
			}
			// Refresh transferred references too: a changed source capability list must
			// be visible to the manager before selecting and approving again.
			for i, item := range c.Items {
				current, e := r.WorkCurrent(ctx, item.Responsibility)
				if e != nil {
					return e
				}
				for _, w := range current {
					if w.OwnerID == item.ReplacementID || len(current) == 1 {
						c.Items[i].Responsibility = w
						break
					}
				}
				c.Items[i].ApprovedVersion = 0
			}
			c.ManagerID = profile.ManagerID
			c.ProfileVersion = profile.Version
			c.State = "planning"
			c.ApprovalByID = ""
			c.ApprovedAt = nil
		case "plan":
			if c.State != "planning" || c.ProfileVersion != profile.Version {
				return domain.ErrOffboardingStale
			}
			seen := map[int]bool{}
			for _, selection := range in.Selections {
				found := -1
				for i, item := range c.Items {
					w := item.Responsibility
					if w.Source == selection.Source && w.Kind == selection.Kind && w.ID == selection.ID {
						found = i
					}
				}
				if found < 0 || seen[found] {
					return &domain.Error{Kind: domain.Invalid, Detail: "Choose each responsibility once from the saved inventory."}
				}
				seen[found] = true
				if selection.ReplacementID != "" {
					u, e := r.User(ctx, selection.ReplacementID, true)
					if e != nil {
						return e
					}
					if u == nil || !domain.EligibleReplacement(*u, c.UserID, c.Items[found].Responsibility) {
						return &domain.Error{Kind: domain.Invalid, Detail: "Replacement must be active and already hold all required permissions."}
					}
				}
				c.Items[found].ReplacementID = selection.ReplacementID
			}
		case "approve":
			if c.State != "planning" || c.ProfileVersion != profile.Version {
				return domain.ErrOffboardingStale
			}
			if e = s.verify(ctx, r, *c, false); e != nil {
				return e
			}
			at := domain.At(s.values.Now())
			c.State = "approved"
			c.ApprovalByID = a.ID
			c.ApprovedAt = &at
			for i, item := range c.Items {
				current, e := r.WorkCurrent(ctx, item.Responsibility)
				if e != nil {
					return e
				}
				c.Items[i].ApprovedVersion = -1
				for _, w := range current {
					if w.OwnerID == item.ReplacementID {
						c.Items[i].ApprovedVersion = w.Version
					}
				}
			}
		case "deactivate":
			if c.State != "approved" || c.ProfileVersion != profile.Version || c.ManagerID != profile.ManagerID || c.ApprovalByID != profile.ManagerID {
				return domain.ErrOffboardingStale
			}
			if a.ID == c.UserID {
				return domain.ErrSelfDeactivation
			}
			manager, e := r.User(ctx, c.ManagerID, true)
			if e != nil {
				return e
			}
			if manager == nil || !manager.IsActive || manager.MustChangePassword {
				return domain.ErrOffboardingStale
			}
			u, e := r.User(ctx, c.UserID, true)
			if e != nil {
				return e
			}
			if u == nil {
				return domain.ErrUserMissing
			}
			if !u.IsActive {
				return domain.ErrOffboardingStale
			}
			if u.IsSuperuser {
				n, e := r.ActiveSuperusers(ctx)
				if e != nil {
					return e
				}
				if n <= 1 {
					return domain.ErrLastSuperadmin
				}
			}
			if e = s.verify(ctx, r, *c, true); e != nil {
				return e
			}
			if e = r.Deactivate(ctx, c.UserID, a.ID); e != nil {
				return e
			}
			if e = r.RevokeUserSessions(ctx, c.UserID); e != nil {
				return e
			}
			c.State = "completed"
		default:
			return &domain.Error{Kind: domain.Invalid, Detail: "Unknown offboarding action."}
		}
		out, e = r.SaveCase(ctx, *c, a.ID)
		if e != nil {
			return e
		}
		return r.RecordOffboarding(ctx, out, s.values.ID(), a.ID, action, in.Reason)
	})
	return out, e
}
func (s *OffboardingService) verify(ctx context.Context, r ports.OffboardingRepository, c domain.OffboardingCase, approved bool) error {
	remaining, e := r.WorkInventory(ctx, c.UserID)
	if e != nil {
		return e
	}
	if len(remaining) > 0 {
		return &domain.Error{Kind: domain.Conflict, Detail: "Complete or transfer all live responsibilities in their owning workspaces before manager approval and deactivation."}
	}
	for _, item := range c.Items {
		if item.ReplacementID != "" {
			if _, e := r.User(ctx, item.ReplacementID, true); e != nil {
				return e
			}
		}
		current, e := r.WorkCurrent(ctx, item.Responsibility)
		if e != nil {
			return e
		}
		if len(current) == 0 {
			if approved && item.ApprovedVersion != -1 {
				return domain.ErrOffboardingStale
			}
			continue
		}
		if item.ReplacementID == "" {
			return &domain.Error{Kind: domain.Conflict, Detail: "The manager must select a replacement for every live responsibility."}
		}
		found := false
		for _, w := range current {
			if w.OwnerID == item.ReplacementID {
				found = true
				if approved && w.Version != item.ApprovedVersion {
					return domain.ErrOffboardingStale
				}
			}
		}
		if !found {
			return &domain.Error{Kind: domain.Conflict, Detail: "A live responsibility has not been transferred to the manager-selected replacement."}
		}
		u, e := r.User(ctx, item.ReplacementID, true)
		if e != nil {
			return e
		}
		if u == nil {
			return &domain.Error{Kind: domain.Conflict, Detail: "The replacement account no longer exists."}
		}
		for _, live := range current {
			if live.OwnerID == item.ReplacementID && !domain.EligibleReplacement(*u, c.UserID, live) {
				return &domain.Error{Kind: domain.Conflict, Detail: "A replacement no longer has the required active account and permissions."}
			}
		}
	}
	return nil
}

func (s *OffboardingService) History(ctx context.Context, token, id string) ([]domain.OffboardingChange, error) {
	out := []domain.OffboardingChange{}
	e := s.unit.Within(ctx, func(ctx context.Context, r ports.OffboardingRepository) error {
		a, e := s.actor(ctx, r, token, false)
		if e != nil {
			return e
		}
		c, e := r.Case(ctx, id)
		if e != nil {
			return e
		}
		if c == nil {
			return &domain.Error{Kind: domain.Missing, Detail: "Offboarding case not found."}
		}
		if e = s.scope(ctx, r, *a, *c, false); e != nil {
			return e
		}
		out, e = r.OffboardingHistory(ctx, id)
		return e
	})
	return out, e
}
