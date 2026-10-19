package auth

import (
	"strings"
	"testing"
)

func TestRoleConstantValues(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"OWNER_ADMIN", RoleOwnerAdmin, "ROLE_OWNER_ADMIN"},
		{"VET_ADMIN", RoleVetAdmin, "ROLE_VET_ADMIN"},
		{"ADMIN", RoleAdmin, "ROLE_ADMIN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
			if !strings.HasPrefix(tt.got, "ROLE_") {
				t.Errorf("%q lacks ROLE_ prefix", tt.got)
			}
			// Reading twice yields the same value (no runtime state).
			if NormalizeRole(tt.got) != tt.got {
				t.Errorf("NormalizeRole changed already-prefixed constant %q", tt.got)
			}
		})
	}
}

func TestRoleConstantsDistinct(t *testing.T) {
	roles := []string{RoleOwnerAdmin, RoleVetAdmin, RoleAdmin}
	seen := map[string]bool{}
	for _, r := range roles {
		if seen[r] {
			t.Errorf("duplicate role value %q", r)
		}
		seen[r] = true
	}
}

func TestNormalizeRole(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"already prefixed", "ROLE_ADMIN", "ROLE_ADMIN"},
		{"missing prefix", "ADMIN", "ROLE_ADMIN"},
		{"missing prefix owner", "OWNER_ADMIN", "ROLE_OWNER_ADMIN"},
		{"empty", "", "ROLE_"},
		{"prefix only", "ROLE_", "ROLE_"},
		{"lowercase prefix not recognised", "role_ADMIN", "ROLE_role_ADMIN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeRole(tt.in); got != tt.want {
				t.Errorf("NormalizeRole(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestHasRole(t *testing.T) {
	tests := []struct {
		name        string
		authorities []string
		role        string
		want        bool
	}{
		{"owner admin granted", []string{"ROLE_OWNER_ADMIN"}, RoleOwnerAdmin, true},
		{"owner admin denied for vet admin", []string{"ROLE_VET_ADMIN"}, RoleOwnerAdmin, false},
		{"vet admin granted", []string{"ROLE_VET_ADMIN"}, RoleVetAdmin, true},
		{"vet admin denied", []string{"ROLE_OWNER_ADMIN", "ROLE_ADMIN"}, RoleVetAdmin, false},
		{"admin granted", []string{"ROLE_VET_ADMIN", "ROLE_ADMIN"}, RoleAdmin, true},
		{"admin denied", []string{"ROLE_OWNER_ADMIN"}, RoleAdmin, false},
		{"nil authorities", nil, RoleAdmin, false},
		{"empty authorities", []string{}, RoleOwnerAdmin, false},
		{"unprefixed role argument normalised", []string{"ROLE_ADMIN"}, "ADMIN", true},
		{"unprefixed authority not matched", []string{"ADMIN"}, RoleAdmin, false},
		{"case sensitive", []string{"role_admin"}, RoleAdmin, false},
		{"partial match not accepted", []string{"ROLE_ADMIN"}, RoleOwnerAdmin, false},
		{"multiple roles", []string{RoleOwnerAdmin, RoleVetAdmin, RoleAdmin}, RoleVetAdmin, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasRole(tt.authorities, tt.role); got != tt.want {
				t.Errorf("HasRole(%v, %q) = %v, want %v", tt.authorities, tt.role, got, tt.want)
			}
		})
	}
}

func TestHasRoleNoSideEffects(t *testing.T) {
	auths := []string{"ROLE_VET_ADMIN", "ADMIN"}
	orig := append([]string(nil), auths...)
	_ = HasRole(auths, RoleAdmin)
	_ = HasRole(auths, "VET_ADMIN")
	for i := range auths {
		if auths[i] != orig[i] {
			t.Errorf("authorities mutated at %d: %q != %q", i, auths[i], orig[i])
		}
	}
}