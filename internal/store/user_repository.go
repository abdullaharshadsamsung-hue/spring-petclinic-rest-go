package store

import (
	"context"

	"migrated-app/internal/model"
)

// UserRepository is the data-access contract for User entities. It is the
// Go port of the source's
// org.springframework.samples.petclinic.repository.UserRepository.
//
// MIGRATION_NOTE: The source had JDBC, JPA and Spring Data JPA
// implementations selected by Spring profile. Here a single database/sql
// implementation (internal/store/sqlstore, PostgreSQL) follows the default
// spring-data-jpa semantics, which is a merge keyed by the natural primary
// key, username. DataAccessException becomes a returned error. Transactions
// are not declared here, and the caller (the user service) owns the
// transaction boundary that @Transactional provided in the source.
type UserRepository interface {
	// Save persists the user together with its roles (cascade = ALL).
	//
	// When no row with user.Username exists in model.UsersTable, Save
	// inserts one (columns username, password, enabled). Otherwise it
	// updates password and enabled on the existing row (merge semantics).
	// Each role in user.Roles is then written to model.RolesTable with
	// roles.username set to user.Username, whatever the role's own
	// back-reference says. The user and all of its roles must be written
	// atomically, either inside the caller's transaction or inside one the
	// implementation opens.
	//
	// Save sets nothing on user, because the key is caller-supplied.
	// Errors:
	//   - a nil user or an empty Username is rejected with an error and
	//     nothing is written;
	//   - a constraint violation (for example a duplicate (username, role)
	//     pair or a NOT NULL column) returns an error wrapping ErrIntegrity,
	//     which the API layer maps to an HTTP 4xx response;
	//   - any other driver failure is returned wrapped with context.
	Save(ctx context.Context, user *model.User) error
}
