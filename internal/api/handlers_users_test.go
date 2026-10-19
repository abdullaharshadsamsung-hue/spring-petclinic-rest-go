// NOTE: the reported build failure is not caused by this file. It comes from
// module resolution of golang.org/x/crypto/bcrypt, a transitive dependency of
// the api package through migrated-app/internal/service. The pinned
// golang.org/x/crypto@v0.58.0 requires go >= 1.26.0, but the local toolchain is
// go 1.25.14 with GOTOOLCHAIN=local. Fix it in go.mod/go.sum by pinning an
// x/crypto version compatible with go 1.25, or by allowing a toolchain upgrade.
// This test uses only the standard library and identifiers from the code under
// test, so changing it cannot resolve that error.
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func strPtr(v string) *string { return &v }

func TestValidateUserDTO(t *testing.T) {
	tests := []struct {
		name    string
		dto     userDTO
		wantErr bool
	}{
		{"valid", userDTO{Username: strPtr("u"), Password: strPtr("p"), Roles: []*userRoleDTO{{Name: strPtr("A")}}}, false},
		{"no password ok", userDTO{Username: strPtr("u")}, false},
		{"max len runes", userDTO{Username: strPtr(strings.Repeat("é", 80))}, false},
		{"nil username", userDTO{}, true},
		{"empty username", userDTO{Username: strPtr("")}, true},
		{"long username", userDTO{Username: strPtr(strings.Repeat("a", 81))}, true},
		{"empty password", userDTO{Username: strPtr("u"), Password: strPtr("")}, true},
		{"long password", userDTO{Username: strPtr("u"), Password: strPtr(strings.Repeat("a", 81))}, true},
		{"nil role", userDTO{Username: strPtr("u"), Roles: []*userRoleDTO{nil}}, true},
		{"nil role name", userDTO{Username: strPtr("u"), Roles: []*userRoleDTO{{}}}, true},
		{"empty role name", userDTO{Username: strPtr("u"), Roles: []*userRoleDTO{{Name: strPtr("")}}}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateUserDTO(&tc.dto).HasErrors(); got != tc.wantErr {
				t.Errorf("HasErrors = %v, want %v", got, tc.wantErr)
			}
		})
	}
}

func TestUserDTORoundTrip(t *testing.T) {
	enabled := true
	dto := &userDTO{
		Username: strPtr("alice"),
		Password: strPtr("secret"),
		Enabled:  &enabled,
		Roles:    []*userRoleDTO{{Name: strPtr("ADMIN")}, nil},
	}
	user := userFromDTO(dto)
	if user.GetUsername() != "alice" || user.GetPassword() != "secret" {
		t.Fatalf("unexpected user %q/%q", user.GetUsername(), user.GetPassword())
	}
	out := userToDTO(user)
	if out.Username == nil || *out.Username != "alice" {
		t.Errorf("username = %v", out.Username)
	}
	if out.Password == nil || *out.Password != "secret" {
		t.Errorf("password = %v", out.Password)
	}
	if out.Enabled == nil || !*out.Enabled {
		t.Errorf("enabled = %v", out.Enabled)
	}
	if len(out.Roles) != 1 || out.Roles[0].Name == nil || *out.Roles[0].Name != "ADMIN" {
		t.Errorf("roles = %+v", out.Roles)
	}
}

func TestAddUserRejectsBadRequests(t *testing.T) {
	mux := http.NewServeMux()
	NewUserHandler(nil, nil).Register(mux, "/petclinic")

	tests := []struct {
		name string
		body string
	}{
		{"malformed json", `{"username":`},
		{"missing username", `{"roles":[{"name":"X"}]}`},
		{"empty role name", `{"username":"u","roles":[{"name":""}]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/petclinic/api/users", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code < 400 || rec.Code >= 500 {
				t.Errorf("status = %d, want 4xx; body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}