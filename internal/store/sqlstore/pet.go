package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

// PetRepository is the PostgreSQL (database/sql) implementation of
// store.PetRepository. It is the Go port of JdbcPetRepositoryImpl.
//
// MIGRATION_NOTE: @Repository/@Profile("jdbc")/@DependsOnDatabaseInitialization
// are replaced by explicit construction in main after the schema has been
// initialised. SimpleJdbcInsert.executeAndReturnKey becomes
// INSERT ... RETURNING id. Spring's DataAccessException translation becomes
// wrapped Go errors.
type PetRepository struct {
	db     *sql.DB
	owners store.OwnerRepository
}

// Compile-time check that PetRepository satisfies the store contract.
var _ store.PetRepository = (*PetRepository)(nil)

// NewPetRepository returns a PetRepository backed by db. owners is used by
// FindByID to hydrate the pet's owner graph, as in the source.
func NewPetRepository(db *sql.DB, owners store.OwnerRepository) *PetRepository {
	return &PetRepository{db: db, owners: owners}
}

// FindPetTypes returns all pet types ordered by name ascending.
func (r *PetRepository) FindPetTypes(ctx context.Context) ([]*model.PetType, error) {
	types, err := r.queryPetTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("find pet types: %w", err)
	}
	return types, nil
}

// FindByID resolves the pet's owner_id, loads that owner (with pets, types
// and visits) through the owner repository, and returns the matching pet
// from the owner's pets. A missing pet yields an error matching both
// store.ErrNotFound and ErrObjectRetrievalFailure.
func (r *PetRepository) FindByID(ctx context.Context, id int) (*model.Pet, error) {
	var ownerID sql.NullInt64
	err := r.db.QueryRowContext(ctx, "SELECT owner_id FROM pets WHERE id = $1", id).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %w", store.ErrNotFound,
			&ObjectRetrievalFailureError{EntityType: typeName[model.Pet](), ID: id})
	}
	if err != nil {
		return nil, fmt.Errorf("find pet %d: %w", id, err)
	}
	if !ownerID.Valid {
		// MIGRATION_NOTE: the source would fail unboxing a NULL owner_id;
		// a pet without owner cannot be found through the owner graph.
		return nil, fmt.Errorf("%w: %w", store.ErrNotFound,
			&ObjectRetrievalFailureError{EntityType: typeName[model.Pet](), ID: id})
	}
	owner, err := r.owners.FindByID(ctx, int(ownerID.Int64))
	if err != nil {
		return nil, fmt.Errorf("find pet %d: load owner: %w", id, err)
	}
	pet, err := byID(owner.GetPets(), id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", store.ErrNotFound, err)
	}
	return pet, nil
}

// Save inserts the pet when it is new (setting the generated id on it) and
// updates name, birth_date, type_id and owner_id otherwise.
func (r *PetRepository) Save(ctx context.Context, pet *model.Pet) error {
	if pet == nil {
		return errors.New("save pet: nil pet")
	}
	if pet.GetType() == nil || pet.GetType().GetID() == nil {
		return store.IntegrityViolation("save pet: pet type with id is required", nil)
	}
	ownerID := pet.GetOwnerID()
	if ownerID == nil {
		return store.IntegrityViolation("save pet: owner id is required", nil)
	}
	typeID := *pet.GetType().GetID()

	if pet.IsNew() {
		var id int
		err := r.db.QueryRowContext(ctx,
			"INSERT INTO pets (name, birth_date, type_id, owner_id) VALUES ($1, $2, $3, $4) RETURNING id",
			pet.GetName(), pet.GetBirthDate(), typeID, *ownerID).Scan(&id)
		if err != nil {
			return fmt.Errorf("insert pet: %w", err)
		}
		pet.SetID(&id)
		return nil
	}
	id := *pet.GetID()
	if _, err := r.db.ExecContext(ctx,
		"UPDATE pets SET name = $1, birth_date = $2, type_id = $3, owner_id = $4 WHERE id = $5",
		pet.GetName(), pet.GetBirthDate(), typeID, *ownerID, id); err != nil {
		return fmt.Errorf("update pet %d: %w", id, err)
	}
	return nil
}

// FindAll returns every pet with its type and owner (basic owner fields,
// without the owner's pets) attached. Visits are not loaded, matching the
// source JDBC implementation.
func (r *PetRepository) FindAll(ctx context.Context) ([]*model.Pet, error) {
	pets, err := r.queryPets(ctx, "SELECT "+joinedPetColumns+" FROM pets ORDER BY pets.id")
	if err != nil {
		return nil, fmt.Errorf("find all pets: %w", err)
	}
	result, err := r.attachTypesAndOwners(ctx, pets)
	if err != nil {
		return nil, fmt.Errorf("find all pets: %w", err)
	}
	if err := r.loadVisits(ctx, result); err != nil {
		return nil, fmt.Errorf("find all pets: %w", err)
	}
	return result, nil
}

