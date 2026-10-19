package model

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// OwnersTable is the name of the database table that stores Owner rows, as
// mapped by the source's @Table(name = "owners").
const OwnersTable = "owners"

// ErrAddressEmpty is reported by Owner.Validate when Address is empty
// (@NotEmpty on the source's address field).
var ErrAddressEmpty = errors.New("address: must not be empty")

// ErrCityEmpty is reported by Owner.Validate when City is empty
// (@NotEmpty on the source's city field).
var ErrCityEmpty = errors.New("city: must not be empty")

// ErrTelephoneEmpty is reported by Owner.Validate when Telephone is empty
// (@NotEmpty on the source's telephone field).
var ErrTelephoneEmpty = errors.New("telephone: must not be empty")

// ErrTelephoneInvalid is reported by Owner.Validate when Telephone is not
// exactly 10 ASCII digits (@Digits(integer = 10) and
// @Pattern("^[0-9]{10}$") on the source's telephone field).
var ErrTelephoneInvalid = errors.New("telephone: Phone number must be exactly 10 digits")

// telephonePattern is the source's @Pattern regexp. [0-9] only matches
// ASCII digits, as in Java.
var telephonePattern = regexp.MustCompile(`^[0-9]{10}$`)

// Owner is a pet owner, stored in the "owners" table
// (id, first_name, last_name, address, city, telephone).
//
// MIGRATION_NOTE: the JPA @Entity/@Table mapping becomes a plain struct. The
// owners table is created at startup by the embedded schema script. The id,
// first_name and last_name columns come from the embedded Person/BaseEntity
// (@MappedSuperclass in the source).
//
// MIGRATION_NOTE: @OneToMany(cascade = ALL, fetch = EAGER) on pets is not
// automatic in Go. Repositories must load the pets along with the owner, and
// must save or delete them whenever the owner is saved or deleted.
type Owner struct {
	Person
	// Address is the address column.
	Address string `json:"address"`
	// City is the city column.
	City string `json:"city"`
	// Telephone is the telephone column.
	Telephone string `json:"telephone"`
	// pets is the internal pet collection, with set semantics by identity.
	// Pet owns the owner_id foreign key (mappedBy = "owner").
	pets []*Pet
}

// NewOwner returns an empty, new (unsaved) Owner.
func NewOwner() *Owner {
	return &Owner{}
}

// GetAddress returns the owner's street address.
func (o *Owner) GetAddress() string {
	if o == nil {
		return ""
	}
	return o.Address
}

// SetAddress sets the owner's street address without validating it.
func (o *Owner) SetAddress(address string) {
	o.Address = address
}

// GetCity returns the owner's city.
func (o *Owner) GetCity() string {
	if o == nil {
		return ""
	}
	return o.City
}

// SetCity sets the owner's city without validating it.
func (o *Owner) SetCity(city string) {
	o.City = city
}

// GetTelephone returns the owner's telephone number.
func (o *Owner) GetTelephone() string {
	if o == nil {
		return ""
	}
	return o.Telephone
}

// SetTelephone sets the owner's telephone number without validating it.
func (o *Owner) SetTelephone(telephone string) {
	o.Telephone = telephone
}

// GetPets returns the owner's pets sorted case-insensitively by name. The
// result is a fresh copy and is never nil. Changing it does not affect the
// owner.
//
// MIGRATION_NOTE: Java returns Collections.unmodifiableList. Go has no
// read-only slices, so this returns a defensive copy. nil entries sort last
// instead of throwing.
func (o *Owner) GetPets() []*Pet {
	if o == nil {
		return []*Pet{}
	}
	sorted := make([]*Pet, len(o.pets))
	copy(sorted, o.pets)
	sort.SliceStable(sorted, func(i, j int) bool {
		pi, pj := sorted[i], sorted[j]
		switch {
		case pi == nil:
			return false
		case pj == nil:
			return true
		default:
			return strings.ToLower(pi.Name) < strings.ToLower(pj.Name)
		}
	})
	return sorted
}

