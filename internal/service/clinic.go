// Package service contains the service layer of the petclinic application.
//
// Clinic is the single facade that every HTTP handler depends on for CRUD and
// query operations on pets, visits, vets, owners, pet types and specialties.
// Handlers depend on this interface rather than on a concrete type so it can
// be replaced by a fake in tests.
//
// MIGRATION_NOTE: In the source project this was the Spring ClinicService
// interface. The differences are:
//   - Spring's unchecked DataAccessException becomes an ordinary returned
//     error. Use errors.Is(err, store.ErrNotFound) to detect a missing entity
//     and errors.Is(err, store.ErrIntegrity) to detect a constraint violation.
//   - Find-by-id methods return an error wrapping store.ErrNotFound when the
//     entity does not exist. They do not return (nil, nil). The Java
//     implementation returned null, and controllers turned that into 404.
//   - Spring Data's Pageable/Page<T> become store.PageRequest/store.Page[T].
//   - Transaction boundaries, which were @Transactional on the
//     implementation, are the implementation's job and do not appear in this
//     contract.
package service

import (
	"context"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

// Clinic is the service-layer facade used by all REST handlers.
//
// Save methods are upserts. When the entity is new (nil id) it is inserted
// and the generated id is written back to the entity. Otherwise the existing
// row is updated. Delete methods take the entity, not its id, to mirror the
// source's entity-based removal semantics.
//
// Collection-returning methods return an empty, non-nil slice when there are
// no results.
type Clinic interface {
	// FindPetByID returns the pet with the given id.
	FindPetByID(ctx context.Context, id int) (*model.Pet, error)
	// FindAllPets returns every pet.
	FindAllPets(ctx context.Context) ([]*model.Pet, error)
	// FindPets returns one page of pets according to req's paging and
	// sorting parameters.
	FindPets(ctx context.Context, req store.PageRequest) (store.Page[*model.Pet], error)
	// SavePet creates a new pet or updates an existing one.
	SavePet(ctx context.Context, pet *model.Pet) error
	// DeletePet deletes the pet together with its dependent visits.
	DeletePet(ctx context.Context, pet *model.Pet) error

	// FindVisitsByPetID returns all visits for the pet with the given id.
	FindVisitsByPetID(ctx context.Context, petID int) ([]*model.Visit, error)
	// FindVisitByID returns the visit with the given id.
	FindVisitByID(ctx context.Context, visitID int) (*model.Visit, error)
	// FindAllVisits returns every visit.
	FindAllVisits(ctx context.Context) ([]*model.Visit, error)
	// SaveVisit creates a new visit or updates an existing one.
	SaveVisit(ctx context.Context, visit *model.Visit) error
	// DeleteVisit deletes the visit.
	DeleteVisit(ctx context.Context, visit *model.Visit) error

	// FindVetByID returns the vet with the given id.
	FindVetByID(ctx context.Context, id int) (*model.Vet, error)
	// FindVets returns all vets. This is the listing variant that was
	// cacheable in the source project.
	//
	// MIGRATION_NOTE: The source declared this method @Cacheable("vets"). No
	// cache is applied here. An implementation may add one, but it must then
	// invalidate the cache on SaveVet and DeleteVet.
	FindVets(ctx context.Context) ([]*model.Vet, error)
	// FindAllVets returns all vets.
	FindAllVets(ctx context.Context) ([]*model.Vet, error)
	// SaveVet creates a new vet or updates an existing one, including its
	// specialty associations.
	SaveVet(ctx context.Context, vet *model.Vet) error
	// DeleteVet deletes the vet.
	DeleteVet(ctx context.Context, vet *model.Vet) error

	// FindOwnerByID returns the owner with the given id.
	FindOwnerByID(ctx context.Context, id int) (*model.Owner, error)
	// FindAllOwners returns every owner.
	FindAllOwners(ctx context.Context) ([]*model.Owner, error)
	// FindOwners returns one page of owners. If lastName is not empty, only
	// owners whose last name starts with it are included. An empty lastName
	// stands in for the source's null and means no filter.
	FindOwners(ctx context.Context, lastName string, req store.PageRequest) (store.Page[*model.Owner], error)
	// SaveOwner creates a new owner or updates an existing one.
	SaveOwner(ctx context.Context, owner *model.Owner) error
	// DeleteOwner deletes the owner together with their pets and the pets'
	// visits.
	DeleteOwner(ctx context.Context, owner *model.Owner) error
	// FindOwnerByLastName returns the owners whose last name starts with
	// lastName.
	FindOwnerByLastName(ctx context.Context, lastName string) ([]*model.Owner, error)

	// FindPetTypeByID returns the pet type with the given id.
	FindPetTypeByID(ctx context.Context, petTypeID int) (*model.PetType, error)
	// FindAllPetTypes returns every pet type.
	FindAllPetTypes(ctx context.Context) ([]*model.PetType, error)
	// FindPetTypes returns the pet types. This is the listing variant used
	// to populate pet forms.
	FindPetTypes(ctx context.Context) ([]*model.PetType, error)
	// SavePetType creates a new pet type or updates an existing one.
	SavePetType(ctx context.Context, petType *model.PetType) error
	// DeletePetType deletes the pet type together with the pets that use it
	// and their visits.
	DeletePetType(ctx context.Context, petType *model.PetType) error

	// FindSpecialtyByID returns the specialty with the given id.
	FindSpecialtyByID(ctx context.Context, specialtyID int) (*model.Specialty, error)
	// FindAllSpecialties returns every specialty.
	FindAllSpecialties(ctx context.Context) ([]*model.Specialty, error)
	// SaveSpecialty creates a new specialty or updates an existing one.
	SaveSpecialty(ctx context.Context, specialty *model.Specialty) error
	// DeleteSpecialty deletes the specialty and removes its vet
	// associations.
	DeleteSpecialty(ctx context.Context, specialty *model.Specialty) error
	// FindSpecialtiesByNameIn returns the specialties whose names are in
	// names. The map is used as a set, replacing Java's Set<String>.
	FindSpecialtiesByNameIn(ctx context.Context, names map[string]struct{}) ([]*model.Specialty, error)
}
