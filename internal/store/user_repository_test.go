package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"migrated-app/internal/model"
)

// fakeUserRepo is an in-memory transactional store used to exercise the
// UserRepository contract.
type roleRow struct {
	username string
	role     string
}

type userRow struct {
	password string
	enabled  *bool
}

type fakeUserRepo struct {
	users     map[string]userRow
	roles     []roleRow
	failWith  error
	saveCalls int
}

var _ UserRepository = (*fakeUserRepo)(nil)

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[string]userRow{}}
}

func (f *fakeUserRepo) Save(ctx context.Context, user *model.User) error {
	f.saveCalls++
	if user == nil {
		return errors.New("store: nil user")
	}
	if user.Username == "" {
		return errors.New("store: empty username")
	}
	if f.failWith != nil {
		return fmt.Errorf("store: save user %q: %w", user.Username, f.failWith)
	}
	users := map[string]userRow{}
	for k, v := range f.users {
		users[k] = v
	}
	roles := append([]roleRow(nil), f.roles...)
	users[user.Username] = userRow{password: user.Password, enabled: user.Enabled}
	for _, r := range user.Roles {
		if r == nil {
			return IntegrityViolation("nil role", nil)
		}
		name := r.Name
		if name == "" {
			return IntegrityViolation("role must not be null", nil)
		}
		for _, existing := range roles {
			if existing.username == user.Username && existing.role == name {
				return IntegrityViolation("duplicate (username, role)", errors.New("unique violation"))
			}
		}
		roles = append(roles, roleRow{username: user.Username, role: name})
	}
	f.users, f.roles = users, roles
	return nil
}

func boolPtr(b bool) *bool { return &b }

func (f *fakeUserRepo) rolesOf(u string) []string {
	var out []string
	for _, r := range f.roles {
		if r.username == u {
			out = append(out, r.role)
		}
	}
	return out
}

func userWithRoles(name, pw string, enabled *bool, roles ...string) *model.User {
	u := &model.User{Username: name, Password: pw, Enabled: enabled}
	for _, r := range roles {
		u.Roles = append(u.Roles, &model.Role{Name: r})
	}
	return u
}

func TestUserRepositorySave(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		seed          func(*fakeUserRepo)
		user          *model.User
		wantErr       bool
		wantIntegrity bool
		check         func(t *testing.T, f *fakeUserRepo)
	}{
		{
			name: "insert new user with roles",
			user: userWithRoles("admin", "pw", boolPtr(true), "ROLE_ADMIN", "ROLE_VET_ADMIN"),
			check: func(t *testing.T, f *fakeUserRepo) {
				row, ok := f.users["admin"]
				if !ok || row.password != "pw" || row.enabled == nil || !*row.enabled {
					t.Fatalf("unexpected user row %+v ok=%v", row, ok)
				}
				if got := f.rolesOf("admin"); len(got) != 2 {
					t.Fatalf("roles = %v", got)
				}
			},
		},
		{
			name: "existing user is merged",
			seed: func(f *fakeUserRepo) {
				f.users["bob"] = userRow{password: "old", enabled: boolPtr(false)}
			},
			user: userWithRoles("bob", "new", boolPtr(true), "ROLE_OWNER_ADMIN"),
			check: func(t *testing.T, f *fakeUserRepo) {
				if len(f.users) != 1 {
					t.Fatalf("expected one user, got %d", len(f.users))
				}
				row := f.users["bob"]
				if row.password != "new" || !*row.enabled {
					t.Fatalf("not updated: %+v", row)
				}
				if got := f.rolesOf("bob"); len(got) != 1 || got[0] != "ROLE_OWNER_ADMIN" {
					t.Fatalf("roles = %v", got)
				}
			},
		},
		{
			name: "empty roles persists user only",
			user: userWithRoles("solo", "pw", nil),
			check: func(t *testing.T, f *fakeUserRepo) {
				if _, ok := f.users["solo"]; !ok {
					t.Fatal("user not saved")
				}
				if len(f.roles) != 0 {
					t.Fatalf("unexpected roles %v", f.roles)
				}
			},
		},
		{
			name:    "nil user rejected",
			user:    nil,
			wantErr: true,
		},
		{
			name:    "empty username rejected",
			user:    userWithRoles("", "pw", nil, "ROLE_ADMIN"),
			wantErr: true,
		},
		{
			name:          "duplicate role is integrity error and atomic",
			user:          userWithRoles("dup", "pw", nil, "ROLE_ADMIN", "ROLE_ADMIN"),
			wantErr:       true,
			wantIntegrity: true,
		},
		{
			name:          "null role column is integrity error",
			user:          userWithRoles("nr", "pw", nil, ""),
			wantErr:       true,
			wantIntegrity: true,
		},
		{
			name:    "driver failure wrapped",
			seed:    func(f *fakeUserRepo) { f.failWith = errors.New("connection refused") },
			user:    userWithRoles("x", "pw", nil),
			wantErr: true,
			check: func(t *testing.T, f *fakeUserRepo) {
				if len(f.users) != 0 {
					t.Fatal("data written on failure")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeUserRepo()
			if tc.seed != nil {
				tc.seed(f)
			}
			before := len(f.users)
			var repo UserRepository = f
			err := repo.Save(ctx, tc.user)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantIntegrity && !errors.Is(err, ErrIntegrity) {
				t.Fatalf("expected ErrIntegrity, got %v", err)
			}
			if tc.wantErr {
				if len(f.users) != before || (tc.seed == nil && len(f.roles) != 0) {
					t.Fatalf("partial write: users=%v roles=%v", f.users, f.roles)
				}
			}
			if tc.check != nil {
				tc.check(t, f)
			}
		})
	}
}

func TestUserRepositorySaveDoesNotMutateUser(t *testing.T) {
	f := newFakeUserRepo()
	u := userWithRoles("owner", "pw", nil, "ROLE_ADMIN")
	if err := f.Save(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	for _, r := range f.roles {
		if r.username != "owner" {
			t.Fatalf("role linked to %q", r.username)
		}
	}
	if u.Username != "owner" || u.Password != "pw" {
		t.Fatal("save mutated user")
	}
}

func TestIntegrityViolationWrapsErrIntegrity(t *testing.T) {
	tests := []struct {
		name  string
		cause error
	}{
		{"no cause", nil},
		{"with cause", errors.New("pq: duplicate key")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := IntegrityViolation("dup role", tc.cause)
			if !errors.Is(err, ErrIntegrity) {
				t.Fatalf("not ErrIntegrity: %v", err)
			}
			if !strings.Contains(err.Error(), "dup role") {
				t.Fatalf("missing detail: %v", err)
			}
		})
	}
}