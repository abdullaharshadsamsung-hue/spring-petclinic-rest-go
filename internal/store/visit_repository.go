package store

import (
	"context"

	"migrated-app/internal/model"
)

// VisitRepository is the data-access contract for Visit entities. It is the
// Go port of the source's
// org.springframework.samples.petclinic.repository.VisitRepository.
//
// MIGRATION_NOTE: The source declared this interface with Spring Data naming
// conventions so a derived-query repository could extend it; in Go a single
// database/sql implementation (internal/store/sqlstore) satisfies it, using
// the PostgreSQL visits table (model.VisitsTable) with columns
// id, pet_id, visit_date and description. Spring's DataAccessException
// becomes a returned error: use errors.Is with ErrNotFound (HTTP 404) and
// ErrIntegrity (e.g. a pet_id that references no pet, mapped by the API
// layer). Transactions are not declared here; as in the source, callers at
// the service layer own transaction boundaries. Collection/List return types
// become []*model.Visit, which is empty, never nil, when there are no rows.
// Implementations must order multi-row results deterministically
// (ORDER BY id ASC) so responses are stable.
type VisitRepository interface {
	// Save upserts the visit, following BaseEntity.isNew semantics. When
	// visit.IsNew() reports true (nil id), Save inserts the visit with
	// INSERT ... RETURNING id and sets the generated id on it. Otherwise
	// Save updates the existing row and returns an error wrapping
	// ErrNotFound if no row has that id. A nil visit, or a visit without a
	// persisted pet, is an error.
	Save(ctx context.Context, visit *model.Visit) error

	// FindByPetID returns every visit that belongs to the pet with the
	// given id (the Spring Data derived query findByPetId, resolving to
	// pet.id). The result is empty, not nil, when the pet has no visits or
	// does not exist; this is not an ErrNotFound condition.
	FindByPetID(ctx context.Context, petID int) ([]*model.Visit, error)

	// FindByID returns the visit with the given id. It returns an error
	// wrapping ErrNotFound when no such visit exists, which replaces the
	// source's ObjectRetrievalFailureException / null result. A nil visit
	// is never returned together with a nil error.
	FindByID(ctx context.Context, id int) (*model.Visit, error)

	// FindAll returns every visit in the data store. The result is empty,
	// not nil, when none exist.
	FindAll(ctx context.Context) ([]*model.Visit, error)

	// Delete removes the given visit from the data store. A nil visit, or a
	// visit with a nil id, is an error. As with spring-data-jpa's
	// SimpleJpaRepository.delete, deleting a visit whose row no longer
	// exists is a silent no-op.
	Delete(ctx context.Context, visit *model.Visit) error
}
