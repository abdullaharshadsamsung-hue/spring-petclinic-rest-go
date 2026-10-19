// Package store defines the persistence contracts of the petclinic service:
// the repository interfaces, the pagination value types and the sentinel
// errors that every concrete implementation (see internal/store/sqlstore)
// must honour.
//
// MIGRATION_NOTE: In the source project these interfaces had several
// implementations (JDBC, JPA, Spring Data JPA) selected by Spring profile.
// The agreed plan keeps a single database/sql implementation (PostgreSQL,
// $n placeholders, INSERT ... RETURNING id) that follows the default
// spring-data-jpa semantics. Spring's unchecked DataAccessException hierarchy
// becomes ordinary Go errors. Use errors.Is with ErrNotFound and ErrIntegrity
// to tell those cases apart from other failures.
package store

import (
	"context"
	"fmt"
	"strings"

	"migrated-app/internal/model"
)

// Error is the type of the sentinel errors exported by this package. It is a
// string type so the sentinels can be declared as constants and cannot be
// reassigned by other packages.
type Error string

// Error implements the error interface.
func (e Error) Error() string { return string(e) }

const (
	// ErrNotFound is returned when a requested entity does not exist. It
	// replaces Spring's DataRetrievalFailureException /
	// ObjectRetrievalFailureException / EmptyResultDataAccessException.
	ErrNotFound Error = "store: entity not found"

	// ErrIntegrity is returned when a write violates a database constraint,
	// such as a foreign key, unique or not-null constraint. It replaces
	// Spring's DataIntegrityViolationException.
	ErrIntegrity Error = "store: data integrity violation"
)

// IntegrityViolation wraps ErrIntegrity with a human-readable detail message
// and, optionally, the underlying driver error. errors.Is(err, ErrIntegrity)
// reports true for the result.
func IntegrityViolation(detail string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%w: %s", ErrIntegrity, detail)
	}
	return fmt.Errorf("%w: %s: %v", ErrIntegrity, detail, cause)
}

// SortOrder is a single sort criterion, the equivalent of Spring Data's
// Sort.Order. Property is the entity property name, which implementations
// must map to a whitelisted column and never interpolate verbatim.
type SortOrder struct {
	Property   string
	Descending bool
}

// String renders the order in Spring's "property: DIRECTION" style.
func (o SortOrder) String() string {
	dir := "ASC"
	if o.Descending {
		dir = "DESC"
	}
	return o.Property + ": " + dir
}

// PageRequest describes one page of a paginated query, replacing Spring
// Data's Pageable. Page is zero-based, as in Spring Data.
type PageRequest struct {
	Page int
	Size int
	Sort []SortOrder
}

// NewPageRequest validates its arguments and builds a PageRequest. It
// returns an error under the same conditions in which Spring's PageRequest.of
// throws IllegalArgumentException: a negative page index, a size below one,
// or a sort order with an empty property.
func NewPageRequest(page, size int, sort ...SortOrder) (PageRequest, error) {
	if page < 0 {
		return PageRequest{}, fmt.Errorf("store: page index must not be less than zero, got %d", page)
	}
	if size < 1 {
		return PageRequest{}, fmt.Errorf("store: page size must not be less than one, got %d", size)
	}
	orders := make([]SortOrder, 0, len(sort))
	for _, o := range sort {
		if strings.TrimSpace(o.Property) == "" {
			return PageRequest{}, fmt.Errorf("store: sort property must not be empty")
		}
		orders = append(orders, o)
	}
	return PageRequest{Page: page, Size: size, Sort: orders}, nil
}

// Offset returns the number of rows to skip for this page, computed as
// page * size.
func (p PageRequest) Offset() int {
	return p.Page * p.Size
}

// Page is one slice of a paginated result, replacing Spring Data's Page<T>.
type Page[T any] struct {
	// Content holds the elements of this page. It is never nil.
	Content []T
	// TotalElements is the total number of elements across all pages.
	TotalElements int64
	// TotalPages is the total number of pages.
	TotalPages int
	// Number is the zero-based index of this page.
	Number int
	// Size is the requested page size.
	Size int
}

