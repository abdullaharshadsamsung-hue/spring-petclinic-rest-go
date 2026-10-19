package mapper

// This file is the Go port of OwnerMapper (a MapStruct @Mapper interface).
// It converts between the domain model.Owner and the REST DTOs.
//
// MIGRATION_NOTE: MapStruct generates mapper code at compile time. Go has no
// annotation processing, so every mapping is written out by hand.
// @Mapper(uses = PetMapper.class) becomes a PetMapper dependency injected
// through NewOwnerMapper.
//
// MIGRATION_NOTE: The DTO types (OwnerDto, OwnerFieldsDto, OwnerPageDto,
// PetDto) and the Spring Data Page abstraction are declared here because the
// rest/dto package was not in the provided context. If a migrated dto
// package, or a pet_mapper.go in this package, already declares any of these
// types, delete the duplicate declarations below and import those instead.
// The JSON tags follow the petclinic OpenAPI contract.

import (
	"errors"

	"migrated-app/internal/model"
)

// ErrNilPage is returned by OwnerMapper.ToOwnerPageDto when it is given a nil
// page. It replaces the source's @NonNull contract on the Page parameter.
var ErrNilPage = errors.New("mapper: owner page must not be nil")

// PetDto is the REST representation of a pet nested inside an OwnerDto.
//
// MIGRATION_NOTE: The full field set is owned by PetMapper/PetDto. It is
// declared here only so that OwnerDto can reference it.
type PetDto struct {
	ID        *int        `json:"id,omitempty"`
	Name      string      `json:"name"`
	BirthDate *model.Date `json:"birthDate,omitempty"`
	OwnerID   *int        `json:"ownerId,omitempty"`
}

// OwnerFieldsDto carries the editable owner fields, with no id and no pets.
// It is the request body for creating or updating an owner.
type OwnerFieldsDto struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Address   string `json:"address"`
	City      string `json:"city"`
	Telephone string `json:"telephone"`
}

// OwnerDto is the full REST representation of an owner, including the id and
// the owner's pets.
type OwnerDto struct {
	ID        *int     `json:"id,omitempty"`
	FirstName string   `json:"firstName"`
	LastName  string   `json:"lastName"`
	Address   string   `json:"address"`
	City      string   `json:"city"`
	Telephone string   `json:"telephone"`
	Pets      []PetDto `json:"pets"`
}

// OwnerPageDto is one page of owners plus the paging metadata.
type OwnerPageDto struct {
	Content       []OwnerDto `json:"content"`
	Page          int        `json:"page"`
	Size          int        `json:"size"`
	TotalElements int64      `json:"totalElements"`
	TotalPages    int        `json:"totalPages"`
}

// Page is the Go replacement for Spring Data's Page<T>. Number is the
// zero-based page index and Size is the requested page size.
type Page[T any] struct {
	Content       []T
	Number        int
	Size          int
	TotalElements int64
	TotalPages    int
}

// PetMapper converts pets between the domain model and DTOs. OwnerMapper
// delegates the nested pet collection to it, as MapStruct's
// uses = PetMapper.class did.
type PetMapper interface {
	// ToPetDto converts a domain pet into its DTO.
	ToPetDto(pet *model.Pet) PetDto
	// ToPet converts a pet DTO into a domain pet.
	ToPet(dto PetDto) *model.Pet
}

// OwnerMapper converts owners between model.Owner and the REST DTOs.
type OwnerMapper struct {
	pets PetMapper
}

// NewOwnerMapper returns an OwnerMapper that uses pets for nested pet
// conversion. If pets is nil, pet collections are not mapped and are left
// nil in both directions.
func NewOwnerMapper(pets PetMapper) *OwnerMapper {
	return &OwnerMapper{pets: pets}
}

