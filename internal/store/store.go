// Package store defines the persistence ports (repository interfaces) of the
// PetClinic service. Concrete implementations live in sub-packages such as
// sqlstore (database/sql on PostgreSQL).
//
// MIGRATION_NOTE: In the source, several implementations (jdbc, jpa,
// spring-data-jpa) were selected at runtime through Spring @Profile. The Go
// port follows the agreed plan (default spring-data-jpa semantics) and ships
// a single database/sql implementation; the interfaces remain so services
// can be tested against fakes.
package store

import (
	"context"
	"errors"

	"migrated-app/internal/model"
)

// ErrNotFound is returned (possibly wrapped) by repository lookups when no
// entity matches the requested key. Callers test for it with errors.Is.
//
// MIGRATION_NOTE: the source's spring-data-jpa repositories returned null for
// a missing entity and the REST controllers translated null to 404. In Go a
// missing entity is reported as an error matching ErrNotFound instead of a
// nil pointer with a nil error.
var ErrNotFound = errors.New("store: entity not found")

// PetTypeRepository is the persistence contract for model.PetType.
//
// Every method takes a context so implementations can honour cancellation and
// participate in a transaction carried by the caller (the source relied on
// the @Transactional service layer to supply the transaction boundary).
// Errors from the underlying data source are returned rather than thrown; the
// Spring DataAccessException hierarchy maps to wrapped Go errors.
type PetTypeRepository interface {
	// FindByID returns the pet type with the given primary key, or an error
	// matching ErrNotFound if none exists.
	FindByID(ctx context.Context, id int) (*model.PetType, error)

	// FindByName returns the pet type with the given name, or an error
	// matching ErrNotFound if none exists.
	FindByName(ctx context.Context, name string) (*model.PetType, error)

	// FindAll returns every stored pet type. An empty store yields an empty
	// (non-nil) slice and a nil error.
	FindAll(ctx context.Context) ([]*model.PetType, error)

	// Save persists petType: it is inserted when it is new (IsNew, i.e. it
	// has no id) and updated otherwise. On insert the generated id is written
	// back onto petType (PostgreSQL INSERT ... RETURNING id), mirroring the
	// source's mutate-the-argument semantics.
	Save(ctx context.Context, petType *model.PetType) error

	// Delete removes petType from storage together with its dependent data:
	// the visits of every pet of this type, then those pets, then the type
	// itself. Implementations must perform these steps atomically.
	Delete(ctx context.Context, petType *model.PetType) error
}
