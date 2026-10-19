package sqlstore

import (
	"database/sql"
	"fmt"

	"migrated-app/internal/model"
)

// joinedPetColumns selects a pets row for queries that join pets with other
// tables (owners, types). It keeps the source's "pets.id AS pets_id" alias
// so the pet id cannot be confused with the joined table's id. The column
// order is the one mapPetRow expects.
const joinedPetColumns = "pets.id AS pets_id, pets.name, pets.birth_date, pets.type_id, pets.owner_id"

// mapPetRow maps the current row of a pets query (pets_id, name, birth_date,
// type_id, owner_id, in that order) into a new jdbcPet. It sets the id,
// name, birth date, type id and owner id. The Type and Owner associations
// stay unset, and the calling repository resolves them (see resolveType).
//
// MIGRATION_NOTE: Go port of JdbcPetRowMapper.mapRow. JDBC's getInt returns 0
// for SQL NULL and getString returns null, so the source never failed on
// NULL values. Go's Scan returns an error when NULL is scanned into an int
// or string, so nullable wrappers are used here: NULL becomes 0 or "", and
// a NULL birth_date becomes a nil *model.Date, as
// getObject(..., LocalDate.class) returned null. scanPet (scan.go) is the
// strict variant for NOT NULL rows.
// Columns are read by position, not by label. Callers must select
// joinedPetColumns, or petColumns for single-table queries.
func mapPetRow(row rowScanner) (*jdbcPet, error) {
	var (
		id      int
		name    sql.NullString
		birth   *model.Date
		typeID  sql.NullInt64
		ownerID sql.NullInt64
	)
	if err := row.Scan(&id, &name, &birth, &typeID, &ownerID); err != nil {
		return nil, fmt.Errorf("map pet row: %w", err)
	}

	p := newJdbcPet()
	p.ID = &id
	p.Name = name.String
	p.BirthDate = birth
	p.SetTypeID(int(typeID.Int64))
	p.SetOwnerIDValue(int(ownerID.Int64))
	return p, nil
}
