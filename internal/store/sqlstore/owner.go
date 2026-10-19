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

// ownerColumns selects an owners row in the order scanOwner expects.
const ownerColumns = "id, first_name, last_name, address, city, telephone"

// OwnerRepository is the PostgreSQL (database/sql) implementation of
// store.OwnerRepository. It is the Go port of JdbcOwnerRepositoryImpl.
//
// MIGRATION_NOTE: @Repository/@Profile("jdbc")/@DependsOnDatabaseInitialization
// are replaced by explicit construction in main after the schema has been
// initialised. Spring's exception translation becomes wrapped Go errors.
type OwnerRepository struct {
	db *sql.DB
}

// Compile-time check that OwnerRepository satisfies the store contract.
var _ store.OwnerRepository = (*OwnerRepository)(nil)

// NewOwnerRepository returns an OwnerRepository backed by db.
func NewOwnerRepository(db *sql.DB) *OwnerRepository {
	return &OwnerRepository{db: db}
}

// FindByLastName returns all owners whose last name starts with lastName
// (case-sensitive prefix match), with pets, pet types and visits loaded.
func (r *OwnerRepository) FindByLastName(ctx context.Context, lastName string) ([]*model.Owner, error) {
	owners, err := r.queryOwners(ctx,
		"SELECT "+ownerColumns+" FROM owners WHERE last_name LIKE $1 || '%'", lastName)
	if err != nil {
		return nil, fmt.Errorf("find owners by last name: %w", err)
	}
	if err := r.loadOwnersPetsAndVisits(ctx, owners); err != nil {
		return nil, fmt.Errorf("find owners by last name: %w", err)
	}
	return owners, nil
}

// FindByLastNamePage returns one page of owners whose last name starts with
// lastName, ordered by id, together with the total number of matches.
//
// MIGRATION_NOTE: Go port of the overloaded findByLastName(String, Pageable).
func (r *OwnerRepository) FindByLastNamePage(ctx context.Context, lastName string, req store.PageRequest) (store.Page[*model.Owner], error) {
	owners, err := r.queryOwners(ctx,
		"SELECT "+ownerColumns+" FROM owners WHERE last_name LIKE $1 || '%' ORDER BY id LIMIT $2 OFFSET $3",
		lastName, req.Size, req.Offset())
	if err != nil {
		return store.Page[*model.Owner]{}, fmt.Errorf("find owners page by last name: %w", err)
	}
	if err := r.loadOwnersPetsAndVisits(ctx, owners); err != nil {
		return store.Page[*model.Owner]{}, fmt.Errorf("find owners page by last name: %w", err)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM owners WHERE last_name LIKE $1 || '%'", lastName).Scan(&total); err != nil {
		return store.Page[*model.Owner]{}, fmt.Errorf("count owners by last name: %w", err)
	}
	return store.NewPage(owners, req, total), nil
}

// FindByID returns the owner with the given id, with pets, pet types and
// visits loaded. A missing owner yields an error matching both
// store.ErrNotFound and ErrObjectRetrievalFailure.
func (r *OwnerRepository) FindByID(ctx context.Context, id int) (*model.Owner, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+ownerColumns+" FROM owners WHERE id = $1", id)
	owner, err := scanOwner(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %w", store.ErrNotFound,
			&ObjectRetrievalFailureError{EntityType: typeName[model.Owner](), ID: id})
	}
	if err != nil {
		return nil, fmt.Errorf("find owner %d: %w", id, err)
	}
	if err := r.LoadPetsAndVisits(ctx, owner); err != nil {
		return nil, fmt.Errorf("find owner %d: %w", id, err)
	}
	return owner, nil
}

// LoadPetsAndVisits populates owner's pets (ordered by pet id) with their
// types and visits.
func (r *OwnerRepository) LoadPetsAndVisits(ctx context.Context, owner *model.Owner) error {
	if owner == nil {
		return errors.New("load pets and visits: nil owner")
	}
	return r.loadOwnersPetsAndVisits(ctx, []*model.Owner{owner})
}

// Save inserts the owner when it is new (assigning the generated id) and
// updates it otherwise.
func (r *OwnerRepository) Save(ctx context.Context, owner *model.Owner) error {
	if owner == nil {
		return errors.New("save owner: nil owner")
	}
	if owner.IsNew() {
		var id int
		err := r.db.QueryRowContext(ctx,
			"INSERT INTO owners (first_name, last_name, address, city, telephone) VALUES ($1, $2, $3, $4, $5) RETURNING id",
			owner.FirstName, owner.LastName, owner.Address, owner.City, owner.Telephone).Scan(&id)
		if err != nil {
			return fmt.Errorf("insert owner: %w", err)
		}
		owner.ID = &id
		return nil
	}
	if _, err := r.db.ExecContext(ctx,
		"UPDATE owners SET first_name = $1, last_name = $2, address = $3, city = $4, telephone = $5 WHERE id = $6",
		owner.FirstName, owner.LastName, owner.Address, owner.City, owner.Telephone, *owner.ID); err != nil {
		return fmt.Errorf("update owner %d: %w", *owner.ID, err)
	}
	return nil
}

