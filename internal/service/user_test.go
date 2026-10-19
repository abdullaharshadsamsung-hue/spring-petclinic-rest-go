package service

import (
	"context"
	"errors"
	"testing"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

// fakeUserRepo embeds the interface so any methods not overridden panic if called.
type fakeUserRepo struct {
	store.UserRepository
	err     error
	calls   int
	saved   *model.User
	ctxSeen context.Context
}

func (f *fakeUserRepo) Save(ctx context.Context, user *model.User) error {
	f.calls++
	f.saved = user
	f.ctxSeen = ctx
	return f.err
}

type ctxKey struct{}

func newRole(name string, owner *model.User) *model.Role {
	r := &model.Role{}
	r.SetName(name)
	if owner != nil {
		r.SetUser(owner)
	}
	return r
}

func newUserWithRoles(username string, roleNames ...string) *model.User {
	u := &model.User{Username: username}
	roles := make([]*model.Role, 0, len(roleNames))
	for _, n := range roleNames {
		roles = append(roles, newRole(n, nil))
	}
	u.Roles = roles
	return u
}

func TestSaveUser_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		user    *model.User
		wantIs  error
		wantErr bool
	}{
		{name: "nil user", user: nil, wantIs: ErrNilUser, wantErr: true},
		{name: "nil roles", user: model.NewUser("bob", "pw"), wantIs: ErrUserHasNoRoles, wantErr: true},
		{name: "empty roles", user: &model.User{Username: "bob", Roles: []*model.Role{}}, wantIs: ErrUserHasNoRoles, wantErr: true},
		{name: "nil role entry", user: &model.User{Username: "bob", Roles: []*model.Role{nil}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeUserRepo{}
			svc := NewUserService(repo)
			err := svc.SaveUser(context.Background(), tt.user)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("expected %v, got %v", tt.wantIs, err)
			}
			if repo.calls != 0 {
				t.Fatalf("repository must not be called, got %d calls", repo.calls)
			}
		})
	}
}

func TestUserServiceSave_RolePrefixAndBackReference(t *testing.T) {
	tests := []struct {
		name      string
		roleNames []string
		want      []string
	}{
		{"single unprefixed", []string{"admin"}, []string{"ROLE_admin"}},
		{"already prefixed", []string{"ROLE_admin"}, []string{"ROLE_admin"}},
		{"mixed", []string{"OWNER_ADMIN", "ROLE_VET_ADMIN", ""}, []string{"ROLE_OWNER_ADMIN", "ROLE_VET_ADMIN", "ROLE_"}},
		{"lowercase role prefix not recognized", []string{"role_x"}, []string{"ROLE_role_x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newUserWithRoles("alice", tt.roleNames...)
			repo := &fakeUserRepo{}
			ctx := context.WithValue(context.Background(), ctxKey{}, "v")
			if err := NewUserService(repo).SaveUser(ctx, u); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.calls != 1 {
				t.Fatalf("expected 1 save call, got %d", repo.calls)
			}
			if repo.saved != u {
				t.Fatal("repository received a different user")
			}
			if repo.ctxSeen != ctx {
				t.Fatal("context not propagated to repository")
			}
			roles := repo.saved.GetRoles()
			if len(roles) != len(tt.want) {
				t.Fatalf("role count = %d, want %d", len(roles), len(tt.want))
			}
			for i, r := range roles {
				if r.GetName() != tt.want[i] {
					t.Errorf("role[%d] name = %q, want %q", i, r.GetName(), tt.want[i])
				}
				if r.GetUser() == nil {
					t.Fatalf("role[%d] has no owning user", i)
				}
				if any(r.GetUser()) != any(u) {
					t.Errorf("role[%d] does not reference saved user", i)
				}
			}
		})
	}
}