package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

// rolePrefix is the prefix every persisted authority name must carry, as in
// Spring Security's hasRole() convention.
const rolePrefix = "ROLE_"

// ErrUserHasNoRoles is returned by UserService.SaveUser when the user has no
// roles. It replaces the source's IllegalArgumentException and keeps its exact
// message so error bodies stay identical. The API layer should map it with
// errors.Is (the source surfaced it as HTTP 500, title IllegalArgumentException).
var ErrUserHasNoRoles = errors.New("User must have at least a role set!")

// ErrNilUser is returned by UserService.SaveUser when it is given a nil user.
var ErrNilUser = errors.New("user must not be nil")

// UserService is the service-layer contract for persisting User accounts.
// It is the Go port of the source's UserService interface. Handlers depend on
// this interface so tests can supply a fake.
type UserService interface {
	// SaveUser validates the user and persists it with its roles. It fails
	// with ErrUserHasNoRoles when no roles are set. Before saving, it adds
	// the "ROLE_" prefix to every role name that lacks it and sets each
	// role's owning user where none is set.
	SaveUser(ctx context.Context, user *model.User) error
}

// userService is the default UserService implementation. It is the port of
// UserServiceImpl.
//
// MIGRATION_NOTE: @Transactional is delegated to store.UserRepository.Save.
// Its contract requires the user and all of its roles to be written
// atomically (upsert user, replace roles, one tx). The service makes only one
// data-layer call, so it needs no transaction of its own.
//
// MIGRATION_NOTE: like the source, the password is persisted exactly as given
// (no hashing). Review before production use.
type userService struct {
	users store.UserRepository
}

// Compile-time check that userService satisfies UserService.
var _ UserService = (*userService)(nil)

// NewUserService returns a UserService that persists users through repo.
func NewUserService(repo store.UserRepository) UserService {
	return &userService{users: repo}
}

// SaveUser implements UserService. Like the source, it mutates the caller's
// user in place (role names and role back-references).
func (s *userService) SaveUser(ctx context.Context, user *model.User) error {
	if user == nil {
		return ErrNilUser
	}
	roles := user.GetRoles()
	if len(roles) == 0 {
		return ErrUserHasNoRoles
	}

	for _, role := range roles {
		if role == nil {
			// MIGRATION_NOTE: Java would throw an NPE here. Reject the input
			// explicitly instead.
			return fmt.Errorf("save user %q: nil role in roles", user.GetUsername())
		}
		if !strings.HasPrefix(role.GetName(), rolePrefix) {
			role.SetName(rolePrefix + role.GetName())
		}
		if role.GetUser() == nil {
			role.SetUser(user)
		}
	}

	if err := s.users.Save(ctx, user); err != nil {
		return fmt.Errorf("save user %q: %w", user.GetUsername(), err)
	}
	return nil
}
