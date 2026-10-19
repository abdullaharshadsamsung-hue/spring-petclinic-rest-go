package store

import (
	"context"

	"migrated-app/internal/model"
)

// SpecialtyRepository is the data-access contract for Specialty entities. It
// is the Go port of the source's
// org.springframework.samples.petclinic.repository.SpecialtyRepository.
//
// MIGRATION_NOTE: The Java Set<String> parameter of findSpecialtiesByNameIn
// becomes a []string. Implementations must treat it as a set: duplicate
// names must not produce duplicate rows (a single "name = ANY($1)" query
// satisfies this naturally). DataAccessException becomes a returned error;
// use errors.Is with ErrNotFound / ErrIntegrity to classify failures.
type SpecialtyRepository interface {
	// FindByID returns the specialty with the given id. It returns an error
	// wrapping ErrNotFound when no such specialty exists.
	FindByID(ctx context.Context, id int) (*model.Specialty, error)

	// FindByNameIn returns every specialty whose name exactly matches one of
	// names (the Spring Data derived query "name IN :names"). The result is
	// empty, not nil, when names is empty or nothing matches.
	FindByNameIn(ctx context.Context, names []string) ([]*model.Specialty, error)

	// FindAll returns every specialty in the data store. The result is
	// empty, not nil, when none exist.
	FindAll(ctx context.Context) ([]*model.Specialty, error)

	// Save upserts the specialty. When specialty.IsNew() reports true (nil
	// id), Save inserts it and sets the generated id on it using
	// INSERT ... RETURNING id. Otherwise Save updates the existing row.
	// A nil specialty is an error.
	Save(ctx context.Context, specialty *model.Specialty) error

	// Delete removes the given specialty from the data store, first removing
	// its rows from the vet_specialties join table so vets no longer
	// reference it. Both deletes must run in the same transaction. A nil
	// specialty is an error.
	Delete(ctx context.Context, specialty *model.Specialty) error
}
