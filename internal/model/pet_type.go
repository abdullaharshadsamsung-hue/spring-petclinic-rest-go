package model

// PetTypesTable is the name of the database table that stores PetType rows.
// The source maps the class with @Table(name = "types"), so the table name is
// not derived from the type name.
const PetTypesTable = "types"

// PetType is a category of pet, such as Cat, Dog or Hamster. It is the
// reference (lookup) data that Pet.Type points to.
//
// It gets its auto-generated integer id (BaseEntity) and its name
// (NamedEntity) by embedding. GetID/SetID, GetName/SetName, String, IsNew
// and Validate are promoted from those types.
//
// MIGRATION_NOTE: The JPA @Entity/@Table mapping becomes plain struct
// embedding. The "types" table (id SERIAL/IDENTITY primary key, name
// VARCHAR) is created by the embedded schema scripts
// (src/main/resources/db/postgres/schema.sql), which run at startup. This
// type owns no DDL of its own.
type PetType struct {
	NamedEntity
}

// NewPetType returns a new, unpersisted PetType with no id and an empty name,
// the equivalent of the source's implicit no-arg constructor.
func NewPetType() *PetType {
	return &PetType{}
}