// GetPetTypes returns all pet types ordered by name.
func (r *OwnerRepository) GetPetTypes(ctx context.Context) ([]*model.PetType, error) {
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

// FindAll returns every owner with pets, pet types and visits loaded.
func (r *OwnerRepository) FindAll(ctx context.Context) ([]*model.Owner, error) {
	owners, err := r.queryOwners(ctx, "SELECT "+ownerColumns+" FROM owners")
	if err != nil {
		return nil, fmt.Errorf("find all owners: %w", err)
	}
	if err := r.loadOwnersPetsAndVisits(ctx, owners); err != nil {
		return nil, fmt.Errorf("find all owners: %w", err)
	}
	return owners, nil
}

// FindAllPage returns one page of owners ordered by id, with the total
// number of owners.
func (r *OwnerRepository) FindAllPage(ctx context.Context, req store.PageRequest) (store.Page[*model.Owner], error) {
	owners, err := r.queryOwners(ctx,
		"SELECT "+ownerColumns+" FROM owners ORDER BY id LIMIT $1 OFFSET $2", req.Size, req.Offset())
	if err != nil {
		return store.Page[*model.Owner]{}, fmt.Errorf("find owners page: %w", err)
	}
	if err := r.loadOwnersPetsAndVisits(ctx, owners); err != nil {
		return store.Page[*model.Owner]{}, fmt.Errorf("find owners page: %w", err)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM owners").Scan(&total); err != nil {
		return store.Page[*model.Owner]{}, fmt.Errorf("count owners: %w", err)
	}
	return store.NewPage(owners, req, total), nil
}

// Delete removes, in a single transaction, the visits of every pet held in
// the owner object, then those pets, then the owner itself.
//
// MIGRATION_NOTE: like the source, only pets/visits present on the in-memory
// owner are deleted; rows not loaded into the object are not touched.
func (r *OwnerRepository) Delete(ctx context.Context, owner *model.Owner) (err error) {
	if owner == nil || owner.ID == nil {
		return errors.New("delete owner: owner has no id")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete owner: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, pet := range owner.GetPets() {
		if pet == nil {
			continue
		}
		for _, visit := range pet.GetVisits() {
			if visit == nil || visit.GetID() == nil {
				continue
			}
			if _, err = tx.ExecContext(ctx, "DELETE FROM visits WHERE id = $1", *visit.GetID()); err != nil {
				return fmt.Errorf("delete visit %d: %w", *visit.GetID(), err)
			}
		}
		if pet.GetID() == nil {
			continue
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM pets WHERE id = $1", *pet.GetID()); err != nil {
			return fmt.Errorf("delete pet %d: %w", *pet.GetID(), err)
		}
		if err = deleteCascadedPetType(ctx, tx, pet); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM owners WHERE id = $1", *owner.ID); err != nil {
		return fmt.Errorf("delete owner %d: %w", *owner.ID, err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("delete owner: commit: %w", err)
	}
	return nil
}

// queryOwners runs a query selecting ownerColumns and maps every row.
func (r *OwnerRepository) queryOwners(ctx context.Context, query string, args ...any) ([]*model.Owner, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
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

// scanOwner maps a row selecting ownerColumns into a new Owner. NULL text
// columns become "" (the model uses plain strings).
func scanOwner(rs rowScanner) (*model.Owner, error) {
	var (
		id                                         int
		first, last, address, city, telephone sql.NullString
	)
	if err := rs.Scan(&id, &first, &last, &address, &city, &telephone); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("scan owner row: %w", err)
	}
	o := model.NewOwner()
	o.ID = &id
	o.FirstName = first.String
	o.LastName = last.String
	o.Address = address.String
	o.City = city.String
	o.Telephone = telephone.String
	return o, nil
}

// loadOwnersPetsAndVisits loads the pets (with visits and types) of all
// given owners using one pets/visits query and one pet-type query, instead
// of the source's per-owner N+1 queries.
func (r *OwnerRepository) loadOwnersPetsAndVisits(ctx context.Context, owners []*model.Owner) error {
	if len(owners) == 0 {
		return nil
	}
	byOwner := make(map[int]*model.Owner, len(owners))
	placeholders := make([]string, 0, len(owners))
	args := make([]any, 0, len(owners))
	for _, o := range owners {
		if o == nil || o.ID == nil {
			continue
		}
		if _, dup := byOwner[*o.ID]; dup {
			continue
		}
		byOwner[*o.ID] = o
		args = append(args, *o.ID)
		placeholders = append(placeholders, "$"+strconv.Itoa(len(args)))
	}
	if len(args) == 0 {
		return nil
	}

	query := "SELECT " + petVisitColumns +
		" FROM pets LEFT OUTER JOIN visits ON pets.id = visits.pet_id" +
		" WHERE pets.owner_id IN (" + strings.Join(placeholders, ", ") + ")" +
		" ORDER BY pets.id"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query pets and visits: %w", err)
	}
	pets, err := extractPetsWithVisits(rows)
	closeErr := rows.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("close pets and visits rows: %w", closeErr)
	}

	petTypes, err := r.GetPetTypes(ctx)
	if err != nil {
		return err
	}
	typeIndex := indexByID(petTypes)

	for _, pet := range pets {
		pt, err := lookupByID(typeIndex, pet.TypeID())
		if err != nil {
			return fmt.Errorf("resolve type of pet: %w", err)
		}
		pet.SetType(pt)
		owner, ok := byOwner[pet.OwnerIDValue()]
		if !ok {
			continue
		}
		owner.AddPet(&pet.Pet)
	}
	return nil
}