// SetPets replaces the pet collection with a new set copied from pets.
// Duplicate pointers are collapsed, as building a HashSet does. It does not
// set the owner back-reference on any pet, matching the source.
func (o *Owner) SetPets(pets []*Pet) {
	set := make([]*Pet, 0, len(pets))
	seen := make(map[*Pet]struct{}, len(pets))
	for _, p := range pets {
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		set = append(set, p)
	}
	o.pets = set
}

// AddPet adds pet to the owner's collection and sets the pet's owner to this
// owner. Adding the same pet twice leaves the collection unchanged. A nil pet
// is ignored. (Java would throw a NullPointerException.)
func (o *Owner) AddPet(pet *Pet) {
	if pet == nil {
		return
	}
	found := false
	for _, p := range o.pets {
		if p == pet {
			found = true
			break
		}
	}
	if !found {
		o.pets = append(o.pets, pet)
	}
	pet.SetOwner(o)
}

// GetPet returns the pet with the given name, compared case-insensitively.
// New (unsaved) pets are included. It returns nil if there is no match. It is
// equivalent to GetPetIgnoreNew(name, false).
func (o *Owner) GetPet(name string) *Pet {
	return o.GetPetIgnoreNew(name, false)
}

// GetPetIgnoreNew returns the pet with the given name, compared
// case-insensitively, or nil if there is no match. When ignoreNew is true,
// pets without an id (new pets) are skipped.
//
// MIGRATION_NOTE: Go has no overloading, so the Java getPet(String, boolean)
// overload is named GetPetIgnoreNew.
func (o *Owner) GetPetIgnoreNew(name string, ignoreNew bool) *Pet {
	if o == nil {
		return nil
	}
	name = strings.ToLower(name)
	for _, pet := range o.pets {
		if pet == nil {
			continue
		}
		if ignoreNew && pet.IsNew() {
			continue
		}
		if strings.ToLower(pet.Name) == name {
			return pet
		}
	}
	return nil
}

// GetPetByID returns the pet with the given id, or nil if there is none.
//
// MIGRATION_NOTE: the Java stream filter would throw a NullPointerException
// on a pet without an id. Here such pets are skipped. A nil petID matches
// nothing, as Integer.equals(null) is false.
func (o *Owner) GetPetByID(petID *int) *Pet {
	if o == nil || petID == nil {
		return nil
	}
	for _, pet := range o.pets {
		if pet == nil {
			continue
		}
		if id := pet.GetID(); id != nil && *id == *petID {
			return pet
		}
	}
	return nil
}

// Validate checks the Bean Validation constraints of the owner and of its
// embedded Person. It returns every violation joined into one error (test
// each one with errors.Is), or nil when the owner is valid.
//
// An empty telephone reports both ErrTelephoneEmpty and ErrTelephoneInvalid,
// because Bean Validation would flag both @NotEmpty and @Pattern.
func (o *Owner) Validate() error {
	if o == nil {
		return errors.Join(ErrFirstNameEmpty, ErrLastNameEmpty, ErrAddressEmpty,
			ErrCityEmpty, ErrTelephoneEmpty, ErrTelephoneInvalid)
	}
	var errs []error
	if err := o.Person.Validate(); err != nil {
		errs = append(errs, err)
	}
	if o.Address == "" {
		errs = append(errs, ErrAddressEmpty)
	}
	if o.City == "" {
		errs = append(errs, ErrCityEmpty)
	}
	if o.Telephone == "" {
		errs = append(errs, ErrTelephoneEmpty)
	}
	if !telephonePattern.MatchString(o.Telephone) {
		errs = append(errs, ErrTelephoneInvalid)
	}
	return errors.Join(errs...)
}

// String returns a debug representation listing id, new, lastName,
// firstName, address, city and telephone, in that order. Pets are left out.
//
// MIGRATION_NOTE: this approximates Spring's ToStringCreator format, but
// without the identity hash code.
func (o Owner) String() string {
	id := "[null]"
	if o.ID != nil {
		id = fmt.Sprintf("%d", *o.ID)
	}
	return fmt.Sprintf("[Owner id = %s, new = %t, lastName = '%s', firstName = '%s', address = '%s', city = '%s', telephone = '%s']",
		id, o.ID == nil, o.LastName, o.FirstName, o.Address, o.City, o.Telephone)
}
