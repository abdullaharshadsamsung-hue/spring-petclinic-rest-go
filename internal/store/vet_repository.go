package store

import (
	"context"

	"migrated-app/internal/model"
)

// VetRepository is the data-access contract for Vet entities. It is the Go
// port of the source's
// org.springframework.samples.petclinic.repository.VetRepository.
//
// MIGRATION_NOTE: The Java Collection<Vet> return type becomes []*model.Vet.
// Vet's @ManyToMany(fetch = EAGER) specialties association must be loaded
// together with each vet (from model.VetSpecialtiesTable joined with the
// specialties table) and kept in sync on Save and Delete. This reproduces
// the eager fetch and the join-table maintenance that JPA performed. Every
// method takes a context and returns an error instead of throwing
// DataAccessException. Use errors.Is with ErrNotFound / ErrIntegrity to
// classify failures. HTTP layer mapping: ErrNotFound → 404 Not Found.
type VetRepository interface {
	// FindAll returns every vet, each with its specialties populated. The
	// result is empty, not nil, when there are no vets.
	FindAll(ctx context.Context) ([]*model.Vet, error)

	// FindByID returns the vet with the given id and its specialties. When
	// no such vet exists it returns a nil vet and an error wrapping
	// ErrNotFound. It never returns a zero-value vet in that case.
	FindByID(ctx context.Context, id int) (*model.Vet, error)

	// Save upserts the vet. When the vet is new (nil id), Save inserts it
	// and sets the generated id on it with INSERT ... RETURNING id.
	// Otherwise Save updates the existing row. In both cases Save replaces
	// the vet's rows in the vet_specialties join table so that they match
	// vet.GetSpecialties(). This must happen in a single transaction.
	// Referencing a specialty that does not exist returns an error wrapping
	// ErrIntegrity. A nil vet is an error.
	Save(ctx context.Context, vet *model.Vet) error

	// Delete removes the vet and its vet_specialties rows in a single
	// transaction. It does not delete the specialties themselves. A nil or
	// unsaved vet is an error.
	Delete(ctx context.Context, vet *model.Vet) error
}
