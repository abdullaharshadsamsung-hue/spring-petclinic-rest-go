package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeUsers struct {
	users map[string]UserDetails
	err   error
	calls int
}

func (f *fakeUsers) LoadUserByUsername(ctx context.Context, username string) (UserDetails, bool, error) {
	f.calls++
	if f.err != nil {
		return UserDetails{}, false, f.err
	}
	u, ok := f.users[username]
	if !ok || len(u.Authorities) == 0 {
		return UserDetails{}, false, nil
	}
	return u, true, nil
}

// plainEncoder avoids bcrypt cost in middleware tests.
type plainEncoder struct{}

func (plainEncoder) Encode(raw string) (string, error) { return "enc:" + raw, nil }
func (plainEncoder) Matches(raw, encoded string) bool  { return encoded != "" && encoded == "enc:"+raw }

func TestIsSecurityEnabled(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"true", true}, {"TRUE", true}, {" True ", true},
		{"false", false}, {"", false}, {"yes", false},
	}
	for _, tt := range tests {
		if got := IsSecurityEnabled(tt.in); got != tt.want {
			t.Errorf("IsSecurityEnabled(%q)=%v want %v", tt.in, got, tt.want)
		}
	}
}

func TestBCryptPasswordEncoderEmptyHash(t *testing.T) {
	e := NewPasswordEncoder()
	if e.Matches("secret", "") {
		t.Error("empty hash should not match")
	}
}

func TestQueries(t *testing.T) {
	if UsersByUsernameQuery != "select username,password,enabled from users where username=$1" {
		t.Error("users query")
	}
	if AuthoritiesByUsernameQuery != "select username,role from roles where username=$1" {
		t.Error("roles query")
	}
}

func TestBasicAuthenticationFilterChain(t *testing.T) {
	users := map[string]UserDetails{
		"admin":    {Username: "admin", Password: "enc:admin", Enabled: true, Authorities: []string{RoleOwnerAdmin, RoleVetAdmin}},
		"disabled": {Username: "disabled", Password: "enc:pw", Enabled: false, Authorities: []string{RoleAdmin}},
		"noroles":  {Username: "noroles", Password: "enc:pw", Enabled: true},
	}
	tests := []struct {
		name     string
		method   string
		setAuth  bool
		user, pw string
		err      error
		want     int
	}{
		{"no credentials", http.MethodGet, false, "", "", nil, 401},
		{"valid", http.MethodGet, true, "admin", "admin", nil, 200},
		{"valid POST no csrf", http.MethodPost, true, "admin", "admin", nil, 200},
		{"valid PUT no csrf", http.MethodPut, true, "admin", "admin", nil, 200},
		{"valid DELETE no csrf", http.MethodDelete, true, "admin", "admin", nil, 200},
		{"wrong password", http.MethodGet, true, "admin", "bad", nil, 401},
		{"unknown user", http.MethodGet, true, "ghost", "x", nil, 401},
		{"disabled user", http.MethodGet, true, "disabled", "pw", nil, 401},
		{"no roles", http.MethodGet, true, "noroles", "pw", nil, 401},
		{"lookup error", http.MethodGet, true, "admin", "admin", errors.New("db down"), 401},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fu := &fakeUsers{users: users, err: tt.err}
			called := false
			var principal UserDetails
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				principal, _ = PrincipalFromContext(r.Context())
				w.WriteHeader(200)
			})
			h := BasicAuthenticationFilterChain(fu, plainEncoder{})(next)
			req := httptest.NewRequest(tt.method, "/api/owners", nil)
			if tt.setAuth {
				req.SetBasicAuth(tt.user, tt.pw)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status %d want %d", rec.Code, tt.want)
			}
			if rec.Header().Get("X-Frame-Options") != "DENY" {
				t.Error("missing X-Frame-Options DENY")
			}
			if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Error("missing nosniff")
			}
			if tt.want == 401 {
				if called {
					t.Error("handler should not be called")
				}
				if got := rec.Header().Get("WWW-Authenticate"); got != `Basic realm="Realm"` {
					t.Errorf("WWW-Authenticate=%q", got)
				}
				if rec.Body.Len() != 0 {
					t.Error("expected empty body")
				}
				if tt.setAuth && fu.calls == 0 {
					t.Error("expected user lookup")
				}
				return
			}
			if !called {
				t.Fatal("handler not called")
			}
			if principal.Username != tt.user || principal.Password != "" {
				t.Errorf("principal %+v", principal)
			}
			if len(principal.Authorities) != 2 || principal.Authorities[0] != RoleOwnerAdmin {
				t.Errorf("authorities %v", principal.Authorities)
			}
		})
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()
	if _, ok := PrincipalFromContext(ctx); ok {
		t.Error("unexpected principal")
	}
	if AuthoritiesFromContext(ctx) != nil {
		t.Error("expected nil")
	}
	ctx = WithPrincipal(ctx, UserDetails{Username: "u", Authorities: []string{RoleAdmin}})
	if a := AuthoritiesFromContext(ctx); len(a) != 1 || a[0] != RoleAdmin {
		t.Errorf("got %v", a)
	}
}

func TestRequire(t *testing.T) {
	tests := []struct {
		name      string
		principal *UserDetails
		roles     []string
		want      int
	}{
		{"no principal skips", nil, []string{RoleAdmin}, 200},
		{"has role", &UserDetails{Authorities: []string{RoleOwnerAdmin}}, []string{RoleOwnerAdmin}, 200},
		{"unprefixed role", &UserDetails{Authorities: []string{RoleOwnerAdmin}}, []string{"OWNER_ADMIN"}, 200},
		{"one of many", &UserDetails{Authorities: []string{RoleVetAdmin}}, []string{RoleAdmin, RoleVetAdmin}, 200},
		{"missing role", &UserDetails{Authorities: []string{RoleVetAdmin}}, []string{RoleOwnerAdmin}, 403},
		{"no authorities", &UserDetails{}, []string{RoleAdmin}, 403},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := Require(tt.roles...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
			}))
			req := httptest.NewRequest(http.MethodGet, "/api/owners", nil)
			if tt.principal != nil {
				req = req.WithContext(WithPrincipal(req.Context(), *tt.principal))
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status %d want %d", rec.Code, tt.want)
			}
			if tt.want == 403 {
				if rec.Header().Get("Content-Type") != "application/problem+json" {
					t.Error("content type")
				}
				b := rec.Body.String()
				if !strings.Contains(b, `"status":403`) || !strings.Contains(b, `"instance":"/api/owners"`) {
					t.Errorf("body %s", b)
				}
			}
		})
	}
}

func TestFilterChainWithRequire403(t *testing.T) {
	fu := &fakeUsers{users: map[string]UserDetails{
		"vet": {Username: "vet", Password: "enc:pw", Enabled: true, Authorities: []string{RoleVetAdmin}},
	}}
	inner := Require(RoleOwnerAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	h := BasicAuthenticationFilterChain(fu, plainEncoder{})(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/owners", nil)
	req.SetBasicAuth("vet", "pw")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("status %d", rec.Code)
	}
}