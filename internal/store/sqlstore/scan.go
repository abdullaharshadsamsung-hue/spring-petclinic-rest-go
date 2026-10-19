package sqlstore

import (
	"fmt"

	"migrated-app/internal/model"
)

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// petColumns is the column list scanPet expects, in order. Repositories
// select it from model.PetsTable, e.g.
// "SELECT " + petColumns + " FROM pets WHERE id = $1".
const petColumns = "id, name, birth_date, type_id, owner_id"

// jdbcPet is a Pet together with the raw type_id and owner_id foreign keys
// read from a pets row, before they are resolved into object references.
//
// MIGRATION_NOTE: Go port of JdbcPet (a Pet subclass with typeId/ownerId).
// Inheritance becomes embedding of *model.Pet; the type is unexported
// because it only exists inside the SQL layer during two-phase hydration.
type jdbcPet struct {
	*model.Pet
	typeID  int
	ownerID int
}

// newJdbcPet returns an empty jdbcPet wrapping a fresh Pet.
func newJdbcPet() *jdbcPet {
	return &jdbcPet{Pet: &model.Pet{}}
}

// TypeID returns the transient pet type foreign key.
func (p *jdbcPet) TypeID() int { return p.typeID }

// SetTypeID stores the transient pet type foreign key.
func (p *jdbcPet) SetTypeID(typeID int) { p.typeID = typeID }

// OwnerIDValue returns the transient owner foreign key.
// (Named to avoid clashing with the promoted model.Pet.OwnerID field.)
func (p *jdbcPet) OwnerIDValue() int { return p.ownerID }

// SetOwnerIDValue stores the transient owner foreign key.
func (p *jdbcPet) SetOwnerIDValue(ownerID int) { p.ownerID = ownerID }

// scanPet reads one row with the columns in petColumns into a jdbcPet.
// The pet type is not resolved yet; call resolveType afterwards.
func scanPet(row rowScanner) (*jdbcPet, error) {
	var (
		id      int
		name    string
		birth   *model.Date
		typeID  int
		ownerID int
	)
	if err := row.Scan(&id, &name, &birth, &typeID, &ownerID); err != nil {
		return nil, fmt.Errorf("scan pet: %w", err)
	}
	p := newJdbcPet()
	p.ID = &id
	p.Name = name
	p.BirthDate = birth
	p.SetTypeID(typeID)
	p.SetOwnerIDValue(ownerID)
	owner := ownerID
	p.Pet.OwnerID = &owner
	return p, nil
}

// resolveType replaces the raw type_id with the matching PetType from types
// (built with indexByID). It returns an *ObjectRetrievalFailureError if the
// type id is unknown, as EntityUtils.getById did in the JDBC repository.
func (p *jdbcPet) resolveType(types map[int]*model.PetType) error {
	t, err := lookupByID(types, p.typeID)
	if err != nil {
		return fmt.Errorf("resolve type of pet: %w", err)
	}
	p.Type = t
	return nil
}
