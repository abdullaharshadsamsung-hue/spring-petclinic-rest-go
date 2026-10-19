package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

// VetRepository is the PostgreSQL (database/sql) implementation of
// store.VetRepository. It is the Go port of JdbcVetRepositoryImpl.
//
// MIGRATION_NOTE: @Repository/@Profile("jdbc")/@DependsOnDatabaseInitialization
// are replaced by explicit construction in main after the schema has been
// initialised. SimpleJdbcInsert.executeAndReturnKey becomes
// INSERT ... RETURNING id. BeanPropertyRowMapper / BeanPropertySqlParameterSource
// become explicit Scan calls and positional $n arguments. The source issued
// N+1 queries (one per vet) to resolve specialties; here a single join query
// loads all vet/specialty pairs.
type VetRepository struct {
	db *sql.DB
}

// Compile-time check that VetRepository satisfies the store contract.
var _ store.VetRepository = (*VetRepository)(nil)

// NewVetRepository returns a VetRepository backed by db.
func NewVetRepository(db *sql.DB) *VetRepository {
	return &VetRepository{db: db}
}

// FindAll returns every vet ordered by last name then first name, each with
// its specialties populated from vet_specialties. The result is empty, not
// nil, when there are no vets.
func (r *VetRepository) FindAll(ctx context.Context) ([]*model.Vet, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, first_name, last_name FROM vets ORDER BY last_name, first_name")
	if err != nil {
		return nil, fmt.Errorf("find all vets: %w", err)
	}
	defer rows.Close()

	vets := make([]*model.Vet, 0)
	byID := make(map[int]*model.Vet)
	for rows.Next() {
		v, err := scanVet(rows)
		if err != nil {
			return nil, fmt.Errorf("find all vets: %w", err)
		}
		vets = append(vets, v)
		byID[*v.ID] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find all vets: iterate: %w", err)
	}
	if len(vets) == 0 {
		return vets, nil
	}

	specRows, err := r.db.QueryContext(ctx,
		"SELECT vs.vet_id, s.id, s.name FROM vet_specialties vs "+
			"JOIN specialties s ON s.id = vs.specialty_id ORDER BY vs.vet_id, s.id")
	if err != nil {
		return nil, fmt.Errorf("find all vets: load specialties: %w", err)
	}
	defer specRows.Close()

	for specRows.Next() {
		var (
			vetID  int
			specID int
			name   sql.NullString
		)
		if err := specRows.Scan(&vetID, &specID, &name); err != nil {
			return nil, fmt.Errorf("find all vets: scan specialty: %w", err)
		}
		v, ok := byID[vetID]
		if !ok {
			continue
		}
		v.AddSpecialty(newSpecialtyValue(specID, name.String))
	}
	if err := specRows.Err(); err != nil {
		return nil, fmt.Errorf("find all vets: iterate specialties: %w", err)
	}
	return vets, nil
}

// FindByID returns the vet with the given id and its specialties. A missing
// row yields an error matching both store.ErrNotFound and
// ErrObjectRetrievalFailure.
func (r *VetRepository) FindByID(ctx context.Context, id int) (*model.Vet, error) {
	v, err := scanVet(r.db.QueryRowContext(ctx,
		"SELECT id, first_name, last_name FROM vets WHERE id = $1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %w", store.ErrNotFound,
			&ObjectRetrievalFailureError{EntityType: typeName[model.Vet](), ID: id})
	}
	if err != nil {
		return nil, fmt.Errorf("find vet %d: %w", id, err)
	}

	rows, err := r.db.QueryContext(ctx,
		"SELECT s.id, s.name FROM vet_specialties vs "+
			"JOIN specialties s ON s.id = vs.specialty_id WHERE vs.vet_id = $1 ORDER BY s.id", id)
	if err != nil {
		return nil, fmt.Errorf("find vet %d: load specialties: %w", id, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			specID int
			name   sql.NullString
		)
		if err := rows.Scan(&specID, &name); err != nil {
			return nil, fmt.Errorf("find vet %d: scan specialty: %w", id, err)
		}
		v.AddSpecialty(newSpecialtyValue(specID, name.String))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find vet %d: iterate specialties: %w", id, err)
	}
	return v, nil
}

