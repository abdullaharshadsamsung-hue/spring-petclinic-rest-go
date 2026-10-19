package model

import (
	"encoding/json"
	"sort"
	"strings"
)

// VetsTable is the name of the database table that stores Vet rows, as
// mapped by the source's @Table(name = "vets").
const VetsTable = "vets"

// VetSpecialtiesTable is the join table for the Vet<->Specialty many-to-many
// association (columns vet_id, specialty_id), as mapped by the source's
// @JoinTable(name = "vet_specialties").
const VetSpecialtiesTable = "vet_specialties"

// Vet is a veterinarian, stored in the "vets" table (id, first_name,
// last_name). Id and names come from the embedded Person/BaseEntity.
//
// MIGRATION_NOTE: The JPA @ManyToMany(fetch = EAGER) association becomes an
// unexported slice with set semantics. Repositories must load it eagerly
// (join on vet_specialties) and persist it themselves. The tables are created
// by the embedded schema scripts that run at startup, so this type owns no DDL.
type Vet struct {
	Person
	specialties []Specialty
}

// NewVet returns a new, unpersisted Vet with no specialties.
func NewVet() *Vet {
	return &Vet{}
}

// sameSpecialty mirrors the source's set membership. Persisted specialties
// (non-nil ID) are equal when their IDs match.
//
// MIGRATION_NOTE: The Java entity uses identity equality (no
// equals/hashCode override). Go values have no identity, so unpersisted
// specialties (nil ID) are never treated as duplicates.
func sameSpecialty(a, b *Specialty) bool {
	if a.ID == nil || b.ID == nil {
		return false
	}
	return *a.ID == *b.ID
}

func (v *Vet) contains(s *Specialty) bool {
	for i := range v.specialties {
		if sameSpecialty(&v.specialties[i], s) {
			return true
		}
	}
	return false
}

// GetSpecialties returns a copy of the vet's specialties, sorted
// case-insensitively by name. Changing the returned slice does not change the
// vet, so it is read-only in effect. This list is serialized as
// "specialties".
func (v *Vet) GetSpecialties() []Specialty {
	if v == nil {
		return []Specialty{}
	}
	sorted := make([]Specialty, len(v.specialties))
	copy(sorted, v.specialties)
	sort.SliceStable(sorted, func(i, j int) bool {
		return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
	})
	return sorted
}

// SetSpecialties replaces the vet's specialties with a new set built from the
// given list. Duplicates (same persisted ID) are dropped.
func (v *Vet) SetSpecialties(specialties []Specialty) {
	v.specialties = make([]Specialty, 0, len(specialties))
	for i := range specialties {
		v.AddSpecialty(specialties[i])
	}
}

// NrOfSpecialties returns the number of distinct specialties. It is not part
// of the JSON output.
func (v *Vet) NrOfSpecialties() int {
	if v == nil {
		return 0
	}
	return len(v.specialties)
}

// AddSpecialty adds a specialty to the vet's set of specialties. It does
// nothing if a specialty with the same persisted ID is already there.
func (v *Vet) AddSpecialty(specialty Specialty) {
	if v.contains(&specialty) {
		return
	}
	v.specialties = append(v.specialties, specialty)
}

// ClearSpecialties removes all specialties from the vet.
func (v *Vet) ClearSpecialties() {
	v.specialties = v.specialties[:0]
}

// vetJSON is the wire shape of a Vet.
type vetJSON struct {
	ID          *int        `json:"id"`
	FirstName   string      `json:"firstName"`
	LastName    string      `json:"lastName"`
	Specialties []Specialty `json:"specialties"`
}

// MarshalJSON encodes the vet with its sorted specialties under
// "specialties". The specialty count is left out, as @JsonIgnore does in the
// source.
func (v Vet) MarshalJSON() ([]byte, error) {
	return json.Marshal(vetJSON{
		ID:          v.ID,
		FirstName:   v.FirstName,
		LastName:    v.LastName,
		Specialties: v.GetSpecialties(),
	})
}

// UnmarshalJSON decodes a vet, including its "specialties" list.
func (v *Vet) UnmarshalJSON(data []byte) error {
	var in vetJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	v.ID = in.ID
	v.FirstName = in.FirstName
	v.LastName = in.LastName
	v.SetSpecialties(in.Specialties)
	return nil
}
