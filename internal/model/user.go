package model

// UsersTable is the name of the database table that stores User rows, as
// mapped by the source's @Table(name = "users"). Its columns are username
// (primary key), password and enabled.
const UsersTable = "users"

// User is a security principal stored in the "users" table. It is keyed by
// a natural primary key, Username, which the caller must supply (there is no
// generated id).
//
// MIGRATION_NOTE: the JPA @OneToMany(mappedBy = "user", cascade = ALL,
// fetch = EAGER) association becomes a plain slice of *Role. Repositories
// must load it together with the user (select from roles where
// username = $1) and must persist and delete it whenever they save or delete
// the user, which reproduces the eager fetch and the cascade. The slice has
// set semantics: Role does not override equality, so the Java HashSet
// compared roles by identity. Appending a distinct *Role is equivalent.
type User struct {
	// Username is the primary key, stored in the "username" column.
	Username string `json:"username"`
	// Password is the stored credential, in the "password" column. The
	// entity stores it exactly as given and never encodes it.
	Password string `json:"password"`
	// Enabled is the "enabled" column. Spring Security-style authentication
	// lets the user log in only when it is true. nil means unset (the
	// source's Boolean could be null).
	Enabled *bool `json:"enabled"`
	// Roles are the authorities granted to the user, stored in the "roles"
	// table and joined on roles.username.
	Roles []*Role `json:"roles"`
}

// NewUser returns an unsaved User with the given username and password.
// Enabled and Roles are left unset, as on a newly constructed Java entity.
func NewUser(username, password string) *User {
	return &User{Username: username, Password: password}
}

// GetUsername returns the username (primary key). It also makes *User
// satisfy UserRef, so a user can be set as a Role's owner.
func (u *User) GetUsername() string {
	if u == nil {
		return ""
	}
	return u.Username
}

// SetUsername sets the username (primary key).
func (u *User) SetUsername(username string) {
	u.Username = username
}

// GetPassword returns the stored credential exactly as it was set.
func (u *User) GetPassword() string {
	if u == nil {
		return ""
	}
	return u.Password
}

// SetPassword stores the credential as given. No encoding is performed.
func (u *User) SetPassword(password string) {
	u.Password = password
}

// GetEnabled returns the enabled flag, or nil if it is unset.
func (u *User) GetEnabled() *bool {
	if u == nil {
		return nil
	}
	return u.Enabled
}

// SetEnabled sets the enabled flag. Pass nil to clear it.
func (u *User) SetEnabled(enabled *bool) {
	u.Enabled = enabled
}

// IsEnabled reports whether the user is enabled. An unset flag counts as
// disabled.
func (u *User) IsEnabled() bool {
	e := u.GetEnabled()
	return e != nil && *e
}

// GetRoles returns the user's roles. The slice is nil if no roles have been
// set or added.
func (u *User) GetRoles() []*Role {
	if u == nil {
		return nil
	}
	return u.Roles
}

// SetRoles replaces the user's roles. Back-references on the roles are not
// changed, as in the source.
func (u *User) SetRoles(roles []*Role) {
	u.Roles = roles
}

// AddRole creates a new Role with the given name and adds it to the user's
// roles, initializing the collection if it is nil. Like the source, it does
// not set the role's back-reference to this user. Repositories write the
// owning user's username to the roles.username column when they cascade the
// save.
func (u *User) AddRole(roleName string) {
	if u.Roles == nil {
		u.Roles = make([]*Role, 0, 1)
	}
	u.Roles = append(u.Roles, NewRole(roleName, nil))
}
