package model

// RolesTable is the name of the database table that stores Role rows, as
// mapped by the source's @Table(name = "roles"). The table has a unique
// constraint on (username, role).
const RolesTable = "roles"

// UserRef is the view of a user that a Role needs: the username, which is the
// users table's primary key and the value of the roles.username join column.
//
// MIGRATION_NOTE: the source's @ManyToOne User association is held through
// this interface (like OwnerRef in pet.go and PetRef in visit.go) so this file
// does not depend on the layout of the separately migrated User type. Never
// store a typed nil pointer in it; SetUser(nil) clears the association.
type UserRef interface {
	GetUsername() string
}

// Role is a security authority granted to a user, stored in the "roles" table
// (id, username, role).
//
// MIGRATION_NOTE: the JPA back-pointer to User is kept in two forms:
//   - Username holds the raw value of the "username" column, the form in
//     which repositories load and store it;
//   - an optional in-memory UserRef set with SetUser, the equivalent of the
//     Java object reference.
//
// SetUser does not change Username; repositories call PersistedUsername
// when they write the row. Neither form is encoded to JSON (the source's
// @JsonIgnore), which also breaks the User -> roles -> User cycle.
type Role struct {
	BaseEntity
	// Name is the authority name (e.g. "ROLE_ADMIN"), stored in the "role"
	// column.
	Name string `json:"name"`
	// Username is the value of the "username" join column.
	Username string `json:"-"`

	user UserRef
}

// NewRole returns an unsaved Role with the given authority name, granted to
// user. user may be nil and set later with SetUser.
func NewRole(name string, user UserRef) *Role {
	r := &Role{Name: name}
	r.SetUser(user)
	return r
}

// GetUser returns the user this role is granted to, or nil if none is set.
func (r *Role) GetUser() UserRef {
	if r == nil {
		return nil
	}
	return r.user
}

// SetUser assigns the user this role is granted to. Pass nil to clear the
// association. When the role is saved, the user's username is written to the
// "username" column (see PersistedUsername).
func (r *Role) SetUser(user UserRef) {
	r.user = user
}

// GetName returns the role/authority name, stored in the "role" column.
func (r *Role) GetName() string {
	if r == nil {
		return ""
	}
	return r.Name
}

// SetName sets the role/authority name, stored in the "role" column.
func (r *Role) SetName(name string) {
	r.Name = name
}

// PersistedUsername returns the value to write to the "username" column:
// the username of the user set with SetUser, or Username if no user is set.
// Repositories use it on insert and update.
func (r *Role) PersistedUsername() string {
	if r == nil {
		return ""
	}
	if r.user != nil {
		return r.user.GetUsername()
	}
	return r.Username
}