// Save inserts the vet when it is new (setting the generated id on it),
// otherwise updates its names. In both cases the vet's vet_specialties rows
// are rewritten to match vet.GetSpecialties(), all in one transaction.
// Specialties without an id are skipped; a specialty id that does not exist
// yields an error wrapping store.ErrIntegrity.
func (r *VetRepository) Save(ctx context.Context, vet *model.Vet) (err error) {
	if vet == nil {
		return errors.New("save vet: nil vet")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("save vet: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var newID *int
	if vet.ID == nil {
		var id int
		if err = tx.QueryRowContext(ctx,
			"INSERT INTO vets (first_name, last_name) VALUES ($1, $2) RETURNING id",
			vet.FirstName, vet.LastName).Scan(&id); err != nil {
			return fmt.Errorf("insert vet: %w", err)
		}
		newID = &id
	} else {
		if _, err = tx.ExecContext(ctx,
			"UPDATE vets SET first_name = $1, last_name = $2 WHERE id = $3",
			vet.FirstName, vet.LastName, *vet.ID); err != nil {
			return fmt.Errorf("update vet %d: %w", *vet.ID, err)
		}
	}

	vetID := 0
	if newID != nil {
		vetID = *newID
	} else {
		vetID = *vet.ID
	}
	if err = updateVetSpecialties(ctx, tx, vetID, vet.GetSpecialties()); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("save vet %d: commit: %w", vetID, err)
	}
	// Only publish the generated id once the insert is durable.
	if newID != nil {
		vet.SetID(newID)
	}
	return nil
}

// Delete removes the vet's vet_specialties rows and then the vet itself, in
// a single transaction. The specialties themselves are not deleted.
func (r *VetRepository) Delete(ctx context.Context, vet *model.Vet) (err error) {
	if vet == nil || vet.ID == nil {
		return errors.New("delete vet: vet has no id")
	}
	id := *vet.ID
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete vet %d: begin tx: %w", id, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, "DELETE FROM vet_specialties WHERE vet_id = $1", id); err != nil {
		return fmt.Errorf("delete vet %d: delete vet_specialties: %w", id, err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM vets WHERE id = $1", id); err != nil {
		return fmt.Errorf("delete vet %d: %w", id, err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("delete vet %d: commit: %w", id, err)
	}
	return nil
}

// updateVetSpecialties replaces every vet_specialties row of vetID with one
// row per persisted specialty.
//
// MIGRATION_NOTE: the source relied on the FK constraint to reject unknown
// specialty ids (surfacing as DataIntegrityViolationException). Since driver
// error codes are not inspected here, existence is checked explicitly inside
// the same transaction and reported as store.ErrIntegrity.
func updateVetSpecialties(ctx context.Context, tx *sql.Tx, vetID int, specialties []model.Specialty) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM vet_specialties WHERE vet_id = $1", vetID); err != nil {
		return fmt.Errorf("save vet %d: clear vet_specialties: %w", vetID, err)
	}
	inserted := make(map[int]struct{}, len(specialties))
	for _, s := range specialties {
		if s.ID == nil || *s.ID == 0 {
			continue
		}
		specID := *s.ID
		if _, dup := inserted[specID]; dup {
			continue
		}
		var exists bool
		if err := tx.QueryRowContext(ctx,
			"SELECT EXISTS (SELECT 1 FROM specialties WHERE id = $1)", specID).Scan(&exists); err != nil {
			return fmt.Errorf("save vet %d: check specialty %d: %w", vetID, specID, err)
		}
		if !exists {
			return store.IntegrityViolation(
				fmt.Sprintf("save vet %d: specialty %d does not exist", vetID, specID), nil)
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO vet_specialties (vet_id, specialty_id) VALUES ($1, $2)", vetID, specID); err != nil {
			return fmt.Errorf("save vet %d: insert specialty %d: %w", vetID, specID, err)
		}
		inserted[specID] = struct{}{}
	}
	return nil
}

// scanVet maps an (id, first_name, last_name) row to a Vet without
// specialties.
func scanVet(s rowScanner) (*model.Vet, error) {
	var (
		id        int
		firstName sql.NullString
		lastName  sql.NullString
	)
	if err := s.Scan(&id, &firstName, &lastName); err != nil {
		return nil, err
	}
	v := model.NewVet()
	v.SetID(&id)
	v.FirstName = firstName.String
	v.LastName = lastName.String
	return v, nil
}

// newSpecialtyValue builds a persisted Specialty value with its own id copy.
func newSpecialtyValue(id int, name string) model.Specialty {
	sp := model.NewSpecialty()
	sp.SetID(&id)
	sp.SetName(name)
	return *sp
}
