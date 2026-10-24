package httpx

import (
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"net/http"
)

type organisationHandler struct {
	service *application.OrganisationService
}

func (h *organisationHandler) departments(w http.ResponseWriter, r *http.Request) {
	values, e := h.service.Departments(r.Context(), bearer(r))
	if failed(w, r, e) {
		return
	}
	out := []dtos.DepartmentResponse{}
	for _, v := range values {
		out = append(out, dtos.Department(v))
	}
	write(w, 200, out)
}
func (h *organisationHandler) employees(w http.ResponseWriter, r *http.Request) {
	values, e := h.service.Employees(r.Context(), bearer(r))
	if failed(w, r, e) {
		return
	}
	out := []dtos.EmployeeResponse{}
	for _, v := range values {
		out = append(out, dtos.Employee(v))
	}
	write(w, 200, out)
}
func (h *organisationHandler) team(w http.ResponseWriter, r *http.Request) {
	values, e := h.service.Team(r.Context(), bearer(r))
	if failed(w, r, e) {
		return
	}
	out := []dtos.EmployeeResponse{}
	for _, v := range values {
		out = append(out, dtos.Employee(v))
	}
	write(w, 200, out)
}
func (h *organisationHandler) saveDepartment(w http.ResponseWriter, r *http.Request) {
	id := ""
	if r.Method == "PATCH" {
		var ok bool
		id, ok = pathID(w, r, "department_id")
		if !ok {
			return
		}
	}
	in, ok := requestBody(w, r, dtos.ParseDepartment)
	if !ok {
		return
	}
	value, e := h.service.SaveDepartment(r.Context(), bearer(r), id, in)
	if failed(w, r, e) {
		return
	}
	audit(r.Context(), "organisation.department_updated", false)
	status := 200
	if r.Method == "POST" {
		status = 201
	}
	write(w, status, dtos.Department(value))
}
func (h *organisationHandler) saveEmployee(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "user_id")
	if !ok {
		return
	}
	in, ok := requestBody(w, r, dtos.ParseEmployee)
	if !ok {
		return
	}
	value, e := h.service.SaveEmployee(r.Context(), bearer(r), id, in)
	if failed(w, r, e) {
		return
	}
	audit(r.Context(), "organisation.employee_updated", false)
	write(w, 200, dtos.Employee(value))
}

func (h *organisationHandler) work(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "user_id")
	if !ok {
		return
	}
	values, e := h.service.TeamWork(r.Context(), bearer(r), id)
	if failed(w, r, e) {
		return
	}
	out := []dtos.WorkResponse{}
	for _, v := range values {
		out = append(out, dtos.Work(v))
	}
	write(w, 200, out)
}
