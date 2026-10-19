package model

import (
	"encoding/json"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func TestUsersTableConstant(t *testing.T) {
	if UsersTable != "users" {
		t.Fatalf("UsersTable = %q, want users", UsersTable)
	}
}

func TestNewUser(t *testing.T) {
	u := NewUser("admin", "{noop}admin")
	if u.GetUsername() != "admin" || u.GetPassword() != "{noop}admin" {
		t.Fatalf("unexpected user %+v", u)
	}
	if u.GetEnabled() != nil {
		t.Fatal("Enabled should be nil")
	}
	if u.GetRoles() != nil {
		t.Fatal("Roles should be nil")
	}
}

func TestZeroValueUser(t *testing.T) {
	var u User
	if u.GetUsername() != "" || u.GetPassword() != "" || u.GetEnabled() != nil || u.GetRoles() != nil {
		t.Fatalf("zero user not empty: %+v", u)
	}
	if u.IsEnabled() {
		t.Fatal("unset enabled must be disabled")
	}
}

func TestNilUserGetters(t *testing.T) {
	var u *User
	if u.GetUsername() != "" || u.GetPassword() != "" || u.GetEnabled() != nil || u.GetRoles() != nil || u.IsEnabled() {
		t.Fatal("nil user getters should return zero values")
	}
}

func TestUsername(t *testing.T) {
	tests := []struct{ name, in string }{
		{"admin", "admin"}, {"empty", ""}, {"spaces", "  x  "}, {"unicode", "üsér"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := &User{}
			u.SetUsername(tc.in)
			if got := u.GetUsername(); got != tc.in {
				t.Fatalf("got %q want %q", got, tc.in)
			}
		})
	}
}

func TestPassword(t *testing.T) {
	tests := []struct{ name, in string }{
		{"noop", "{noop}secret"},
		{"bcrypt", "$2a$10$abcdefghijklmnopqrstuv"},
		{"empty", ""},
		{"plain", "secret"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := &User{}
			u.SetPassword(tc.in)
			if got := u.GetPassword(); got != tc.in {
				t.Fatalf("got %q want %q", got, tc.in)
			}
		})
	}
}

func TestEnabled(t *testing.T) {
	tests := []struct {
		name    string
		in      *bool
		wantNil bool
		want    bool
	}{
		{"true", boolPtr(true), false, true},
		{"false", boolPtr(false), false, false},
		{"nil", nil, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := &User{Enabled: boolPtr(true)}
			u.SetEnabled(tc.in)
			got := u.GetEnabled()
			if tc.wantNil {
				if got != nil {
					t.Fatal("want nil")
				}
			} else {
				if got != tc.in || *got != tc.want {
					t.Fatalf("got %v", got)
				}
			}
			if u.IsEnabled() != tc.want {
				t.Fatalf("IsEnabled = %v want %v", u.IsEnabled(), tc.want)
			}
		})
	}
}

func TestRolesSetGet(t *testing.T) {
	r1 := NewRole("ROLE_ADMIN", nil)
	r2 := NewRole("ROLE_USER", nil)
	tests := []struct {
		name  string
		roles []*Role
	}{
		{"nil", nil},
		{"empty", []*Role{}},
		{"two", []*Role{r1, r2}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := &User{}
			u.SetRoles(tc.roles)
			got := u.GetRoles()
			if (got == nil) != (tc.roles == nil) || len(got) != len(tc.roles) {
				t.Fatalf("got %v", got)
			}
			for i := range got {
				if got[i] != tc.roles[i] {
					t.Fatal("role pointer mismatch")
				}
			}
			if len(tc.roles) > 0 && &got[0] != &tc.roles[0] {
				t.Fatal("expected same backing slice, not a copy")
			}
		})
	}
}

func TestSetRolesDoesNotChangeBackReference(t *testing.T) {
	r := NewRole("ROLE_ADMIN", nil)
	u := NewUser("admin", "pw")
	u.SetRoles([]*Role{r})
	if r.GetUser() != nil {
		t.Fatal("SetRoles must not set back-reference")
	}
}

