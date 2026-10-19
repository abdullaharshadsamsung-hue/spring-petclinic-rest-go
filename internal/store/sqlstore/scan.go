package sqlstore

import (
	"database/sql"
	"fmt"

	"migrated-app/internal/model"
)

// rowScanner is the subset of *sql.Row and *sql.Rows that the row mappers in
// this file need. Accepting the interface lets one mapper serve both
// single-row (QueryRowContext) and multi-row (QueryContext) call sites, in
// the same way a Spring RowMapper serves both queryForObject and query.
type rowScanner interface {
	Scan(dest ...any) error
}

// joinedPetColumns selects a pets row for queries that join pets with other
// tables (owners, types). It keeps the source's "pets.id AS pets_id" alias
// so the pet id cannot be confused with the joined table's id. The column
// order is the one mapPetRow expects.
const joinedPetColumns = "pets.id AS pets_id, pets.name, pets.birth_date, pets.type_id, pets.owner_id"

// visitColumns selects a visits row in the column order scanVisit expects.
// It keeps the source's "id AS visit_id" alias, so a visit id cannot be
// confused with a joined pet's or owner's id.
const visitColumns = "visits.id AS visit_id, visits.visit_date, visits.description"

// jdbcPet is a Pet together with the raw foreign keys read from the pets
// row. The calling repository uses the keys to resolve the Type and Owner
// associations.
//
// MIGRATION_NOTE: Go port of the source's JdbcPet, which extended Pet with
// typeId and ownerId fields.
type jdbcPet struct {
	model.Pet
	typeID  int
	ownerID int
}

// SetTypeID records the pets.type_id foreign key read from the row.
func (p *jdbcPet) SetTypeID(id int) { p.typeID = id }

// TypeID returns the pets.type_id foreign key read from the row.
func (p *jdbcPet) TypeID() int { return p.typeID }

// SetOwnerIDValue records the pets.owner_id foreign key read from the row.
func (p *jdbcPet) SetOwnerIDValue(id int) { p.ownerID = id }

// OwnerIDValue returns the pets.owner_id foreign key read from the row.
func (p *jdbcPet) OwnerIDValue() int { return p.ownerID }

// mapPetRow maps the current row of a pets query (pets_id, name, birth_date,
// type_id, owner_id, in that order) into a new jdbcPet. It sets the id,
// name, birth date, type id and owner id. The Type and Owner associations
// stay unset, and the calling repository resolves them (see resolveType).
//
// MIGRATION_NOTE: Go port of JdbcPetRowMapper. A NULL type_id or owner_id
// maps to 0, which matches the source's rs.getInt behavior.
func mapPetRow(rs rowScanner) (*jdbcPet, error) {
	var (
		id        int
		name      sql.NullString
		birthDate model.Date
		typeID    sql.NullInt64
		ownerID   sql.NullInt64
	)
	if err := rs.Scan(&id, &name, &birthDate, &typeID, &ownerID); err != nil {
		return nil, fmt.Errorf("scan pet row: %w", err)
	}

	p := &jdbcPet{}
	p.SetID(id)
	p.SetName(name.String)
	p.SetBirthDate(birthDate)
	p.SetTypeID(int(typeID.Int64))
	p.SetOwnerIDValue(int(ownerID.Int64))
	return p, nil
}

// scanVisit maps the current row of a visits query (visit_id, visit_date,
// description, in that order; see visitColumns) into a new Visit. It sets
// the id from visit_id, the date from visit_date and the description from
// description. The pet association is left unset, and the calling repository
// attaches it when it needs to.
//
// MIGRATION_NOTE: Go port of JdbcVisitRowMapper. Spring's RowMapper reads
// columns by label. database/sql scans by position, so the column order is
// fixed by visitColumns, and every query that uses scanVisit must select
// visitColumns. rs.getObject("visit_date", LocalDate.class) becomes a scan
// into model.Date, which implements sql.Scanner. A NULL description becomes
// "", because Go strings are not nullable.
func scanVisit(rs rowScanner) (*model.Visit, error) {
	var (
		id          int
		date        model.Date
		description sql.NullString
	)
	if err := rs.Scan(&id, &date, &description); err != nil {
		return nil, fmt.Errorf("scan visit row: %w", err)
	}

	v := model.NewVisit()
	v.SetID(id)
	v.SetDate(date)
	v.SetDescription(description.String)
	return v, nil
}
