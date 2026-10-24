package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

type deniedTokens struct{ ports.Tokens }

func (deniedTokens) Subject(string) (string, error) { return "", domain.ErrUnauthorized }

type unit[R any] struct{}

func (unit[R]) Within(ctx context.Context, work func(context.Context, R) error) error {
	var repo R
	return work(ctx, repo)
}
func TestBoundaryValidationAndDenialResponses(t *testing.T) {
	router := chi.NewRouter()
	Register(router, Services{Sessions: application.NewSessions(unit[ports.SessionRepository]{}, nil, deniedTokens{}, domain.Policy{}), Users: application.NewUsers(unit[ports.UserRepository]{}, nil, deniedTokens{}, domain.Policy{}), Roles: application.NewRoles(unit[ports.RoleRepository]{}, deniedTokens{}), Account: application.NewAccount(unit[ports.AccountRepository]{}, nil, deniedTokens{})})
	for _, test := range []struct {
		method, path, body string
		code               int
	}{
		{"POST", "/api/v1/auth/login", `{"Identifier":"abc","password":"value123456"}`, 422},
		{"POST", "/api/v1/auth/login", `{"identifier":"abc","password":"value123456","extra":true}`, 422},
		{"POST", "/api/v1/auth/login", `{"identifier":123,"password":"value123456"}`, 422},
		{"POST", "/api/v1/auth/login", `{"identifier":"abc","password":"value123456"}{}`, 422},
		{"POST", "/api/v1/auth/login", `null`, 422},
		{"POST", "/api/v1/auth/refresh", `{"refresh_token":null}`, 422},
		{"POST", "/api/v1/auth/activity", `{"current_page":"https://external.example"}`, 422},
		{"POST", "/api/v1/users", `{"username":"user","email":"user@example.invalid","password":"Value123456!","role_names":[null]}`, 422},
		{"PATCH", "/api/v1/roles/12345678-1234-4234-8234-123456789abc", `{"is_active":"invalid"}`, 422},
		{"GET", "/api/v1/auth/me", "", 401}, {"GET", "/api/v1/users", "", 401},
		{"GET", "/api/v1/users?page_size=101", "", 422}, {"GET", "/api/v1/auth/login", "", 405},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(test.method, test.path, strings.NewReader(test.body)))
		if response.Code != test.code {
			t.Fatalf("%s %s: got %d want %d", test.method, test.path, response.Code, test.code)
		}
		if test.code == 401 && response.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Fatal("missing bearer challenge")
		}
		if test.code == 422 {
			var value struct {
				Detail []dtos.Issue `json:"detail"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil || len(value.Detail) != 1 || len(value.Detail[0].Loc) == 0 {
				t.Fatal("validation response shape changed")
			}
		}
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(strings.Repeat("x", 16385))))
	if response.Code != 413 {
		t.Fatal("auth-specific JSON ceiling not enforced")
	}
}
func TestSafeDTOsNeverExposeCredentialHashes(t *testing.T) {
	raw, _ := json.Marshal(dtos.User(domain.User{PasswordHash: "private-hash", Roles: []domain.Role{{Name: "reader"}}}))
	if strings.Contains(string(raw), "private-hash") || strings.Contains(string(raw), `"password":`) {
		t.Fatal("credential field escaped")
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if len(value) != 12 {
		t.Fatal("user response fields drifted")
	}
}
func TestBooleanAndCollectionCoercion(t *testing.T) {
	for _, value := range []string{`false`, `"false"`, `0`, `"off"`} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(`{"is_active":`+value+`}`))
		var payload struct {
			Active *bool `json:"is_active"`
		}
		_, ok := decode(response, request, &payload, "is_active")
		if !ok || payload.Active == nil || *payload.Active {
			t.Fatal("nullable bool compatibility failed")
		}
	}
}

func TestJSONMediaTypeContract(t *testing.T) {
	for _, contentType := range []string{"", "application/json; charset=utf-8", "application/problem+json", "text/plain", "application/x-www-form-urlencoded"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"identifier":"abc"}`))
		request.Header.Set("Content-Type", contentType)
		var payload struct {
			Identifier *string `json:"identifier"`
		}
		_, ok := decode(response, request, &payload, "identifier")
		want := contentType == "" || strings.HasPrefix(contentType, "application/json") || contentType == "application/problem+json"
		if ok != want || (!ok && response.Code != 422) {
			t.Fatal("JSON media type compatibility failed")
		}
	}
}
