package model

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"
)

// VisitsTable is the name of the database table that stores Visit rows, as
// mapped by the source's @Table(name = "visits").
const VisitsTable = "visits"

// dateLayout is the ISO-8601 calendar date layout produced by
// java.time.LocalDate.toString() and expected by the REST API.
const dateLayout = "2006-01-02"

// ErrVisitDescriptionEmpty is reported by Visit.Validate when Description is
// empty. It is the equivalent of the Bean Validation @NotEmpty constraint on
// the source's description field.
var ErrVisitDescriptionEmpty = errors.New("description: must not be empty")

// Date is a calendar date without a time of day or time zone. It replaces
// java.time.LocalDate.
//
// The zero value is the zero time.Time's date (0001-01-01). Date encodes to
// JSON and text as "2006-01-02", and implements sql.Scanner and
// driver.Valuer so it can be read from and written to DATE columns.
type Date struct {
	t time.Time // always midnight UTC
}

// NewDate returns the date for the given year, month and day. Out-of-range
// values are normalized as time.Date does (e.g. October 32 is November 1).
func NewDate(year int, month time.Month, day int) Date {
	return Date{t: time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// DateOf returns the calendar date of t in t's own location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return NewDate(y, m, d)
}

// Today returns the current date in the system's local time zone, as
// LocalDate.now() does.
func Today() Date {
	return DateOf(time.Now())
}

// ParseDate parses an ISO-8601 calendar date such as "2013-01-01".
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("parse date %q: %w", s, err)
	}
	return DateOf(t), nil
}

// Time returns the date as a time.Time at midnight UTC.
func (d Date) Time() time.Time { return d.t }

// IsZero reports whether d is the zero Date.
func (d Date) IsZero() bool { return d.t.IsZero() }

// Before reports whether d is strictly before o.
func (d Date) Before(o Date) bool { return d.t.Before(o.t) }

// After reports whether d is strictly after o.
func (d Date) After(o Date) bool { return d.t.After(o.t) }

// Equal reports whether d and o are the same calendar date.
func (d Date) Equal(o Date) bool { return d.t.Equal(o.t) }

// String formats d as "2006-01-02".
func (d Date) String() string { return d.t.Format(dateLayout) }

// MarshalText implements encoding.TextMarshaler.
func (d Date) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Date) UnmarshalText(b []byte) error {
	parsed, err := ParseDate(string(b))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// MarshalJSON implements json.Marshaler, encoding d as "2006-01-02".
func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler. A JSON null leaves d unchanged,
// following the encoding/json convention.
func (d *Date) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("null")) {
		return nil
	}
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("date: expected JSON string, got %s", b)
	}
	return d.UnmarshalText(b[1 : len(b)-1])
}

// Scan implements sql.Scanner. It accepts time.Time, string and []byte
// values. A NULL is rejected; scan nullable columns into a **Date or
// sql.Null[Date] destination instead.
func (d *Date) Scan(src any) error {
	switch v := src.(type) {
	case time.Time:
		*d = DateOf(v)
		return nil
	case string:
		return d.scanString(v)
	case []byte:
		return d.scanString(string(v))
	case nil:
		return errors.New("date: cannot scan NULL into Date")
	default:
		return fmt.Errorf("date: cannot scan %T into Date", src)
	}
}

func (d *Date) scanString(s string) error {
	if len(s) > len(dateLayout) {
		// Tolerate drivers that return timestamps as text.
		s = s[:len(dateLayout)]
	}
	return d.UnmarshalText([]byte(s))
}

// Value implements driver.Valuer, writing d as a time.Time at midnight UTC.
func (d Date) Value() (driver.Value, error) {
	return d.t, nil
}

// PetRef is the view of a pet that a Visit needs: its identifier. *Pet
// satisfies it through its embedded BaseEntity.
//
// MIGRATION_NOTE: the source's @ManyToOne Pet association is kept as an
// interface so this file does not depend on the concrete Pet type's layout
// (Pet is migrated separately). Callers that need the full pet type-assert
// GetPet() to *Pet. Never store a typed nil pointer in it; SetPet(nil) is
// the way to clear the association.
type PetRef interface {
	GetID() *int
}

// Visit is a veterinary visit of a pet, stored in the "visits" table
// (id, visit_date DATE, description, pet_id -> pets.id).
//
// MIGRATION_NOTE: the JPA @Entity/@Table mapping becomes a plain struct. The
// visits table is created at startup by the embedded schema script
// (src/main/resources/db/postgres/schema.sql). The @ManyToOne/@JoinColumn
// association is represented by PetID (the pet_id column) plus an optional
// in-memory Pet reference; repositories load and save PetID. @NotEmpty on
// description is the explicit Validate method, which handlers must call
// where the Java code used @Valid.
type Visit struct {
	BaseEntity
	// Date is the visit_date column. nil means no date (Java null).
	Date *Date `json:"date"`
	// Description is the description column. It must not be empty.
	Description string `json:"description"`
	// PetID is the pet_id foreign key column.
	PetID *int `json:"petId"`
	// Pet is the associated pet, when loaded. It is not serialized, avoiding
	// the Pet -> Visits -> Pet cycle.
	Pet PetRef `json:"-"`
}

// NewVisit returns a new, unsaved Visit dated today (local time zone), as
// the source's no-arg constructor does.
func NewVisit() *Visit {
	today := Today()
	return &Visit{Date: &today}
}

// GetDate returns the visit date, or nil if none is set.
func (v *Visit) GetDate() *Date {
	if v == nil {
		return nil
	}
	return v.Date
}

// SetDate sets the visit date. nil clears it.
func (v *Visit) SetDate(date *Date) {
	v.Date = date
}

// GetDescription returns the visit description.
func (v *Visit) GetDescription() string {
	if v == nil {
		return ""
	}
	return v.Description
}

// SetDescription sets the visit description. It does no validation; call
// Validate for that.
func (v *Visit) SetDescription(description string) {
	v.Description = description
}

// GetPet returns the pet this visit belongs to, or nil if none is loaded.
func (v *Visit) GetPet() PetRef {
	if v == nil {
		return nil
	}
	return v.Pet
}

// SetPet associates the visit with pet and sets PetID to the pet's id. A nil
// pet clears both the association and PetID.
func (v *Visit) SetPet(pet PetRef) {
	v.Pet = pet
	if pet == nil {
		v.PetID = nil
		return
	}
	v.PetID = pet.GetID()
}

// GetPetID returns the pet_id foreign key: the associated pet's id when a
// pet with an id is set, otherwise PetID.
func (v *Visit) GetPetID() *int {
	if v == nil {
		return nil
	}
	if v.Pet != nil {
		if id := v.Pet.GetID(); id != nil {
			return id
		}
	}
	return v.PetID
}

// Validate checks the @NotEmpty constraint on Description. It returns
// ErrVisitDescriptionEmpty when the description is empty (Java null and ""
// both map to the Go empty string), otherwise nil. Whitespace-only
// descriptions pass, matching @NotEmpty (not @NotBlank).
func (v *Visit) Validate() error {
	if v == nil || v.Description == "" {
		return ErrVisitDescriptionEmpty
	}
	return nil
}
