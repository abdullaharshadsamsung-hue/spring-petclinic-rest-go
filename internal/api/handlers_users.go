package api

// This file is the port of UserRestControllerV1: the admin-only endpoint
// POST {contextPath}/api/users, which creates an application user with roles.
//
// MIGRATION_NOTE: @PreAuthorize("hasRole(@roles.ADMIN)") is expressed via the
// injected Authorizer middleware (see handlers_pettypes.go). When security is
// disabled, pass a nil Authorizer. @CrossOrigin(exposedHeaders = "errors, content-type")
// is provided by withRootCORS. Bean Validation (@Valid on the generated UsersApi)
// is done by validateUserDTO. The MapStruct UserMapper is replaced by
// userFromDTO / userToDTO.

import (
	"fmt"
	"net/http"
	"unicode/utf8"

	"migrated-app/internal/model"
	"migrated-app/internal/service"
)

// userRoleAdmin is the role required to create users (Roles.ADMIN without
// the ROLE_ prefix, as hasRole expects).
const userRoleAdmin = "ADMIN"

// userFieldMaxLen is the maximum length of username, password and role name
// (User/Role schemas in openapi.yml).
const userFieldMaxLen = 80

// userRoleDTO is the JSON shape of a Role (the generated RoleDto).
type userRoleDTO struct {
	Name *string `json:"name"`
}

// userDTO is the JSON shape of a User (the generated UserDto).
type userDTO struct {
	Username *string        `json:"username"`
	Password *string        `json:"password,omitempty"`
	Enabled  *bool          `json:"enabled,omitempty"`
	Roles    []*userRoleDTO `json:"roles,omitempty"`
}

// UserHandler serves the /api/users endpoint.
type UserHandler struct {
	users     service.UserService
	authorize Authorizer
}

// NewUserHandler returns a UserHandler backed by users. authorize may be nil,
// in which case no role check is applied (petclinic.security.enable=false).
func NewUserHandler(users service.UserService, authorize Authorizer) *UserHandler {
	if authorize == nil {
		authorize = func(next http.Handler, _ ...string) http.Handler { return next }
	}
	return &UserHandler{users: users, authorize: authorize}
}

// Register mounts the user routes on mux under contextPath (empty for root).
func (h *UserHandler) Register(mux *http.ServeMux, contextPath string) {
	base := normalizeContextPath(contextPath) + "/api/users"
	mux.Handle("POST "+base, withRootCORS(h.authorize(http.HandlerFunc(h.addUser), userRoleAdmin)))
}

// addUser handles POST /api/users. It returns 201 with the saved user (role
// names now carrying the ROLE_ prefix) and, like the source, no Location header.
func (h *UserHandler) addUser(w http.ResponseWriter, r *http.Request) {
	var dto userDTO
	if !petTypeDecodeBody(w, r, &dto) {
		return
	}
	if verr := validateUserDTO(&dto); verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	user := userFromDTO(&dto)
	// SaveUser mutates user in place (role prefixing and back-references);
	// the mutated user is what gets mapped into the response, as in the source.
	if err := h.users.SaveUser(r.Context(), user); err != nil {
		// MIGRATION_NOTE: service.ErrUserHasNoRoles (source:
		// IllegalArgumentException) and persistence failures such as a
		// duplicate username are surfaced through HandleError, which must
		// map them to 500 (verify title "IllegalArgumentException" there).
		HandleError(w, r, fmt.Errorf("saving user: %w", err))
		return
	}
	petTypeWriteJSON(w, http.StatusCreated, userToDTO(user))
}

// validateUserDTO applies the User/Role schema constraints: username required
// (1..80), password 1..80 when present, every role name required (1..80).
func validateUserDTO(dto *userDTO) *ValidationError {
	verr := NewValidationError()
	if dto.Username == nil {
		verr.Add("username", nil, "must not be null")
	} else if n := utf8.RuneCountInString(*dto.Username); n < 1 || n > userFieldMaxLen {
		verr.Add("username", *dto.Username, fmt.Sprintf("size must be between 1 and %d", userFieldMaxLen))
	}
	if dto.Password != nil {
		if n := utf8.RuneCountInString(*dto.Password); n < 1 || n > userFieldMaxLen {
			verr.Add("password", *dto.Password, fmt.Sprintf("size must be between 1 and %d", userFieldMaxLen))
		}
	}
	for i, role := range dto.Roles {
		field := fmt.Sprintf("roles[%d].name", i)
		if role == nil || role.Name == nil {
			verr.Add(field, nil, "must not be null")
			continue
		}
		if n := utf8.RuneCountInString(*role.Name); n < 1 || n > userFieldMaxLen {
			verr.Add(field, *role.Name, fmt.Sprintf("size must be between 1 and %d", userFieldMaxLen))
		}
	}
	return verr
}

// userFromDTO maps a validated userDTO to a new model.User.
func userFromDTO(dto *userDTO) *model.User {
	var username, password string
	if dto.Username != nil {
		username = *dto.Username
	}
	if dto.Password != nil {
		password = *dto.Password
	}
	user := model.NewUser(username, password)
	user.SetEnabled(dto.Enabled)
	if len(dto.Roles) > 0 {
		roles := make([]*model.Role, 0, len(dto.Roles))
		for _, rd := range dto.Roles {
			if rd == nil || rd.Name == nil {
				continue
			}
			roles = append(roles, model.NewRole(*rd.Name, nil))
		}
		user.SetRoles(roles)
	}
	return user
}

// userToDTO maps a model.User to its JSON representation.
func userToDTO(user *model.User) *userDTO {
	username := user.GetUsername()
	password := user.GetPassword()
	dto := &userDTO{
		Username: &username,
		Password: &password,
		Enabled:  user.GetEnabled(),
	}
	for _, role := range user.GetRoles() {
		if role == nil {
			continue
		}
		name := role.GetName()
		dto.Roles = append(dto.Roles, &userRoleDTO{Name: &name})
	}
	return dto
}
