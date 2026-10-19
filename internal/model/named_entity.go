package model

import "errors"

// ErrNameEmpty is returned by NamedEntity.Validate when the name is empty.
// It is the equivalent of the Bean Validation @NotEmpty constraint on the
// source's name field.
var ErrNameEmpty = errors.New("name: must not be empty")

// NamedEntity adds a name property to BaseEntity. Embed it in domain types
// that have a name, such as PetType, Specialty and Pet.
//
// MIGRATION_NOTE: JPA @MappedSuperclass + @Column(name = "name") is replaced
// by struct embedding. The "name" column lives in each embedding entity's own
// table DDL; this type owns no table.
//
// MIGRATION_NOTE: @NotEmpty is declarative in the source. It runs only when a
// validator is invoked, never inside the setter. Here it is the explicit
// Validate method, which handlers and services must call where the Java code
// used @Valid.
type NamedEntity struct {
	BaseEntity
	Name string `json:"name"`
}

// GetName returns the entity's name.
func (e *NamedEntity) GetName() string {
	if e == nil {
		return ""
	}
	return e.Name
}

// SetName sets the entity's name. It does no validation; call Validate for
// that.
func (e *NamedEntity) SetName(name string) {
	e.Name = name
}

// String returns the entity's name. It is the display representation used
// when the entity is rendered or formatted.
func (e *NamedEntity) String() string {
	return e.GetName()
}

// Validate checks the @NotEmpty constraint on Name. It returns ErrNameEmpty
// if the name is the empty string. Whitespace-only names pass, matching
// @NotEmpty (which is not @NotBlank).
func (e *NamedEntity) Validate() error {
	if e == nil || e.Name == "" {
		return ErrNameEmpty
	}
	return nil
}
