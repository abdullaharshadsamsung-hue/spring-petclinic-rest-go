// Package service contains the service layer of the petclinic application.
//
// Clinic is the single facade that every HTTP handler depends on for CRUD and
// query operations on pets, visits, vets, owners, pet types and specialties.
// Handlers depend on this interface rather than on a concrete type, so it can
// be replaced by a fake in tests.
//
// MIGRATION_NOTE: In the source project this was the Spring ClinicService
// interface plus its ClinicServiceImpl implementation. Both live in this file
// so that only one Clinic declaration exists in the package. The differences
// from the source are listed below.
//
//   - Spring's unchecked DataAccessException becomes an ordinary returned
//     error. Use errors.Is(err, store.ErrNotFound) to detect a missing entity
//     and errors.Is(err, store.ErrIntegrity) to detect a constraint violation.
//
//   - Find-by-id methods return an error wrapping store.ErrNotFound when the
//     entity does not exist. The source returned null instead. Every handler
//     calling FindXxxByID must check errors.Is(err, store.ErrNotFound) and
//     answer with an empty 404, which is what the source did for null.
//     This applies to the owner, pet, vet, visit, pet type and specialty
//     handlers. MANUAL REVIEW: confirm each of those call sites does this.
//
//   - FindSpecialtiesByNameIn keeps the source's "null on not-found"
//     behaviour. It returns (nil, nil) when the repository reports
//     store.ErrNotFound.
//
//   - @Transactional is replaced by an explicit Transactor. Each write runs
//     in its own transaction. Reads run without an explicit transaction, so
//     the readOnly=true hint is dropped. This does not change observable
//     behaviour.
//
//   - Save methods run the entity's Validate() method first, when it has
//     one. This mirrors Hibernate's pre-persist bean validation.
package service

