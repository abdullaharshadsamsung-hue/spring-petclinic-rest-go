package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

// VisitRepository is the PostgreSQL (database/sql) implementation of
// store.VisitRepository. It is the Go port of JdbcVisitRepositoryImpl.
//
// MIGRATION_NOTE: @Repository/@Profile("jdbc")/@DependsOnDatabaseInitialization
// are replaced by explicit construction in main after the schema has been
// initialised. SimpleJdbcInsert.executeAndReturnKey becomes
// INSERT ... RETURNING id. The source's JdbcVisitRowMapperExt issued three
// extra queries per visit row (pet, type, owner); here visits and pets are
// read with one join query and types/owners are resolved in bulk.
type VisitRepository struct {
	db *sql.DB
}

// Compile-time check that VisitRepository satisfies the store contract.
var _ store.VisitRepository = (*VisitRepository)(nil)

// NewVisitRepository returns a VisitRepository backed by db.
func NewVisitRepository(db *sql.DB) *VisitRepository {
	return &VisitRepository{db: db}
}

// visitPetColumns selects a visit joined with its pet, in the order
// scanVisitWithPet expects.
const visitPetColumns = visitColumns + ", " + joinedPetColumns

// FindByPetID returns every visit of the given pet, each with its pet set
// (type and owner not loaded). A nonexistent pet yields an empty slice.
//
// MIGRATION_NOTE: the source's queryForObject threw
// EmptyResultDataAccessException for an unknown pet; the store contract
// (spring-data-jpa semantics) returns an empty result instead.
func (r *VisitRepository) FindByPetID(ctx context.Context, petID int) ([]*model.Visit, error) {
	pet, err := mapPetRow(r.db.QueryRowContext(ctx,
		"SELECT "+joinedPetColumns+" FROM pets WHERE pets.id = $1", petID))
	if errors.Is(err, sql.ErrNoRows) {
		return []*model.Visit{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find visits by pet %d: load pet: %w", petID, err)
	}

	rows, err := r.db.QueryContext(ctx,
		"SELECT "+visitColumns+" FROM visits WHERE visits.pet_id = $1 ORDER BY visits.id", petID)
	if err != nil {
		return nil, fmt.Errorf("find visits by pet %d: %w", petID, err)
	}
	defer rows.Close()

	visits := make([]*model.Visit, 0)
	for rows.Next() {
		v, err := scanVisit(rows)
		if err != nil {
			return nil, fmt.Errorf("find visits by pet %d: %w", petID, err)
		}
		v.SetPet(&pet.Pet)
		visits = append(visits, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find visits by pet %d: iterate: %w", petID, err)
	}
	return visits, nil
}

// FindByID returns the visit with the given id, hydrated with its pet and
// the pet's type and owner. A missing visit yields an error wrapping
// store.ErrNotFound (the source's ObjectRetrievalFailureException).
func (r *VisitRepository) FindByID(ctx context.Context, id int) (*model.Visit, error) {
	visits, err := r.queryHydrated(ctx,
		"SELECT "+visitPetColumns+" FROM visits JOIN pets ON visits.pet_id = pets.id WHERE visits.id = $1", id)
	if err != nil {
		return nil, fmt.Errorf("find visit %d: %w", id, err)
	}
	if len(visits) == 0 {
		return nil, fmt.Errorf("%w: %w", store.ErrNotFound,
			&ObjectRetrievalFailureError{EntityType: typeName[model.Visit](), ID: id})
	}
	return visits[0], nil
}

// FindAll returns every visit ordered by id, each hydrated with its pet and
// the pet's type and owner.
func (r *VisitRepository) FindAll(ctx context.Context) ([]*model.Visit, error) {
	visits, err := r.queryHydrated(ctx,
		"SELECT "+visitPetColumns+" FROM visits JOIN pets ON visits.pet_id = pets.id ORDER BY visits.id")
	if err != nil {
		return nil, fmt.Errorf("find all visits: %w", err)
	}
	return visits, nil
}

// Save inserts the visit when it is new (setting the generated id) and
// otherwise updates visit_date, description and pet_id. A visit without a
// pet id, or whose pet does not exist, yields store.ErrIntegrity. Updating a
// visit whose row is gone yields store.ErrNotFound.
func (r *VisitRepository) Save(ctx context.Context, visit *model.Visit) error {
	if visit == nil {
		return errors.New("save visit: nil visit")
	}
	petIDPtr := visit.GetPetID()
	if petIDPtr == nil {
		return store.IntegrityViolation("save visit: pet id is required", nil)
	}
	petID := *petIDPtr

	var exists bool
	if err := r.db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM pets WHERE id = $1)", petID).Scan(&exists); err != nil {
		return fmt.Errorf("save visit: check pet %d: %w", petID, err)
	}
	if !exists {
		return store.IntegrityViolation(fmt.Sprintf("save visit: pet %d does not exist", petID), nil)
	}

	if visit.IsNew() {
		var id int
		if err := r.db.QueryRowContext(ctx,
			"INSERT INTO visits (visit_date, description, pet_id) VALUES ($1, $2, $3) RETURNING id",
			visit.GetDate(), visit.GetDescription(), petID).Scan(&id); err != nil {
			return fmt.Errorf("insert visit: %w", err)
		}
		visit.SetID(&id)
		return nil
	}

	id := *visit.GetID()
	res, err := r.db.ExecContext(ctx,
		"UPDATE visits SET visit_date = $1, description = $2, pet_id = $3 WHERE id = $4",
		visit.GetDate(), visit.GetDescription(), petID, id)
	if err != nil {
		return fmt.Errorf("update visit %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update visit %d: rows affected: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("update visit %d: %w", id, store.ErrNotFound)
	}
	return nil
}

// Delete removes the visit row with the visit's id. Deleting a row that no
// longer exists is a silent no-op.
func (r *VisitRepository) Delete(ctx context.Context, visit *model.Visit) error {
	if visit == nil || visit.GetID() == nil {
		return errors.New("delete visit: visit has no id")
	}
	id := *visit.GetID()
	if _, err := r.db.ExecContext(ctx, "DELETE FROM visits WHERE id = $1", id); err != nil {
		return fmt.Errorf("delete visit %d: %w", id, err)
	}
	return nil
}

// queryHydrated runs a query selecting visitPetColumns and returns the
// visits with their pets, and the pets' types and owners, attached.
func (r *VisitRepository) queryHydrated(ctx context.Context, query string, args ...any) ([]*model.Visit, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query visits: %w", err)
	}
	defer rows.Close()

	visits := make([]*model.Visit, 0)
	pets := make([]*jdbcPet, 0)
	for rows.Next() {
		var (
			vr visitRow
			pr petRow
		)
		dest := append(vr.dest(), pr.dest()...)
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("scan visit/pet row: %w", err)
		}
		visits = append(visits, vr.toVisit())
		pets = append(pets, pr.toPet())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate visits: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close visit rows: %w", err)
	}

	// Reuse the pet repository's bulk type/owner resolution (same package,
	// same db). Unresolvable keys fail as the source's queryForObject did.
	petRepo := &PetRepository{db: r.db}
	hydrated, err := petRepo.attachTypesAndOwners(ctx, pets)
	if err != nil {
		return nil, err
	}
	for i, v := range visits {
		v.SetPet(hydrated[i])
	}
	return visits, nil
}