// FindAllPage returns one page of pets ordered by id, with type and owner
// attached, together with the total pet count.
func (r *PetRepository) FindAllPage(ctx context.Context, req store.PageRequest) (store.Page[*model.Pet], error) {
	pets, err := r.queryPets(ctx,
		"SELECT "+joinedPetColumns+" FROM pets ORDER BY pets.id LIMIT $1 OFFSET $2",
		req.Size, req.Offset())
	if err != nil {
		return store.Page[*model.Pet]{}, fmt.Errorf("find pets page: %w", err)
	}
	result, err := r.attachTypesAndOwners(ctx, pets)
	if err != nil {
		return store.Page[*model.Pet]{}, fmt.Errorf("find pets page: %w", err)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pets").Scan(&total); err != nil {
		return store.Page[*model.Pet]{}, fmt.Errorf("count pets: %w", err)
	}
	return store.NewPage(result, req, total), nil
}

// Delete removes, in one transaction, each visit held by the pet (by visit
// id, a manual cascade) and then the pet row itself.
//
// MIGRATION_NOTE: like the source, only visits present on the in-memory pet
// are deleted. Running both steps in one transaction is a deliberate
// improvement; the source relied on a service-level @Transactional.
func (r *PetRepository) Delete(ctx context.Context, pet *model.Pet) (err error) {
	if pet == nil || pet.GetID() == nil {
		return errors.New("delete pet: pet has no id")
	}
	petID := *pet.GetID()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete pet %d: begin tx: %w", petID, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, visit := range pet.GetVisits() {
		if visit == nil || visit.GetID() == nil {
			continue
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM visits WHERE id = $1", *visit.GetID()); err != nil {
			return fmt.Errorf("delete visit %d: %w", *visit.GetID(), err)
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM pets WHERE id = $1", petID); err != nil {
		return fmt.Errorf("delete pet %d: %w", petID, err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("delete pet %d: commit: %w", petID, err)
	}
	return nil
}

// queryPets runs a query selecting joinedPetColumns and maps every row.
func (r *PetRepository) queryPets(ctx context.Context, query string, args ...any) ([]*jdbcPet, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pets: %w", err)
	}
	defer rows.Close()

	pets := make([]*jdbcPet, 0)
	for rows.Next() {
		p, err := mapPetRow(rows)
		if err != nil {
			return nil, err
		}
		pets = append(pets, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pets: %w", err)
	}
	return pets, nil
}

// queryPetTypes returns all pet types ordered by name.
func (r *PetRepository) queryPetTypes(ctx context.Context) ([]*model.PetType, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name FROM types ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("query pet types: %w", err)
	}
	defer rows.Close()

	types := make([]*model.PetType, 0)
	for rows.Next() {
		var (
			id   int
			name sql.NullString
		)
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan pet type: %w", err)
		}
		pt := &model.PetType{}
		pt.SetID(&id)
		pt.SetName(name.String)
		types = append(types, pt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pet types: %w", err)
	}
	return types, nil
}

// queryBasicOwners returns every owner with basic fields only (no pets),
// ordered by last name as in the source.
func (r *PetRepository) queryBasicOwners(ctx context.Context) ([]*model.Owner, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+ownerColumns+" FROM owners ORDER BY last_name")
	if err != nil {
		return nil, fmt.Errorf("query owners: %w", err)
	}
	defer rows.Close()

	owners := make([]*model.Owner, 0)
	for rows.Next() {
		o, err := scanOwner(rows)
		if err != nil {
			return nil, err
		}
		owners = append(owners, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate owners: %w", err)
	}
	return owners, nil
}

// attachTypesAndOwners resolves each pet's type and owner from its raw
// foreign keys. An unresolvable key is an ObjectRetrievalFailureError, as
// EntityUtils.getById threw in the source.
func (r *PetRepository) attachTypesAndOwners(ctx context.Context, pets []*jdbcPet) ([]*model.Pet, error) {
	result := make([]*model.Pet, 0, len(pets))
	if len(pets) == 0 {
		return result, nil
	}
	types, err := r.queryPetTypes(ctx)
	if err != nil {
		return nil, err
	}
	owners, err := r.queryBasicOwners(ctx)
	if err != nil {
		return nil, err
	}
	typeIndex := indexByID(types)
	ownerIndex := indexByID(owners)

	for _, jp := range pets {
		pt, err := lookupByID(typeIndex, jp.TypeID())
		if err != nil {
			return nil, fmt.Errorf("resolve type of pet: %w", err)
		}
		owner, err := lookupByID(ownerIndex, jp.OwnerIDValue())
		if err != nil {
			return nil, fmt.Errorf("resolve owner of pet: %w", err)
		}
		jp.SetType(pt)
		jp.SetOwner(owner)
		result = append(result, &jp.Pet)
	}
	return result, nil
}