import (
	"context"
	"errors"
	"fmt"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

// Clinic is the facade used by all petclinic HTTP handlers.
type Clinic interface {
	// FindAllPets returns all pets.
	FindAllPets(ctx context.Context) ([]*model.Pet, error)
	// FindPets returns one page of pets.
	FindPets(ctx context.Context, page, size int) (store.Page[*model.Pet], error)
	// DeletePet deletes a pet.
	DeletePet(ctx context.Context, pet *model.Pet) error
	// FindPetByID returns a pet with its owner, or an error wrapping store.ErrNotFound.
	FindPetByID(ctx context.Context, id int) (*model.Pet, error)
	// SavePet resolves the pet's type, then inserts or updates the pet.
	SavePet(ctx context.Context, pet *model.Pet) error
	// FindPetTypes returns all pet types via the pet repository.
	FindPetTypes(ctx context.Context) ([]*model.PetType, error)

	// FindVisitByID returns a visit, or an error wrapping store.ErrNotFound.
	FindVisitByID(ctx context.Context, id int) (*model.Visit, error)
	// FindAllVisits returns all visits.
	FindAllVisits(ctx context.Context) ([]*model.Visit, error)
	// FindVisitsByPetID returns the visits of one pet.
	FindVisitsByPetID(ctx context.Context, petID int) ([]*model.Visit, error)
	// SaveVisit inserts or updates a visit.
	SaveVisit(ctx context.Context, visit *model.Visit) error
	// DeleteVisit deletes a visit.
	DeleteVisit(ctx context.Context, visit *model.Visit) error

	// FindVetByID returns a vet, or an error wrapping store.ErrNotFound.
	FindVetByID(ctx context.Context, id int) (*model.Vet, error)
	// FindAllVets returns all vets with their specialties.
	FindAllVets(ctx context.Context) ([]*model.Vet, error)
	// FindVets returns all vets. It is identical to FindAllVets.
	FindVets(ctx context.Context) ([]*model.Vet, error)
	// SaveVet inserts or updates a vet.
	SaveVet(ctx context.Context, vet *model.Vet) error
	// DeleteVet deletes a vet.
	DeleteVet(ctx context.Context, vet *model.Vet) error

	// FindAllOwners returns all owners.
	FindAllOwners(ctx context.Context) ([]*model.Owner, error)
	// FindOwners returns one page of owners. It filters by last name when
	// lastName is non-nil and returns all owners otherwise.
	FindOwners(ctx context.Context, lastName *string, page, size int) (store.Page[*model.Owner], error)
	// FindOwnerByID returns an owner with pets loaded, or an error wrapping store.ErrNotFound.
	FindOwnerByID(ctx context.Context, id int) (*model.Owner, error)
	// FindOwnerByLastName returns all owners whose last name matches.
	FindOwnerByLastName(ctx context.Context, lastName string) ([]*model.Owner, error)
	// SaveOwner inserts or updates an owner.
	SaveOwner(ctx context.Context, owner *model.Owner) error
	// DeleteOwner deletes an owner.
	DeleteOwner(ctx context.Context, owner *model.Owner) error

	// FindPetTypeByID returns a pet type, or an error wrapping store.ErrNotFound.
	FindPetTypeByID(ctx context.Context, id int) (*model.PetType, error)
	// FindAllPetTypes returns all pet types.
	FindAllPetTypes(ctx context.Context) ([]*model.PetType, error)
	// SavePetType inserts or updates a pet type.
	SavePetType(ctx context.Context, petType *model.PetType) error
	// DeletePetType deletes a pet type.
	DeletePetType(ctx context.Context, petType *model.PetType) error

	// FindSpecialtyByID returns a specialty, or an error wrapping store.ErrNotFound.
	FindSpecialtyByID(ctx context.Context, id int) (*model.Specialty, error)
	// FindAllSpecialties returns all specialties.
	FindAllSpecialties(ctx context.Context) ([]*model.Specialty, error)
	// SaveSpecialty inserts or updates a specialty.
	SaveSpecialty(ctx context.Context, specialty *model.Specialty) error
	// DeleteSpecialty deletes a specialty.
	DeleteSpecialty(ctx context.Context, specialty *model.Specialty) error
	// FindSpecialtiesByNameIn returns the specialties whose names are in
	// names. It returns (nil, nil) when the repository reports not-found.
	FindSpecialtiesByNameIn(ctx context.Context, names []string) ([]*model.Specialty, error)
}

// Transactor runs fn inside a single database transaction. The transaction
// is carried in the context passed to fn.
type Transactor interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// PetRepository is the subset of pet persistence used by the service.
type PetRepository interface {
	FindAll(ctx context.Context) ([]*model.Pet, error)
	FindAllPage(ctx context.Context, page, size int) (store.Page[*model.Pet], error)
	FindByID(ctx context.Context, id int) (*model.Pet, error)
	FindPetTypes(ctx context.Context) ([]*model.PetType, error)
	Save(ctx context.Context, pet *model.Pet) error
	Delete(ctx context.Context, pet *model.Pet) error
}

// OwnerRepository is the subset of owner persistence used by the service.
type OwnerRepository interface {
	FindAll(ctx context.Context) ([]*model.Owner, error)
	FindAllPage(ctx context.Context, page, size int) (store.Page[*model.Owner], error)
	FindByLastName(ctx context.Context, lastName string) ([]*model.Owner, error)
	FindByLastNamePage(ctx context.Context, lastName string, page, size int) (store.Page[*model.Owner], error)
	FindByID(ctx context.Context, id int) (*model.Owner, error)
	Save(ctx context.Context, owner *model.Owner) error
	Delete(ctx context.Context, owner *model.Owner) error
}

// VetRepository is the subset of vet persistence used by the service.
type VetRepository interface {
	FindAll(ctx context.Context) ([]*model.Vet, error)
	FindByID(ctx context.Context, id int) (*model.Vet, error)
	Save(ctx context.Context, vet *model.Vet) error
	Delete(ctx context.Context, vet *model.Vet) error
}

// VisitRepository is the subset of visit persistence used by the service.
type VisitRepository interface {
	FindAll(ctx context.Context) ([]*model.Visit, error)
	FindByID(ctx context.Context, id int) (*model.Visit, error)
	FindByPetID(ctx context.Context, petID int) ([]*model.Visit, error)
	Save(ctx context.Context, visit *model.Visit) error
	Delete(ctx context.Context, visit *model.Visit) error
}

// SpecialtyRepository is the subset of specialty persistence used by the service.
type SpecialtyRepository interface {
	FindAll(ctx context.Context) ([]*model.Specialty, error)
	FindByID(ctx context.Context, id int) (*model.Specialty, error)
	FindSpecialtiesByNameIn(ctx context.Context, names []string) ([]*model.Specialty, error)
	Save(ctx context.Context, specialty *model.Specialty) error
	Delete(ctx context.Context, specialty *model.Specialty) error
}

// PetTypeRepository is the subset of pet type persistence used by the service.
type PetTypeRepository interface {
	FindAll(ctx context.Context) ([]*model.PetType, error)
	FindByID(ctx context.Context, id int) (*model.PetType, error)
	Save(ctx context.Context, petType *model.PetType) error
	Delete(ctx context.Context, petType *model.PetType) error
}

// ClinicService is the default implementation of Clinic. It is the Go port
// of ClinicServiceImpl and holds no state besides its dependencies.
type ClinicService struct {
	tx          Transactor
	pets        PetRepository
	vets        VetRepository
	owners      OwnerRepository
	visits      VisitRepository
	specialties SpecialtyRepository
	petTypes    PetTypeRepository
}

// Compile-time check that ClinicService satisfies Clinic.
var _ Clinic = (*ClinicService)(nil)

// NewClinic builds a ClinicService from its repositories.
//
// tx may be nil. In that case each write runs directly, without an explicit
// transaction.
func NewClinic(
	tx Transactor,
	pets PetRepository,
	vets VetRepository,
	owners OwnerRepository,
	visits VisitRepository,
	specialties SpecialtyRepository,
	petTypes PetTypeRepository,
) *ClinicService {
	return &ClinicService{
		tx:          tx,
		pets:        pets,
		vets:        vets,
		owners:      owners,
		visits:      visits,
		specialties: specialties,
		petTypes:    petTypes,
	}
}

// inTx runs fn in a transaction when a Transactor is configured.
func (s *ClinicService) inTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if s.tx == nil {
		return fn(ctx)
	}
	return s.tx.WithTx(ctx, fn)
}

