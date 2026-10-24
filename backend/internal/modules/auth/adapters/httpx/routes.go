package httpx

import (
	"github.com/go-chi/chi/v5"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/application"
)

type Services struct {
	Offboarding  *application.OffboardingService
	Organisation *application.OrganisationService
	Credentials  *application.CredentialService
	Recovery     *application.RecoveryService
	Profile      *application.ProfileService
	Sessions     *application.SessionService
	Users        *application.UserService
	Roles        *application.RoleService
	Account      *application.AccountService
}

func Register(router chi.Router, services Services) {
	if services.Offboarding != nil {
		h := &offboardingHandler{services.Offboarding}
		router.Get("/api/v1/auth/organisation/offboarding", h.list)
		router.Post("/api/v1/auth/organisation/offboarding", h.create)
		router.Get("/api/v1/auth/organisation/offboarding/{case_id}", h.get)
		router.Get("/api/v1/auth/organisation/offboarding/{case_id}/candidates", h.candidates)
		router.Get("/api/v1/auth/organisation/offboarding/{case_id}/history", h.history)
		router.Post("/api/v1/auth/organisation/offboarding/{case_id}/{action}", h.change)
	}
	if services.Organisation != nil {
		org := &organisationHandler{services.Organisation}
		router.Get("/api/v1/auth/organisation/departments", org.departments)
		router.Post("/api/v1/auth/organisation/departments", org.saveDepartment)
		router.Patch("/api/v1/auth/organisation/departments/{department_id}", org.saveDepartment)
		router.Get("/api/v1/auth/organisation/employees", org.employees)
		router.Patch("/api/v1/auth/organisation/employees/{user_id}", org.saveEmployee)
		router.Get("/api/v1/auth/organisation/team", org.team)
		router.Get("/api/v1/auth/organisation/team/{user_id}/work", org.work)
	}
	if services.Recovery != nil {
		recovery := &recoveryHandler{services.Recovery}
		router.With(recoveryLimit()).Post("/api/v1/auth/forgot-password", recovery.request)
		router.With(recoveryLimit()).Post("/api/v1/auth/reset-password", recovery.complete)
	}
	sessions := &sessionHandler{services.Sessions}
	users := &userHandler{services.Users, services.Credentials}
	roles := &roleHandler{services.Roles}
	router.Post("/api/v1/auth/login", sessions.login)
	router.Post("/api/v1/auth/refresh", sessions.refresh)
	router.Post("/api/v1/auth/logout", sessions.logout)
	router.Get("/api/v1/auth/me", sessions.me)
	router.Post("/api/v1/auth/activity", sessions.activity)
	router.Get("/api/v1/users", users.users)
	router.Post("/api/v1/users", users.createUser)
	router.Get("/api/v1/users/online/activity", users.online)
	router.Get("/api/v1/users/{user_id}", users.user)
	router.Patch("/api/v1/users/{user_id}", users.updateUser)
	router.Post("/api/v1/users/{user_id}/password", users.password)
	router.Post("/api/v1/users/{user_id}/deactivate", users.deactivate)
	router.Get("/api/v1/roles", roles.roles)
	router.Get("/api/v1/roles/permissions", roles.permissions)
	router.Post("/api/v1/roles", roles.createRole)
	router.Post("/api/v1/roles/finance-presets", roles.finance)
	router.Patch("/api/v1/roles/{role_id}", roles.updateRole)
	account := &accountHandler{services.Account}
	router.Get("/api/v1/auth/me/username", account.usernameAvailable)
	router.Patch("/api/v1/auth/me/username", account.username)
	router.Post("/api/v1/auth/me/password", account.password)
	router.Patch("/api/v1/auth/me/email", account.email)
	profile := &profileHandler{services.Profile}
	router.Get("/api/v1/auth/user_profile/me", profile.me)
	router.Patch("/api/v1/auth/user_profile/me", profile.update)
	router.Put("/api/v1/auth/user_profile/me/photo", profile.photo)
	router.Delete("/api/v1/auth/user_profile/me/photo", profile.remove)
	router.Get("/api/v1/auth/user_profile/{user_id}", profile.user)
	router.Get("/api/v1/auth/user_profile/{user_id}/photo", profile.image)
}
