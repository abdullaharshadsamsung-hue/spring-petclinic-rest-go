package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

// PetTypeRepository is the PostgreSQL (database/sql) implementation of
// store.PetTypeRepository. It is the Go port of JdbcPetTypeRepositoryImpl.
//
// MIGRATION_NOTE: @Repository/@Profile("jdbc")/@DependsOnDatabaseInitialization
// are replaced by explicit construction in main after the schema has been
// initialised. SimpleJdbcInsert.executeAndReturnKey becomes
// INSERT ... RETURNING id. BeanPropertyRowMapper becomes explicit Scan calls.
type PetTypeRepository struct {
	db *sql.DB
}

// Compile-time check that PetTypeRepository satisfies the store contract.
var _ store.PetTypeRepository = (*PetTypeRepository)(nil)

// NewPetTypeRepository returns a PetTypeRepository backed by db.
func NewPetTypeRepository(db *sql.DB) *PetTypeRepository {
	return &PetTypeRepository{db: db}
}

// FindByID returns the pet type with the given id. A missing row yields an
// error matching both store.ErrNotFound and ErrObjectRetrievalFailure.
func (r *PetTypeRepository) FindByID(ctx context.Context, id int) (*model.PetType, error) {
	pt, err := scanPetType(r.db.QueryRowContext(ctx, "SELECT id, name FROM types WHERE id = $1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %w", store.ErrNotFound,
			&ObjectRetrievalFailureError{EntityType: typeName[model.PetType](), ID: id})
	}
	if err != nil {
		return nil, fmt.Errorf("find pet type %d: %w", id, err)
	}
	return pt, nil
}

// FindByName returns the pet type whose name exactly matches name. A missing
// row yields an error matching store.ErrNotFound and ErrObjectRetrievalFailure.
func (r *PetTypeRepository) FindByName(ctx context.Context, name string) (*model.PetType, error) {
	pt, err := scanPetType(r.db.QueryRowContext(ctx, "SELECT id, name FROM types WHERE name = $1", name))
	if errors.Is(err, sql.ErrNoRows) {
		// MIGRATION_NOTE: ObjectRetrievalFailureError carries an int id, so a
		// name-keyed miss uses a formatted message with the same Spring wording.
		return nil, fmt.Errorf("%w: %w: Object of class [%s] with identifier [%s]: not found",
			store.ErrNotFound, ErrObjectRetrievalFailure, typeName[model.PetType](), name)
	}
	if err != nil {
		return nil, fmt.Errorf("find pet type by name %q: %w", name, err)
	}
	return pt, nil
}

// FindAll returns every pet type ordered by id. The result is empty, not nil,
// when none exist.
//
// MIGRATION_NOTE: the source had no ORDER BY; ordering by id makes the
// result deterministic (agreed plan, matches default insertion order).
func (r *PetTypeRepository) FindAll(ctx context.Context) ([]*model.PetType, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name FROM types ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("find all pet types: %w", err)
	}
	defer rows.Close()

	types := make([]*model.PetType, 0)
	for rows.Next() {
		pt, err := scanPetType(rows)
		if err != nil {
			return nil, fmt.Errorf("find all pet types: %w", err)
		}
		types = append(types, pt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find all pet types: iterate: %w", err)
	}
	return types, nil
}

// Save inserts the pet type when it is new (setting the generated id on it),
// otherwise updates its name.
func (r *PetTypeRepository) Save(ctx context.Context, petType *model.PetType) error {
	if petType == nil {
		return errors.New("save pet type: nil pet type")
	}
	if petType.IsNew() {
		var id int
		if err := r.db.QueryRowContext(ctx,
			"INSERT INTO types (name) VALUES ($1) RETURNING id", petType.GetName()).Scan(&id); err != nil {
			return fmt.Errorf("insert pet type: %w", err)
		}
		petType.SetID(&id)
		return nil
	}
	id := *petType.GetID()
	if _, err := r.db.ExecContext(ctx,
		"UPDATE types SET name = $1 WHERE id = $2", petType.GetName(), id); err != nil {
		return fmt.Errorf("update pet type %d: %w", id, err)
	}
	return nil
}

// Delete removes the pet type, cascading manually to every pet of that type
// and every visit of those pets, all in one transaction.
//
// MIGRATION_NOTE: the source loaded each pet and visit and deleted them one
// row at a time; set-based DELETEs give the same end state. Running them in
// one transaction is a deliberate improvement (the source relied on a
// service-level @Transactional).
func (r *PetTypeRepository) Delete(ctx context.Context, petType *model.PetType) (err error) {
	if petType == nil || petType.GetID() == nil {
		return errors.New("delete pet type: pet type has no id")
	}
	id := *petType.GetID()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete pet type %d: begin tx: %w", id, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx,
		"DELETE FROM visits WHERE pet_id IN (SELECT id FROM pets WHERE type_id = $1)", id); err != nil {
		return fmt.Errorf("delete pet type %d: delete visits: %w", id, err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM pets WHERE type_id = $1", id); err != nil {
		return fmt.Errorf("delete pet type %d: delete pets: %w", id, err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM types WHERE id = $1", id); err != nil {
		return fmt.Errorf("delete pet type %d: %w", id, err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("delete pet type %d: commit: %w", id, err)
	}
	return nil
}

// rowScanner is satisfied by *sql.Row and *sql.Rows.
type petTypeRowScanner interface {
	Scan(dest ...any) error
}

// scanPetType maps an (id, name) row to a PetType.
func scanPetType(s petTypeRowScanner) (*model.PetType, error) {
	var (
		id   int
		name sql.NullString
	)
	if err := s.Scan(&id, &name); err != nil {
		return nil, err
	}
	pt := model.NewPetType()
	pt.SetID(&id)
	pt.SetName(name.String)
	return pt, nil
}