// ToOwnerDto converts a domain owner into an OwnerDto. Like MapStruct, it
// returns nil for a nil input.
func (m *OwnerMapper) ToOwnerDto(owner *model.Owner) *OwnerDto {
	if owner == nil {
		return nil
	}
	id := owner.ID
	dto := &OwnerDto{
		ID:        &id,
		FirstName: owner.FirstName,
		LastName:  owner.LastName,
		Address:   owner.Address,
		City:      owner.City,
		Telephone: owner.Telephone,
	}
	if m.pets != nil && owner.Pets != nil {
		dto.Pets = make([]PetDto, 0, len(owner.Pets))
		for _, p := range owner.Pets {
			if p == nil {
				continue
			}
			dto.Pets = append(dto.Pets, m.pets.ToPetDto(p))
		}
	}
	return dto
}

// ToOwner converts a full OwnerDto, including id and pets, into a domain
// owner. It returns nil for a nil input.
//
// MIGRATION_NOTE: MapStruct assigned the pets collection through setPets
// without wiring the back-reference. This port does the same. If model.Owner
// exposes an AddPet/SetOwner helper that also sets Pet.OwnerID, prefer
// calling it here.
func (m *OwnerMapper) ToOwner(dto *OwnerDto) *model.Owner {
	if dto == nil {
		return nil
	}
	owner := &model.Owner{
		FirstName: dto.FirstName,
		LastName:  dto.LastName,
		Address:   dto.Address,
		City:      dto.City,
		Telephone: dto.Telephone,
	}
	if dto.ID != nil {
		owner.ID = *dto.ID
	}
	if m.pets != nil && dto.Pets != nil {
		owner.Pets = make([]*model.Pet, 0, len(dto.Pets))
		for _, p := range dto.Pets {
			owner.Pets = append(owner.Pets, m.pets.ToPet(p))
		}
	}
	return owner
}

// ToOwnerFromFields converts an OwnerFieldsDto into a domain owner. As with
// the source's @Mapping(target = "id", ignore = true) and
// @Mapping(target = "pets", ignore = true), the id is left at its zero value
// and pets is left nil. It returns nil for a nil input.
//
// MIGRATION_NOTE: This is the Java overload toOwner(OwnerFieldsDto), renamed
// because Go has no method overloading.
func (m *OwnerMapper) ToOwnerFromFields(dto *OwnerFieldsDto) *model.Owner {
	if dto == nil {
		return nil
	}
	return &model.Owner{
		FirstName: dto.FirstName,
		LastName:  dto.LastName,
		Address:   dto.Address,
		City:      dto.City,
		Telephone: dto.Telephone,
	}
}

// ToOwnerDtoCollection converts a slice of domain owners into DTOs. A nil
// input yields nil and an empty input yields an empty, non-nil slice. Nil
// elements are skipped.
func (m *OwnerMapper) ToOwnerDtoCollection(owners []*model.Owner) []OwnerDto {
	if owners == nil {
		return nil
	}
	out := make([]OwnerDto, 0, len(owners))
	for _, o := range owners {
		if dto := m.ToOwnerDto(o); dto != nil {
			out = append(out, *dto)
		}
	}
	return out
}

// ToOwners converts a slice of OwnerDto into domain owners. A nil input
// yields nil.
func (m *OwnerMapper) ToOwners(dtos []OwnerDto) []*model.Owner {
	if dtos == nil {
		return nil
	}
	out := make([]*model.Owner, 0, len(dtos))
	for i := range dtos {
		out = append(out, m.ToOwner(&dtos[i]))
	}
	return out
}

// ToOwnerPageDto converts a page of domain owners into an OwnerPageDto. It
// copies the mapped content together with the page number, size, total
// elements and total pages. It returns ErrNilPage if page is nil.
func (m *OwnerMapper) ToOwnerPageDto(page *Page[*model.Owner]) (*OwnerPageDto, error) {
	if page == nil {
		return nil, ErrNilPage
	}
	content := m.ToOwnerDtoCollection(page.Content)
	if content == nil {
		// A Spring Page never has null content, so always emit [].
		content = []OwnerDto{}
	}
	return &OwnerPageDto{
		Content:       content,
		Page:          page.Number,
		Size:          page.Size,
		TotalElements: page.TotalElements,
		TotalPages:    page.TotalPages,
	}, nil
}
