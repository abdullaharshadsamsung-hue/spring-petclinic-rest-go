package model

// SpecialtiesTable is the name of the database table that stores Specialty
// rows, as mapped by the source's @Table(name = "specialties").
const SpecialtiesTable = "specialties"

// Specialty is a veterinarian's specialty, for example radiology, surgery or
// dentistry.
//
// It gets its auto-generated integer id (BaseEntity) and its name
// (NamedEntity) by embedding. GetID/SetID, GetName/SetName, String, IsNew
// and Validate are promoted from those types.
//
// MIGRATION_NOTE: The JPA @Entity/@Table mapping becomes plain struct
// embedding. The "specialties" table (id SERIAL/IDENTITY primary key, name
// VARCHAR) and the "vet_specialties" join table are created by the embedded
// schema scripts (src/main/resources/db/postgres/schema.sql) that run at
// startup. This type owns no DDL of its own.
//
// MIGRATION_NOTE: The Vet<->Specialty many-to-many is unidirectional and
// owned by Vet, so Specialty has no back-reference to its vets.
type Specialty struct {
	NamedEntity
}

// NewSpecialty returns a new, unpersisted Specialty with no id and an empty
// name. It replaces the source's implicit no-arg constructor.
func NewSpecialty() *Specialty {
	return &Specialty{}
}
