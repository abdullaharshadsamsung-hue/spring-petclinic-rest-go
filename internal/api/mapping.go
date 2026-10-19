package api

// This file is the Go port of the MapStruct UserMapper interface
// (org.springframework.samples.petclinic.mapper.UserMapper). MapStruct
// generated UserMapperImpl at compile time. Here the mappers are plain,
// hand-written functions.
//
// MIGRATION_NOTE: The vet mappers (VetToDTO, VetsToDTOs, VetFromDTO,
// VetFromFieldsDTO, VetDTO, VetFieldsDTO) also live in internal/api/mapping.go.
// If this output replaces that file instead of being merged into it, carry
// those declarations over (or move them to vet_mapping.go). Otherwise
// mapping_test.go and the vet handlers will no longer compile.
//
// MIGRATION_NOTE: MapStruct's null semantics are kept on purpose:
//   - A nil input object maps to nil.
//   - A nil collection maps to a nil slice, which encodes as JSON null. This
//     matches Jackson's default output for a null field.
//   - A nil element inside a collection maps to a nil element, so the output
//     length always equals the input length.
//
// MIGRATION_NOTE: The model package has no zero-argument constructors.
//   - Users are built with model.NewUser(username, password).
//   - Roles are built with model.NewRole(name, user), the same constructor
//     that User.AddRole uses.
// @Mapping(target = "id", ignore = true) and @Mapping(target = "user",
// ignore = true) on toRole mean RoleFromDTO only copies the name. It passes a
// nil owning user to model.NewRole, and the id stays unset. When the user is
// saved, the repository/service links roles to their user, as
// JdbcUserRepositoryImpl and UserServiceImpl did.
//
// REQUIRES MANUAL REVIEW: The accessor signatures below are taken from the
// migrated model package (internal/model/user.go and role.go):
//   - NewUser(username, password string) *User
//   - NewRole(name string, user *User) *Role
//   - GetEnabled/SetEnabled use *bool.
//   - GetRoles/SetRoles use []*model.Role.
// Adjust these calls if those signatures differ, for example if the
// constructors also return an error.

import (
	"migrated-app/internal/model"
)

// RoleDTO is the REST representation of a user role. It mirrors the
// OpenAPI-generated RoleDto, which exposes only the role name.
type RoleDTO struct {
	// Name is the role name, e.g. "ROLE_ADMIN" or "OWNER_ADMIN".
	Name string `json:"name"`
}

// UserDTO is the REST representation of an application user. It mirrors the
// OpenAPI-generated UserDto.
type UserDTO struct {
	// Username is the unique login name of the user.
	Username string `json:"username"`
	// Password is the user's password. It is accepted and returned as-is,
	// matching the source mapper.
	Password string `json:"password"`
	// Enabled reports whether the account is active. It is a pointer so that
	// an absent/null value is preserved, like Java's Boolean.
	Enabled *bool `json:"enabled"`
	// Roles lists the roles granted to the user.
	Roles []*RoleDTO `json:"roles"`
}

// RoleFromDTO converts a RoleDTO into a Role entity (port of toRole).
// Only the name is copied. The id and the owning-user back-reference are
// intentionally left unset, so a nil user is passed to model.NewRole.
// A nil DTO yields nil.
func RoleFromDTO(dto *RoleDTO) *model.Role {
	if dto == nil {
		return nil
	}
	return model.NewRole(dto.Name, nil)
}

// RoleToDTO converts a Role entity into a RoleDTO by copying the name
// (port of toRoleDto). A nil role yields nil.
func RoleToDTO(role *model.Role) *RoleDTO {
	if role == nil {
		return nil
	}
	return &RoleDTO{Name: role.GetName()}
}

// RolesToDTOs converts a collection of Role entities into RoleDTOs by
// applying RoleToDTO to each element (port of toRoleDtos). A nil slice yields
// nil, and nil elements are kept as nil so the lengths match.
func RolesToDTOs(roles []*model.Role) []*RoleDTO {
	if roles == nil {
		return nil
	}
	dtos := make([]*RoleDTO, 0, len(roles))
	for _, role := range roles {
		dtos = append(dtos, RoleToDTO(role))
	}
	return dtos
}

// RolesFromDTOs converts a collection of RoleDTOs into Role entities by
// applying RoleFromDTO to each element (port of toRoles). A nil slice yields
// nil, and nil elements are kept as nil so the lengths match.
func RolesFromDTOs(dtos []*RoleDTO) []*model.Role {
	if dtos == nil {
		return nil
	}
	roles := make([]*model.Role, 0, len(dtos))
	for _, dto := range dtos {
		roles = append(roles, RoleFromDTO(dto))
	}
	return roles
}

// UserFromDTO converts a UserDTO into a User entity (port of toUser).
// Username and password are passed to model.NewUser. Enabled is copied, and
// the roles are mapped with RolesFromDTOs. A nil DTO yields nil.
func UserFromDTO(dto *UserDTO) *model.User {
	if dto == nil {
		return nil
	}
	user := model.NewUser(dto.Username, dto.Password)
	user.SetEnabled(copyBoolPtr(dto.Enabled))
	user.SetRoles(RolesFromDTOs(dto.Roles))
	return user
}

// UserToDTO converts a User entity into a UserDTO (port of toUserDto).
// It copies username, password and enabled, and maps the roles with
// RolesToDTOs. A nil user yields nil.
func UserToDTO(user *model.User) *UserDTO {
	if user == nil {
		return nil
	}
	return &UserDTO{
		Username: user.GetUsername(),
		Password: user.GetPassword(),
		Enabled:  copyBoolPtr(user.GetEnabled()),
		Roles:    RolesToDTOs(user.GetRoles()),
	}
}

// copyBoolPtr returns an independent copy of a *bool, so the DTO and the
// entity never share mutable state. A nil pointer stays nil.
func copyBoolPtr(b *bool) *bool {
	if b == nil {
		return nil
	}
	v := *b
	return &v
}
