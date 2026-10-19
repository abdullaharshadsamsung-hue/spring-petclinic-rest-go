package api

import (
	"testing"

	"migrated-app/internal/model"
)

func mappingBoolPtr(b bool) *bool { return &b }

func TestRoleFromDTO(t *testing.T) {
	if got := RoleFromDTO(nil); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
	r := RoleFromDTO(&RoleDTO{Name: "ROLE_ADMIN"})
	if r == nil {
		t.Fatal("expected role")
	}
	if r.GetName() != "ROLE_ADMIN" {
		t.Errorf("name = %q", r.GetName())
	}
}

func TestRoleToDTO(t *testing.T) {
	if got := RoleToDTO(nil); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
	d := RoleToDTO(model.NewRole("OWNER_ADMIN", nil))
	if d == nil || d.Name != "OWNER_ADMIN" {
		t.Fatalf("dto = %+v", d)
	}
}

func TestRolesToDTOs(t *testing.T) {
	if got := RolesToDTOs(nil); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
	in := []*model.Role{model.NewRole("A", nil), nil, model.NewRole("B", nil)}
	got := RolesToDTOs(in)
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0] == nil || got[0].Name != "A" || got[1] != nil || got[2] == nil || got[2].Name != "B" {
		t.Errorf("got %+v", got)
	}
	if empty := RolesToDTOs([]*model.Role{}); empty == nil || len(empty) != 0 {
		t.Errorf("empty = %+v", empty)
	}
}

func TestRolesFromDTOs(t *testing.T) {
	if got := RolesFromDTOs(nil); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
	got := RolesFromDTOs([]*RoleDTO{{Name: "A"}, nil})
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0] == nil || got[0].GetName() != "A" || got[1] != nil {
		t.Errorf("got %+v", got)
	}
}

func TestUserFromDTO(t *testing.T) {
	if got := UserFromDTO(nil); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
	enabled := mappingBoolPtr(true)
	dto := &UserDTO{Username: "alice", Password: "secret", Enabled: enabled, Roles: []*RoleDTO{{Name: "ADMIN"}}}
	u := UserFromDTO(dto)
	if u == nil {
		t.Fatal("expected user")
	}
	if u.GetUsername() != "alice" || u.GetPassword() != "secret" {
		t.Errorf("user = %q/%q", u.GetUsername(), u.GetPassword())
	}
	if e := u.GetEnabled(); e == nil || !*e {
		t.Errorf("enabled = %v", e)
	} else if e == enabled {
		t.Error("enabled pointer shared with DTO")
	}
	roles := u.GetRoles()
	if len(roles) != 1 || roles[0] == nil || roles[0].GetName() != "ADMIN" {
		t.Errorf("roles = %+v", roles)
	}
}

func TestUserToDTO(t *testing.T) {
	if got := UserToDTO(nil); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
	u := model.NewUser("bob", "pw")
	u.SetEnabled(mappingBoolPtr(false))
	u.SetRoles([]*model.Role{model.NewRole("OWNER_ADMIN", nil)})
	d := UserToDTO(u)
	if d == nil {
		t.Fatal("expected dto")
	}
	if d.Username != "bob" || d.Password != "pw" {
		t.Errorf("dto = %+v", d)
	}
	if d.Enabled == nil || *d.Enabled {
		t.Errorf("enabled = %v", d.Enabled)
	}
	if len(d.Roles) != 1 || d.Roles[0] == nil || d.Roles[0].Name != "OWNER_ADMIN" {
		t.Errorf("roles = %+v", d.Roles)
	}
}

func TestUserRoundTrip(t *testing.T) {
	in := &UserDTO{Username: "carol", Password: "x", Enabled: mappingBoolPtr(true), Roles: []*RoleDTO{{Name: "A"}, {Name: "B"}}}
	out := UserToDTO(UserFromDTO(in))
	if out == nil || out.Username != in.Username || out.Password != in.Password {
		t.Fatalf("round trip = %+v", out)
	}
	if out.Enabled == nil || !*out.Enabled {
		t.Errorf("enabled = %v", out.Enabled)
	}
	if len(out.Roles) != 2 || out.Roles[0].Name != "A" || out.Roles[1].Name != "B" {
		t.Errorf("roles = %+v", out.Roles)
	}
}

func TestCopyBoolPtr(t *testing.T) {
	if copyBoolPtr(nil) != nil {
		t.Error("nil must stay nil")
	}
	b := mappingBoolPtr(true)
	c := copyBoolPtr(b)
	if c == nil || *c != true || c == b {
		t.Errorf("copy = %v", c)
	}
}