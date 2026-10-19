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

// rowIterator is the subset of *sql.Rows that extractPetsWithVisits needs to
// walk a multi-row result set.
type rowIterator interface {
	rowScanner
	Next() bool
	Err() error
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

// petVisitColumns selects the columns of a "pets LEFT OUTER JOIN visits ON
// pets.id = visits.pet_id" query in the order extractPetsWithVisits expects:
// the pet columns (joinedPetColumns), the visit's pet foreign key aliased as
// visits_pet_id, then the visit columns (visitColumns).
//
// Queries using it must ORDER BY pets.id (or otherwise keep the rows of one
// pet consecutive), because rows are grouped by consecutive runs of pets_id.
const petVisitColumns = joinedPetColumns + ", visits.pet_id AS visits_pet_id, " + visitColumns

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

// petRow holds the scan destinations for the pet columns of a row
// (pets_id, name, birth_date, type_id, owner_id; see joinedPetColumns).
type petRow struct {
	id        int
	name      sql.NullString
	birthDate *model.Date
	typeID    sql.NullInt64
	ownerID   sql.NullInt64
}

// dest returns the scan destinations in joinedPetColumns order.
func (r *petRow) dest() []any {
	return []any{&r.id, &r.name, &r.birthDate, &r.typeID, &r.ownerID}
}

// toPet builds a new jdbcPet from the scanned values.
func (r *petRow) toPet() *jdbcPet {
	id := r.id
	p := &jdbcPet{}
	p.SetID(&id)
	p.SetName(r.name.String)
	p.SetBirthDate(r.birthDate)
	p.SetTypeID(int(r.typeID.Int64))
	p.SetOwnerIDValue(int(r.ownerID.Int64))
	return p
}

// visitRow holds the scan destinations for the visit columns of a row
// (visit_id, visit_date, description; see visitColumns). Every field is
// nullable so the same holder works for the visit side of a LEFT OUTER JOIN.
type visitRow struct {
	id          sql.NullInt64
	date        *model.Date
	description sql.NullString
}

// dest returns the scan destinations in visitColumns order.
func (r *visitRow) dest() []any {
	return []any{&r.id, &r.date, &r.description}
}

// toVisit builds a new Visit from the scanned values.
func (r *visitRow) toVisit() *model.Visit {
	v := model.NewVisit()
	if r.id.Valid {
		id := int(r.id.Int64)
		v.SetID(&id)
	}
	v.SetDate(r.date)
	v.SetDescription(r.description.String)
	return v
}

// mapPetRow maps the current row of a pets query (pets_id, name, birth_date,
// type_id, owner_id, in that order) into a new jdbcPet. It sets the id,
// name, birth date, type id and owner id. The Type and Owner associations
// stay unset, and the calling repository resolves them.
//
// MIGRATION_NOTE: Go port of JdbcPetRowMapper. A NULL type_id or owner_id
// maps to 0, which matches the source's rs.getInt behavior. A NULL
// birth_date maps to a nil birth date, matching rs.getObject(..., LocalDate).
// BaseEntity.SetID takes *int (nil means "new, not yet persisted"); each
// pet gets its own copy of the id, so the pointer is never shared.
func mapPetRow(rs rowScanner) (*jdbcPet, error) {
	var r petRow
	if err := rs.Scan(r.dest()...); err != nil {
		return nil, fmt.Errorf("scan pet row: %w", err)
	}
	return r.toPet(), nil
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
// visitColumns.
//
// rs.getObject("visit_date", LocalDate.class) returns null for a NULL
// column, and the source then calls setDate(null), overriding the
// constructor's default date. To keep that behavior the date is scanned into
// a *model.Date: database/sql sets the pointer to nil for NULL. A NULL
// description becomes "", because the model uses a plain string.
func scanVisit(rs rowScanner) (*model.Visit, error) {
	var r visitRow
	if err := rs.Scan(r.dest()...); err != nil {
		return nil, fmt.Errorf("scan visit row: %w", err)
	}
	return r.toVisit(), nil
}

// extractPetsWithVisits consumes the rows of a "pets LEFT OUTER JOIN visits"
// query selecting petVisitColumns and returns one jdbcPet per run of
// consecutive rows sharing the same pets_id, each with the visits of those
// rows attached through Pet.AddVisit. A row whose visits_pet_id is NULL
// carries no visit (the pet has no visits) and only contributes the pet.
// The result is never nil; an empty result set yields an empty slice. The
// caller still owns rows and must Close it.
//
// MIGRATION_NOTE: Go port of JdbcPetVisitExtractor, which extended Spring
// Data JDBC Extensions' OneToManyResultSetExtractor with a JdbcPetRowMapper
// root mapper, a JdbcVisitRowMapper child mapper, mapPrimaryKey
// (rs.getInt("pets_id")), mapForeignKey (null when visits_pet_id is NULL)
// and addChild (root.addVisit). The template-method hooks are inlined here:
// the primary key is petRow.id, the foreign key is the nullable
// visits_pet_id, and attachment delegates to AddVisit. Like the source's
// base class, grouping is by consecutive rows, so queries must ORDER BY
// pets.id; non-consecutive rows of the same pet would produce two pets.
func extractPetsWithVisits(rows rowIterator) ([]*jdbcPet, error) {
	pets := make([]*jdbcPet, 0)
	var current *jdbcPet
	var currentKey int

	for rows.Next() {
		var (
			pr          petRow
			visitsPetID sql.NullInt64
			vr          visitRow
		)
		dest := append(pr.dest(), &visitsPetID)
		dest = append(dest, vr.dest()...)
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("scan pet/visit row: %w", err)
		}

		// mapPrimaryKey: start a new root whenever pets_id changes.
		if current == nil || pr.id != currentKey {
			current = pr.toPet()
			currentKey = pr.id
			pets = append(pets, current)
		}

		// mapForeignKey: a NULL visits_pet_id means no visit on this row.
		if !visitsPetID.Valid {
			continue
		}
		// addChild: delegate to the pet's AddVisit.
		current.AddVisit(vr.toVisit())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pet/visit rows: %w", err)
	}
	return pets, nil
}
