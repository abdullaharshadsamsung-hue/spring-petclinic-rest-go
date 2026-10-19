package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

// SpecialtyRepository is the PostgreSQL (database/sql) implementation of
// store.SpecialtyRepository. It is the Go port of JdbcSpecialtyRepositoryImpl.
//
// MIGRATION_NOTE: @Repository/@Profile("jdbc")/@DependsOnDatabaseInitialization
// are replaced by explicit construction in main after the schema has been
// initialised. SimpleJdbcInsert.executeAndReturnKey becomes
// INSERT ... RETURNING id. BeanPropertyRowMapper becomes explicit Scan calls.
type SpecialtyRepository struct {
	db *sql.DB
}

// Compile-time check that SpecialtyRepository satisfies the store contract.
var _ store.SpecialtyRepository = (*SpecialtyRepository)(nil)

// NewSpecialtyRepository returns a SpecialtyRepository backed by db.
func NewSpecialtyRepository(db *sql.DB) *SpecialtyRepository {
	return &SpecialtyRepository{db: db}
}

// FindByID returns the specialty with the given id. A missing row yields an
// error matching both store.ErrNotFound and ErrObjectRetrievalFailure.
func (r *SpecialtyRepository) FindByID(ctx context.Context, id int) (*model.Specialty, error) {
	s, err := scanSpecialty(r.db.QueryRowContext(ctx,
		"SELECT id, name FROM specialties WHERE id = $1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %w", store.ErrNotFound,
			&ObjectRetrievalFailureError{EntityType: typeName[model.Specialty](), ID: id})
	}
	if err != nil {
		return nil, fmt.Errorf("find specialty %d: %w", id, err)
	}
	return s, nil
}

// FindByNameIn returns every specialty whose name exactly matches one of
// names. Unknown names are silently ignored. An empty input returns an empty
// slice without querying the database.
//
// MIGRATION_NOTE: NamedParameterJdbcTemplate's IN-list expansion is replaced
// by a dynamically built "IN ($1, $2, ...)" list. Names are de-duplicated
// first to keep the source's Set semantics. The source's
// EmptyResultDataAccessException catch was dead code (query() never throws
// it), so no-match returns an empty slice, not an error.
func (r *SpecialtyRepository) FindByNameIn(ctx context.Context, names []string) ([]*model.Specialty, error) {
	specialties := make([]*model.Specialty, 0)
	if len(names) == 0 {
		return specialties, nil
	}

	seen := make(map[string]struct{}, len(names))
	args := make([]any, 0, len(names))
	placeholders := make([]string, 0, len(names))
	for _, n := range names {
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		args = append(args, n)
		placeholders = append(placeholders, "$"+strconv.Itoa(len(args)))
	}

	query := "SELECT id, name FROM specialties WHERE specialties.name IN (" +
		strings.Join(placeholders, ", ") + ") ORDER BY id"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find specialties by name: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		s, err := scanSpecialty(rows)
		if err != nil {
			return nil, fmt.Errorf("find specialties by name: %w", err)
		}
		specialties = append(specialties, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find specialties by name: iterate: %w", err)
	}
	return specialties, nil
}

// FindAll returns every specialty ordered by id. The result is empty, not
// nil, when none exist.
//
// MIGRATION_NOTE: the source had no ORDER BY; ordering by id makes the result
// deterministic (consistent with the other sqlstore repositories).
func (r *SpecialtyRepository) FindAll(ctx context.Context) ([]*model.Specialty, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name FROM specialties ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("find all specialties: %w", err)
	}
	defer rows.Close()

	specialties := make([]*model.Specialty, 0)
	for rows.Next() {
		s, err := scanSpecialty(rows)
		if err != nil {
			return nil, fmt.Errorf("find all specialties: %w", err)
		}
		specialties = append(specialties, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find all specialties: iterate: %w", err)
	}
	return specialties, nil
}

// Save inserts the specialty when it is new (setting the generated id on it),
// otherwise updates its name.
func (r *SpecialtyRepository) Save(ctx context.Context, specialty *model.Specialty) error {
	if specialty == nil {
		return errors.New("save specialty: nil specialty")
	}
	if specialty.IsNew() {
		var id int
		if err := r.db.QueryRowContext(ctx,
			"INSERT INTO specialties (name) VALUES ($1) RETURNING id",
			specialty.GetName()).Scan(&id); err != nil {
			return fmt.Errorf("insert specialty: %w", err)
		}
		specialty.SetID(&id)
		return nil
	}
	id := *specialty.GetID()
	if _, err := r.db.ExecContext(ctx,
		"UPDATE specialties SET name = $1 WHERE id = $2", specialty.GetName(), id); err != nil {
		return fmt.Errorf("update specialty %d: %w", id, err)
	}
	return nil
}

// Delete removes the specialty and every vet_specialties row referencing it,
// in a single transaction.
//
// MIGRATION_NOTE: the source issued two independent updates and relied on a
// service-level @Transactional; here both deletes share one transaction.
func (r *SpecialtyRepository) Delete(ctx context.Context, specialty *model.Specialty) (err error) {
	if specialty == nil || specialty.GetID() == nil {
		return errors.New("delete specialty: specialty has no id")
	}
	id := *specialty.GetID()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete specialty %d: begin tx: %w", id, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, "DELETE FROM vet_specialties WHERE specialty_id = $1", id); err != nil {
		return fmt.Errorf("delete specialty %d: delete vet_specialties: %w", id, err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM specialties WHERE id = $1", id); err != nil {
		return fmt.Errorf("delete specialty %d: %w", id, err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("delete specialty %d: commit: %w", id, err)
	}
	return nil
}

// scanSpecialty maps an (id, name) row to a Specialty.
func scanSpecialty(s rowScanner) (*model.Specialty, error) {
	var (
		id   int
		name sql.NullString
	)
	if err := s.Scan(&id, &name); err != nil {
		return nil, err
	}
	sp := model.NewSpecialty()
	sp.SetID(&id)
	sp.SetName(name.String)
	return sp, nil
}
