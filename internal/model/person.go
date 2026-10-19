package model

import "errors"

// ErrFirstNameEmpty is reported by Person.Validate when FirstName is empty.
// It is the equivalent of the Bean Validation @NotEmpty constraint on the
// source's firstName field.
var ErrFirstNameEmpty = errors.New("firstName: must not be empty")

// ErrLastNameEmpty is reported by Person.Validate when LastName is empty.
// It is the equivalent of the Bean Validation @NotEmpty constraint on the
// source's lastName field.
var ErrLastNameEmpty = errors.New("lastName: must not be empty")

// Person adds first and last names to BaseEntity. Embed it in domain types
// that represent people, such as Owner and Vet.
//
// MIGRATION_NOTE: JPA @MappedSuperclass with @Column(name = "first_name") and
// @Column(name = "last_name") is replaced by struct embedding. The first_name
// and last_name columns live in each embedding entity's own table DDL (owners,
// vets); Person owns no table and is never queried polymorphically.
//
// MIGRATION_NOTE: @NotEmpty is declarative in the source. It runs only when a
// validator is invoked, never inside the setters. Here it is the explicit
// Validate method, which handlers and services must call where the Java code
// used @Valid. Java null and "" both map to the Go empty string, and both fail
// validation, as they do under @NotEmpty.
type Person struct {
	BaseEntity
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// GetFirstName returns the person's first name.
func (p *Person) GetFirstName() string {
	if p == nil {
		return ""
	}
	return p.FirstName
}

// SetFirstName sets the person's first name. It does no validation; call
// Validate for that.
func (p *Person) SetFirstName(firstName string) {
	p.FirstName = firstName
}

// GetLastName returns the person's last name.
func (p *Person) GetLastName() string {
	if p == nil {
		return ""
	}
	return p.LastName
}

// SetLastName sets the person's last name. It does no validation; call
// Validate for that.
func (p *Person) SetLastName(lastName string) {
	p.LastName = lastName
}

// Validate checks the @NotEmpty constraints on FirstName and LastName. Like
// Bean Validation, it reports every violation rather than stopping at the
// first: the returned error joins ErrFirstNameEmpty and/or ErrLastNameEmpty,
// so callers can test each with errors.Is. It returns nil when both names are
// non-empty. Whitespace-only names pass, matching @NotEmpty (not @NotBlank).
func (p *Person) Validate() error {
	if p == nil {
		return errors.Join(ErrFirstNameEmpty, ErrLastNameEmpty)
	}
	var errs []error
	if p.FirstName == "" {
		errs = append(errs, ErrFirstNameEmpty)
	}
	if p.LastName == "" {
		errs = append(errs, ErrLastNameEmpty)
	}
	return errors.Join(errs...)
}