// validator is implemented by model entities that carry bean-style validation.
type validator interface {
	Validate() error
}

// validate runs v.Validate() when v supports it.
func validate(v any) error {
	if val, ok := v.(validator); ok {
		if err := val.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ---- pets ----

// FindAllPets returns all pets.
func (s *ClinicService) FindAllPets(ctx context.Context) ([]*model.Pet, error) {
	pets, err := s.pets.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("find all pets: %w", err)
	}
	return pets, nil
}

// FindPets returns one page of pets.
func (s *ClinicService) FindPets(ctx context.Context, page, size int) (store.Page[*model.Pet], error) {
	p, err := s.pets.FindAllPage(ctx, page, size)
	if err != nil {
		return p, fmt.Errorf("find pets page: %w", err)
	}
	return p, nil
}

// DeletePet deletes a pet in its own transaction.
func (s *ClinicService) DeletePet(ctx context.Context, pet *model.Pet) error {
	return s.inTx(ctx, func(ctx context.Context) error {
		if err := s.pets.Delete(ctx, pet); err != nil {
			return fmt.Errorf("delete pet: %w", err)
		}
		return nil
	})
}

// FindPetByID returns a pet with its owner, or an error wrapping store.ErrNotFound.
func (s *ClinicService) FindPetByID(ctx context.Context, id int) (*model.Pet, error) {
	pet, err := s.pets.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find pet %d: %w", id, err)
	}
	return pet, nil
}

// SavePet resolves the pet's type by its id, replaces pet.Type with the
// stored type, then inserts or updates the pet. A missing or unknown pet
// type yields an error wrapping store.ErrIntegrity.
//
// MIGRATION_NOTE: In the source a missing type became null, and the insert
// then failed on the NOT NULL type_id column. Here that failure is reported
// up front as store.ErrIntegrity, so the handler answers the same way.
func (s *ClinicService) SavePet(ctx context.Context, pet *model.Pet) error {
	if pet == nil {
		return errors.New("save pet: nil pet")
	}
	if err := validate(pet); err != nil {
		return err
	}
	return s.inTx(ctx, func(ctx context.Context) error {
		if pet.Type == nil {
			return fmt.Errorf("save pet: pet type missing: %w", store.ErrIntegrity)
		}
		typeID, ok := entityID(pet.Type.ID)
		if !ok {
			return fmt.Errorf("save pet: pet type id missing: %w", store.ErrIntegrity)
		}
		pt, err := s.petTypes.FindByID(ctx, typeID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return fmt.Errorf("save pet: pet type %d not found: %w", typeID, store.ErrIntegrity)
			}
			return fmt.Errorf("save pet: resolve pet type %d: %w", typeID, err)
		}
		pet.Type = pt
		if err := s.pets.Save(ctx, pet); err != nil {
			return fmt.Errorf("save pet: %w", err)
		}
		return nil
	})
}

// entityID extracts an int id from either an int or a *int field.
// It reports false when the id is nil.
func entityID(v any) (int, bool) {
	switch id := v.(type) {
	case int:
		return id, true
	case *int:
		if id == nil {
			return 0, false
		}
		return *id, true
	default:
		return 0, false
	}
}

// FindPetTypes returns all pet types via the pet repository.
func (s *ClinicService) FindPetTypes(ctx context.Context) ([]*model.PetType, error) {
	types, err := s.pets.FindPetTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("find pet types: %w", err)
	}
	return types, nil
}

