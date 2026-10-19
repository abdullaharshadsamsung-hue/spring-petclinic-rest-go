// Package sqlstore implements the PetClinic repositories on top of
// database/sql.
package sqlstore

import (
	"errors"
	"fmt"
	"reflect"
)

// ErrObjectRetrievalFailure is the sentinel matched (via errors.Is) by every
// ObjectRetrievalFailureError. It stands in for Spring's
// ObjectRetrievalFailureException in the DataAccessException hierarchy.
var ErrObjectRetrievalFailure = errors.New("object retrieval failure")

// ObjectRetrievalFailureError reports that an entity of a given type with a
// given identifier could not be found.
//
// MIGRATION_NOTE: replaces org.springframework.orm.ObjectRetrievalFailureException.
// The message format matches Spring's:
// "Object of class [<type>] with identifier [<id>]: not found".
type ObjectRetrievalFailureError struct {
	// EntityType is the Go type name of the entity that was looked up.
	EntityType string
	// ID is the identifier that was looked up.
	ID int
}

// Error implements the error interface.
func (e *ObjectRetrievalFailureError) Error() string {
	return fmt.Sprintf("Object of class [%s] with identifier [%d]: not found", e.EntityType, e.ID)
}

// Unwrap lets errors.Is(err, ErrObjectRetrievalFailure) match.
func (e *ObjectRetrievalFailureError) Unwrap() error {
	return ErrObjectRetrievalFailure
}

// identifiable is satisfied by any pointer to a struct embedding
// model.BaseEntity (GetID is declared on *model.BaseEntity).
type identifiable interface {
	GetID() *int
}

// typeName returns the name of T for error messages.
func typeName[T any]() string {
	return reflect.TypeOf((*T)(nil)).Elem().String()
}

// indexByID builds an id -> entity lookup table from entities. Entities
// without an assigned id are skipped. If several entities share an id the
// first one wins, matching the "first match" semantics of
// EntityUtils.getById.
//
// MIGRATION_NOTE: repositories use this instead of repeatedly scanning a
// slice (EntityUtils.getById was O(n) per lookup).
func indexByID[T identifiable](entities []T) map[int]T {
	index := make(map[int]T, len(entities))
	for _, e := range entities {
		id := e.GetID()
		if id == nil {
			continue
		}
		if _, seen := index[*id]; !seen {
			index[*id] = e
		}
	}
	return index
}

// lookupByID returns the entity with the given id from an index built by
// indexByID, or an *ObjectRetrievalFailureError if none exists.
func lookupByID[T identifiable](index map[int]T, id int) (T, error) {
	if e, ok := index[id]; ok {
		return e, nil
	}
	var zero T
	return zero, &ObjectRetrievalFailureError{EntityType: typeName[T](), ID: id}
}

// byID returns the first entity in entities whose id equals id, or an
// *ObjectRetrievalFailureError if none matches. It is the Go port of
// EntityUtils.getById and is used mainly by tests.
//
// MIGRATION_NOTE: Java's Class<T> token + isInstance check is unnecessary:
// the type parameter T fixes the element type at compile time, so every
// element already "is an instance" of T. Entities with a nil id never match
// (Java would have thrown a NullPointerException while unboxing).
func byID[T identifiable](entities []T, id int) (T, error) {
	for _, e := range entities {
		if eid := e.GetID(); eid != nil && *eid == id {
			return e, nil
		}
	}
	var zero T
	return zero, &ObjectRetrievalFailureError{EntityType: typeName[T](), ID: id}
}
