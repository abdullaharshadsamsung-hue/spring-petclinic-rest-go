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
	"fmt"

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

// integrityError is the concrete type behind ErrIntegrity. It implements the
// IntegrityViolation() marker method that the api package's error handler
// looks for (api cannot be imported from here), so a wrapped ErrIntegrity is
// rendered exactly like Spring's DataIntegrityViolationException.
type integrityError struct{ msg string }

func (e *integrityError) Error() string { return e.msg }

// IntegrityViolation marks the error as a data integrity violation.
func (e *integrityError) IntegrityViolation() bool { return true }

// ErrIntegrity is returned (wrapped) by repositories when a write violates a
// data constraint, e.g. a foreign key referencing a missing row or a unique
// constraint. It replaces Spring's DataIntegrityViolationException. Callers
// test for it with errors.Is.
var ErrIntegrity error = &integrityError{msg: "store: data integrity violation"}

// PageRequest selects one page of a result set. It replaces Spring Data's
// Pageable. Page is zero-based; Size is the maximum number of elements.
type PageRequest struct {
	// Page is the zero-based page index.
	Page int
	// Size is the page size; it must be at least 1.
	Size int
}

// NewPageRequest validates and returns a PageRequest, mirroring
// PageRequest.of(page, size), which rejects a negative page index and a page
// size smaller than one.
func NewPageRequest(page, size int) (PageRequest, error) {
	if page < 0 {
		return PageRequest{}, fmt.Errorf("store: page index must not be less than zero, got %d", page)
	}
	if size < 1 {
		return PageRequest{}, fmt.Errorf("store: page size must not be less than one, got %d", size)
	}
	return PageRequest{Page: page, Size: size}, nil
}

// Offset returns the number of rows to skip for this page (page * size),
// suitable for a SQL OFFSET clause.
func (p PageRequest) Offset() int64 {
	return int64(p.Page) * int64(p.Size)
}

// Page is one page of a larger result set. It replaces Spring Data's Page.
type Page[T any] struct {
	// Content holds the elements of this page; never nil.
	Content []T
	// Page is the zero-based index of this page.
	Page int
	// Size is the requested page size.
	Size int
	// TotalElements is the total number of elements across all pages.
	TotalElements int64
}

// NewPage builds a Page for the given request. A nil content slice is
// replaced by an empty one so it serialises as [] rather than null.
func NewPage[T any](content []T, req PageRequest, total int64) Page[T] {
	if content == nil {
		content = []T{}
	}
	return Page[T]{Content: content, Page: req.Page, Size: req.Size, TotalElements: total}
}

// TotalPages returns ceil(TotalElements / Size). As in Spring's PageImpl, a
// page size of zero (unpaged) reports a single page.
func (p Page[T]) TotalPages() int {
	if p.Size <= 0 {
		return 1
	}
	size := int64(p.Size)
	return int((p.TotalElements + size - 1) / size)
}

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

// OwnerRepository is the persistence contract for model.Owner. It replaces
// the source's OwnerRepository interface; Java's overloaded findByLastName
// and findAll become separate plain and *Page methods.
//
// Errors from the data source are returned (wrapped) instead of thrown as
// DataAccessException.
type OwnerRepository interface {
	// FindByLastName returns every owner whose last name starts with
	// lastName (SQL: last_name LIKE $1 || '%'), with their pets loaded. No
	// match yields an empty (non-nil) slice and a nil error.
	FindByLastName(ctx context.Context, lastName string) ([]*model.Owner, error)

	// FindByLastNamePage is the paginated form of FindByLastName: it returns
	// the requested page of owners whose last name starts with lastName,
	// together with the total number of matches.
	FindByLastNamePage(ctx context.Context, lastName string, req PageRequest) (Page[*model.Owner], error)

	// FindByID returns the owner with the given id, with its pets (and their
	// visits) loaded, or an error matching ErrNotFound if none exists.
	FindByID(ctx context.Context, id int) (*model.Owner, error)

	// Save persists owner: it is inserted when it is new (IsNew, i.e. it has
	// no id) and updated otherwise. On insert the generated id is written
	// back onto owner (PostgreSQL INSERT ... RETURNING id).
	Save(ctx context.Context, owner *model.Owner) error

	// FindAll returns every stored owner. An empty store yields an empty
	// (non-nil) slice and a nil error.
	FindAll(ctx context.Context) ([]*model.Owner, error)

	// FindAllPage returns the requested page of all owners together with
	// the total number of owners.
	FindAllPage(ctx context.Context, req PageRequest) (Page[*model.Owner], error)

	// Delete removes owner from storage together with its dependent data
	// (the visits of its pets, then its pets), mirroring the source's
	// cascade. Implementations must perform these steps atomically.
	Delete(ctx context.Context, owner *model.Owner) error
}