func TestAddRole(t *testing.T) {
	existing := NewRole("ROLE_OLD", nil)
	tests := []struct {
		name      string
		initial   []*Role
		add       []string
		wantNames []string
	}{
		{"nil roles initialized", nil, []string{"ROLE_ADMIN"}, []string{"ROLE_ADMIN"}},
		{"appends to existing", []*Role{existing}, []string{"ROLE_NEW"}, []string{"ROLE_OLD", "ROLE_NEW"}},
		{"same name twice gives two roles", nil, []string{"ROLE_A", "ROLE_A"}, []string{"ROLE_A", "ROLE_A"}},
		{"empty name", nil, []string{""}, []string{""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := NewUser("admin", "pw")
			u.Roles = tc.initial
			for _, n := range tc.add {
				u.AddRole(n)
			}
			got := u.GetRoles()
			if len(got) != len(tc.wantNames) {
				t.Fatalf("len %d want %d", len(got), len(tc.wantNames))
			}
			for i, r := range got {
				if r == nil || r.GetName() != tc.wantNames[i] {
					t.Fatalf("role %d = %+v", i, r)
				}
				if r.GetUser() != nil {
					t.Fatal("AddRole must not set back-reference")
				}
				if r.Username != "" || !r.IsNew() {
					t.Fatal("new role should be unsaved without username")
				}
			}
			if tc.initial != nil && got[0] != existing {
				t.Fatal("existing role lost")
			}
			if len(got) == 2 && got[0] == got[1] {
				t.Fatal("roles must be distinct objects")
			}
		})
	}
}

func TestUserSatisfiesUserRef(t *testing.T) {
	var ref UserRef = NewUser("alice", "pw")
	r := NewRole("ROLE_USER", ref)
	if r.PersistedUsername() != "alice" {
		t.Fatalf("PersistedUsername = %q", r.PersistedUsername())
	}
}

// roleStore is a mock repository verifying cascade semantics via interface.
type roleStore interface {
	SaveRole(username string, r *Role)
}

type mockRoleStore struct{ saved map[string][]string }

func (m *mockRoleStore) SaveRole(username string, r *Role) {
	m.saved[username] = append(m.saved[username], r.GetName())
}

func cascadeSave(s roleStore, u *User) {
	for _, r := range u.GetRoles() {
		s.SaveRole(u.GetUsername(), r)
	}
}

func TestCascadeSaveWithMock(t *testing.T) {
	u := NewUser("admin", "pw")
	u.AddRole("ROLE_ADMIN")
	u.AddRole("ROLE_USER")
	m := &mockRoleStore{saved: map[string][]string{}}
	cascadeSave(m, u)
	if got := m.saved["admin"]; len(got) != 2 || got[0] != "ROLE_ADMIN" || got[1] != "ROLE_USER" {
		t.Fatalf("saved %v", got)
	}
	// Without back-reference, PersistedUsername is empty; repository must supply owner username.
	if u.Roles[0].PersistedUsername() != "" {
		t.Fatal("expected empty persisted username without back-reference")
	}
}

func TestUserJSON(t *testing.T) {
	u := NewUser("admin", "{noop}pw")
	u.SetEnabled(boolPtr(true))
	u.AddRole("ROLE_ADMIN")
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["username"] != "admin" || m["password"] != "{noop}pw" || m["enabled"] != true {
		t.Fatalf("json %s", b)
	}
	roles, ok := m["roles"].([]any)
	if !ok || len(roles) != 1 {
		t.Fatalf("roles %v", m["roles"])
	}

	b, _ = json.Marshal(&User{})
	var m2 map[string]any
	_ = json.Unmarshal(b, &m2)
	if m2["enabled"] != nil || m2["roles"] != nil {
		t.Fatalf("unset fields should be null: %s", b)
	}
}