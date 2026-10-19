// Package auth holds the PetClinic security role names and small helpers for
// checking them.
package auth

import "strings"

// Authority names for the PetClinic roles. They keep Spring Security's "ROLE_"
// prefix, so they match the values stored in the roles.role column and the
// authorities created for an authenticated user.
//
// MIGRATION_NOTE: The Java source was a @Component bean ("roles") with public
// final fields so that SpEL expressions such as
// @PreAuthorize("hasRole(@roles.OWNER_ADMIN)") could read them. Go has no SpEL
// or method security. Handlers and middleware use these constants directly,
// e.g. RequireRole(auth.RoleOwnerAdmin).
const (
	// RoleOwnerAdmin guards owner and pet management operations.
	RoleOwnerAdmin = "ROLE_OWNER_ADMIN"
	// RoleVetAdmin guards vet and specialty management operations.
	RoleVetAdmin = "ROLE_VET_ADMIN"
	// RoleAdmin guards user and role management operations.
	RoleAdmin = "ROLE_ADMIN"
)

// rolePrefix is the prefix Spring Security's hasRole adds to role names.
const rolePrefix = "ROLE_"

// NormalizeRole returns role with the "ROLE_" prefix, adding it only if it is
// missing. This matches what Spring Security's hasRole does with its argument.
func NormalizeRole(role string) string {
	if strings.HasPrefix(role, rolePrefix) {
		return role
	}
	return rolePrefix + role
}

// HasRole reports whether authorities contains role. As in Spring Security's
// hasRole, only the role argument is normalised with NormalizeRole. Each
// granted authority must equal the prefixed name exactly.
func HasRole(authorities []string, role string) bool {
	want := NormalizeRole(role)
	for _, a := range authorities {
		if a == want {
			return true
		}
	}
	return false
}