// ---- visits ----

// FindVisitByID returns a visit, or an error wrapping store.ErrNotFound.
func (s *ClinicService) FindVisitByID(ctx context.Context, id int) (*model.Visit, error) {
	v, err := s.visits.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find visit %d: %w", id, err)
	}
	return v, nil
}

// FindAllVisits returns all visits.
func (s *ClinicService) FindAllVisits(ctx context.Context) ([]*model.Visit, error) {
	v, err := s.visits.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("find all visits: %w", err)
	}
	return v, nil
}

// FindVisitsByPetID returns the visits of one pet.
func (s *ClinicService) FindVisitsByPetID(ctx context.Context, petID int) ([]*model.Visit, error) {
	v, err := s.visits.FindByPetID(ctx, petID)
	if err != nil {
		return nil, fmt.Errorf("find visits for pet %d: %w", petID, err)
	}
	return v, nil
}

// SaveVisit validates, then inserts or updates a visit.
func (s *ClinicService) SaveVisit(ctx context.Context, visit *model.Visit) error {
	if visit == nil {
		return errors.New("save visit: nil visit")
	}
	if err := validate(visit); err != nil {
		return err
	}
	return s.inTx(ctx, func(ctx context.Context) error {
		if err := s.visits.Save(ctx, visit); err != nil {
			return fmt.Errorf("save visit: %w", err)
		}
		return nil
	})
}

// DeleteVisit deletes a visit.
func (s *ClinicService) DeleteVisit(ctx context.Context, visit *model.Visit) error {
	return s.inTx(ctx, func(ctx context.Context) error {
		if err := s.visits.Delete(ctx, visit); err != nil {
			return fmt.Errorf("delete visit: %w", err)
		}
		return nil
	})
}

// ---- vets ----

// FindVetByID returns a vet, or an error wrapping store.ErrNotFound.
func (s *ClinicService) FindVetByID(ctx context.Context, id int) (*model.Vet, error) {
	v, err := s.vets.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find vet %d: %w", id, err)
	}
	return v, nil
}

// FindAllVets returns all vets with their specialties.
func (s *ClinicService) FindAllVets(ctx context.Context) ([]*model.Vet, error) {
	v, err := s.vets.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("find all vets: %w", err)
	}
	return v, nil
}

// FindVets returns all vets. It is identical to FindAllVets.
func (s *ClinicService) FindVets(ctx context.Context) ([]*model.Vet, error) {
	return s.FindAllVets(ctx)
}

// SaveVet validates, then inserts or updates a vet.
func (s *ClinicService) SaveVet(ctx context.Context, vet *model.Vet) error {
	if vet == nil {
		return errors.New("save vet: nil vet")
	}
	if err := validate(vet); err != nil {
		return err
	}
	return s.inTx(ctx, func(ctx context.Context) error {
		if err := s.vets.Save(ctx, vet); err != nil {
			return fmt.Errorf("save vet: %w", err)
		}
		return nil
	})
}

// DeleteVet deletes a vet.
func (s *ClinicService) DeleteVet(ctx context.Context, vet *model.Vet) error {
	return s.inTx(ctx, func(ctx context.Context) error {
		if err := s.vets.Delete(ctx, vet); err != nil {
			return fmt.Errorf("delete vet: %w", err)
		}
		return nil
	})
}

// ---- owners ----

// FindAllOwners returns all owners.
func (s *ClinicService) FindAllOwners(ctx context.Context) ([]*model.Owner, error) {
	o, err := s.owners.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("find all owners: %w", err)
	}
	return o, nil
}

// FindOwners returns one page of owners. It filters by last name when
// lastName is non-nil and returns all owners otherwise.
func (s *ClinicService) FindOwners(ctx context.Context, lastName *string, page, size int) (store.Page[*model.Owner], error) {
	if lastName != nil {
		p, err := s.owners.FindByLastNamePage(ctx, *lastName, page, size)
		if err != nil {
			return p, fmt.Errorf("find owners by last name page: %w", err)
		}
		return p, nil
	}
	p, err := s.owners.FindAllPage(ctx, page, size)
	if err != nil {
		return p, fmt.Errorf("find owners page: %w", err)
	}
	return p, nil
}

// FindOwnerByID returns an owner with pets loaded, or an error wrapping store.ErrNotFound.
func (s *ClinicService) FindOwnerByID(ctx context.Context, id int) (*model.Owner, error) {
	o, err := s.owners.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find owner %d: %w", id, err)
	}
	return o, nil
}

