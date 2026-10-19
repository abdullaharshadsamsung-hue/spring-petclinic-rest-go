// Package model contains the PetClinic domain types shared by the
// repository, service and HTTP layers.
package model

// BaseEntity holds the database-generated primary key shared by every
// PetClinic domain entity. Embed it in a struct to give that struct an ID
// and an IsNew check.
//
// MIGRATION_NOTE: JPA @MappedSuperclass has no Go equivalent. Struct embedding
// does the same job here. The id column is created by each entity's own table
// DDL (SERIAL / GENERATED ... AS IDENTITY in PostgreSQL). Repositories must
// get the generated key with `INSERT ... RETURNING id` and save it with SetID.
//
// ID is a pointer so that "not yet assigned" (Java null) stays distinct from
// any real integer key, including 0. JSON encodes an unassigned ID as null,
// as Jackson does.
type BaseEntity struct {
	ID *int `json:"id"`
}

// GetID returns the entity's identifier, or nil if none has been assigned
// (the entity has not been persisted yet).
func (e *BaseEntity) GetID() *int {
	if e == nil {
		return nil
	}
	return e.ID
}

// SetID assigns the entity's identifier. Any value, including nil, is
// accepted without validation.
func (e *BaseEntity) SetID(id *int) {
	e.ID = id
}

// IsNew reports whether the entity is new (unpersisted), meaning its
// identifier is nil. It is a method, not a field, so it never appears in
// JSON output (the equivalent of @JsonIgnore).
func (e *BaseEntity) IsNew() bool {
	return e == nil || e.ID == nil
}

// IntPtr returns a pointer to v. It is a convenience for assigning literal
// identifiers, e.g. entity.SetID(model.IntPtr(1)).
func IntPtr(v int) *int {
	return &v
}