// NewPage builds a Page from the content of one page, the request that
// produced it and the total element count. TotalPages is derived the same
// way as Spring's PageImpl.
func NewPage[T any](content []T, req PageRequest, total int64) Page[T] {
	if content == nil {
		content = []T{}
	}
	totalPages := 1
	if req.Size > 0 {
		totalPages = int((total + int64(req.Size) - 1) / int64(req.Size))
	}
	return Page[T]{
		Content:       content,
		TotalElements: total,
		TotalPages:    totalPages,
		Number:        req.Page,
		Size:          req.Size,
	}
}

// OwnerRepository is the data-access contract for Owner entities.
type OwnerRepository interface {
	// FindByLastName returns all owners whose last name starts with the
	// given prefix. The result is empty, not nil, when none match.
	FindByLastName(ctx context.Context, lastName string) ([]*model.Owner, error)
	// FindByID returns the owner with the given id, or ErrNotFound.
	FindByID(ctx context.Context, id int) (*model.Owner, error)
	// Save inserts the owner when it is new (nil id), otherwise updates it.
	// On insert the generated id is set on the owner.
	Save(ctx context.Context, owner *model.Owner) error
	// FindAll returns every owner. The result is empty, not nil, when none
	// exist.
	FindAll(ctx context.Context) ([]*model.Owner, error)
	// FindAllPage returns one page of owners.
	FindAllPage(ctx context.Context, req PageRequest) (Page[*model.Owner], error)
	// Delete removes the owner from the data store.
	Delete(ctx context.Context, owner *model.Owner) error
}

// PetTypeRepository is the data-access contract for PetType entities.
type PetTypeRepository interface {
	// FindByID returns the pet type with the given id, or ErrNotFound.
	FindByID(ctx context.Context, id int) (*model.PetType, error)
	// FindByName returns the pet type with the given name, or ErrNotFound.
	FindByName(ctx context.Context, name string) (*model.PetType, error)
	// FindAll returns every pet type. The result is empty, not nil, when
	// none exist.
	FindAll(ctx context.Context) ([]*model.PetType, error)
	// Save inserts the pet type when it is new (nil id), otherwise updates
	// it. On insert the generated id is set on the pet type.
	Save(ctx context.Context, petType *model.PetType) error
	// Delete removes the pet type from the data store.
	Delete(ctx context.Context, petType *model.PetType) error
}

// PetRepository is the data-access contract for Pet entities and for
// reading the PetType lookup table. It is the Go port of the source's
// org.springframework.samples.petclinic.repository.PetRepository.
//
// MIGRATION_NOTE: The overloaded Java methods findAll() and
// findAll(Pageable) become FindAll and FindAllPage, because Go has no
// overloading. Every method takes a context and returns an error instead of
// throwing DataAccessException.
type PetRepository interface {
	// FindPetTypes returns all PetType records. The result is empty, not
	// nil, when none exist.
	FindPetTypes(ctx context.Context) ([]*model.PetType, error)

	// FindByID returns the pet with the given id. It returns an error
	// wrapping ErrNotFound when no such pet exists, which replaces the
	// source's DataRetrievalFailureException.
	FindByID(ctx context.Context, id int) (*model.Pet, error)

	// Save upserts the pet. When pet.IsNew() reports true (nil id), Save
	// inserts the pet and sets the generated id on it using
	// INSERT ... RETURNING id. Otherwise Save updates the existing row.
	// A nil pet is an error.
	Save(ctx context.Context, pet *model.Pet) error

	// FindAll returns every pet in the data store. The result is empty,
	// not nil, when none exist.
	FindAll(ctx context.Context) ([]*model.Pet, error)

	// FindAllPage returns a single page of pets according to req's page
	// number, size and optional sort.
	FindAllPage(ctx context.Context, req PageRequest) (Page[*model.Pet], error)

	// Delete removes the given pet from the data store, including the
	// visits that belong to it, as the source's cascade does. A nil pet is
	// an error.
	Delete(ctx context.Context, pet *model.Pet) error
}