// FindOwnerByLastName returns all owners whose last name matches.
func (s *ClinicService) FindOwnerByLastName(ctx context.Context, lastName string) ([]*model.Owner, error) {
	o, err := s.owners.FindByLastName(ctx, lastName)
	if err != nil {
		return nil, fmt.Errorf("find owners by last name: %w", err)
	}
	return o, nil
}

// SaveOwner validates, then inserts or updates an owner.
func (s *ClinicService) SaveOwner(ctx context.Context, owner *model.Owner) error {
	if owner == nil {
		return errors.New("save owner: nil owner")
	}
	if err := validate(owner); err != nil {
		return err
	}
	return s.inTx(ctx, func(ctx context.Context) error {
		if err := s.owners.Save(ctx, owner); err != nil {
			return fmt.Errorf("save owner: %w", err)
		}
		return nil
	})
}

// DeleteOwner deletes an owner.
func (s *ClinicService) DeleteOwner(ctx context.Context, owner *model.Owner) error {
	return s.inTx(ctx, func(ctx context.Context) error {
		if err := s.owners.Delete(ctx, owner); err != nil {
			return fmt.Errorf("delete owner: %w", err)
		}
		return nil
	})
}

// ---- pet types ----

// FindPetTypeByID returns a pet type, or an error wrapping store.ErrNotFound.
func (s *ClinicService) FindPetTypeByID(ctx context.Context, id int) (*model.PetType, error) {
	pt, err := s.petTypes.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find pet type %d: %w", id, err)
	}
	return pt, nil
}

// FindAllPetTypes returns all pet types.
func (s *ClinicService) FindAllPetTypes(ctx context.Context) ([]*model.PetType, error) {
	pts, err := s.petTypes.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("find all pet types: %w", err)
	}
	return pts, nil
}

// SavePetType inserts or updates a pet type.
func (s *ClinicService) SavePetType(ctx context.Context, petType *model.PetType) error {
	if petType == nil {
		return errors.New("save pet type: nil pet type")
	}
	if err := validate(petType); err != nil {
		return err
	}
	return s.inTx(ctx, func(ctx context.Context) error {
		if err := s.petTypes.Save(ctx, petType); err != nil {
			return fmt.Errorf("save pet type: %w", err)
		}
		return nil
	})
}

// DeletePetType deletes a pet type.
func (s *ClinicService) DeletePetType(ctx context.Context, petType *model.PetType) error {
	return s.inTx(ctx, func(ctx context.Context) error {
		if err := s.petTypes.Delete(ctx, petType); err != nil {
			return fmt.Errorf("delete pet type: %w", err)
		}
		return nil
	})
}

// ---- specialties ----

// FindSpecialtyByID returns a specialty, or an error wrapping store.ErrNotFound.
func (s *ClinicService) FindSpecialtyByID(ctx context.Context, id int) (*model.Specialty, error) {
	sp, err := s.specialties.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find specialty %d: %w", id, err)
	}
	return sp, nil
}

// FindAllSpecialties returns all specialties.
func (s *ClinicService) FindAllSpecialties(ctx context.Context) ([]*model.Specialty, error) {
	sps, err := s.specialties.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("find all specialties: %w", err)
	}
	return sps, nil
}

// SaveSpecialty inserts or updates a specialty.
func (s *ClinicService) SaveSpecialty(ctx context.Context, specialty *model.Specialty) error {
	if specialty == nil {
		return errors.New("save specialty: nil specialty")
	}
	if err := validate(specialty); err != nil {
		return err
	}
	return s.inTx(ctx, func(ctx context.Context) error {
		if err := s.specialties.Save(ctx, specialty); err != nil {
			return fmt.Errorf("save specialty: %w", err)
		}
		return nil
	})
}

// DeleteSpecialty deletes a specialty.
func (s *ClinicService) DeleteSpecialty(ctx context.Context, specialty *model.Specialty) error {
	return s.inTx(ctx, func(ctx context.Context) error {
		if err := s.specialties.Delete(ctx, specialty); err != nil {
			return fmt.Errorf("delete specialty: %w", err)
		}
		return nil
	})
}

// FindSpecialtiesByNameIn returns the specialties whose names are in names.
// Mirroring the source's findEntityById helper, a not-found result from the
// repository is swallowed and (nil, nil) is returned.
func (s *ClinicService) FindSpecialtiesByNameIn(ctx context.Context, names []string) ([]*model.Specialty, error) {
	sps, err := s.specialties.FindSpecialtiesByNameIn(ctx, names)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find specialties by name: %w", err)
	}
	return sps, nil
}
