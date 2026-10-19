// Package mapper converts between the petclinic domain model and the REST
// DTOs. It replaces the MapStruct mapper interfaces of the source project.
//
// This file replaces PetMapper. It also contains the pet type and visit
// mapping that PetMapper delegates to (MapStruct's uses = VisitMapper.class
// and its built-in PetType <-> PetTypeDto methods).
//
// MIGRATION_NOTE: MapStruct generated these mappers at compile time from
// annotations. In Go they are plain functions, and name-based implicit mapping
// is written out field by field. The source's overloaded toPet(PetDto) and
// toPet(PetFieldsDto) become PetFromDTO and PetFromFieldsDTO. Collection
// methods are written as range loops.
//
// Behaviour preserved from the MapStruct-generated code:
//   - A nil entity or DTO maps to nil.
//   - A nil collection maps to nil (JSON null), as MapStruct does.
//   - A nil element in a collection maps to a nil element; it is not dropped.
//   - Nullable scalars (id, birthDate, date, ownerId, petId) are pointers and
//     are copied as is, nil included. No constructor default is substituted.
//   - toPet(PetDto) always creates the nested owner for target "owner.id",
//     even when ownerId is null.
package mapper

import (
	"migrated-app/internal/model"
)

// copyIntPtr returns an independent copy of a nullable int.
func copyIntPtr(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// copyDatePtr returns an independent copy of a nullable model.Date.
//
// MIGRATION_NOTE: the DTOs use model.Date (the domain's calendar-date type,
// declared in internal/model/visit.go) directly instead of a separate DTO date
// type. This removes all date conversions, so the mapper does not depend on
// the internal layout of model.Date.
func copyDatePtr(p *model.Date) *model.Date {
	if p == nil {
		return nil
	}
	d := *p
	return &d
}

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

// PetTypeDTO is the REST representation of a pet type.
type PetTypeDTO struct {
	Name string `json:"name"`
	ID   *int   `json:"id"`
}

// VisitDTO is the REST representation of a visit.
type VisitDTO struct {
	ID          *int        `json:"id"`
	Date        *model.Date `json:"date"`
	Description string      `json:"description"`
	PetID       *int        `json:"petId"`
}

// PetFieldsDTO holds the editable fields of a pet.
type PetFieldsDTO struct {
	Name      string      `json:"name"`
	BirthDate *model.Date `json:"birthDate"`
	Type      *PetTypeDTO `json:"type"`
}

// PetDTO is the REST representation of a pet.
type PetDTO struct {
	Name      string      `json:"name"`
	BirthDate *model.Date `json:"birthDate"`
	Type      *PetTypeDTO `json:"type"`
	ID        *int        `json:"id"`
	OwnerID   *int        `json:"ownerId"`
	Visits    []*VisitDTO `json:"visits"`
}

// PetPageDTO is one page of pets plus pagination metadata.
type PetPageDTO struct {
	Content       []*PetDTO `json:"content"`
	Page          int       `json:"page"`
	Size          int       `json:"size"`
	TotalElements int64     `json:"totalElements"`
	TotalPages    int       `json:"totalPages"`
}

// ---------------------------------------------------------------------------
// Pet types
// ---------------------------------------------------------------------------

// PetTypeToDTO converts a PetType entity into a PetTypeDTO. A nil input
// returns nil.
func PetTypeToDTO(pt *model.PetType) *PetTypeDTO {
	if pt == nil {
		return nil
	}
	return &PetTypeDTO{
		ID:   copyIntPtr(pt.ID),
		Name: pt.Name,
	}
}

// PetTypeFromDTO converts a PetTypeDTO into a PetType entity. A nil input
// returns nil.
func PetTypeFromDTO(dto *PetTypeDTO) *model.PetType {
	if dto == nil {
		return nil
	}
	pt := &model.PetType{}
	pt.ID = copyIntPtr(dto.ID)
	pt.Name = dto.Name
	return pt
}

// PetTypesToDTOs converts a slice of PetType entities into DTOs. A nil slice
// returns nil, and nil elements map to nil elements.
func PetTypesToDTOs(types []*model.PetType) []*PetTypeDTO {
	if types == nil {
		return nil
	}
	out := make([]*PetTypeDTO, 0, len(types))
	for _, pt := range types {
		out = append(out, PetTypeToDTO(pt))
	}
	return out
}

// ---------------------------------------------------------------------------
// Visits (delegated mapper, VisitMapper in the source)
// ---------------------------------------------------------------------------

// VisitToDTO converts a Visit entity into a VisitDTO. petId is taken from the
// visit's PetID foreign key. A nil input returns nil.
//
// MIGRATION_NOTE: the source read visit.pet.id. In the Go model Visit.Pet is
// the PetRef interface, which has no id, and the id is stored in the PetID
// column field. The output is the same.
func VisitToDTO(v *model.Visit) *VisitDTO {
	if v == nil {
		return nil
	}
	return &VisitDTO{
		ID:          copyIntPtr(v.ID),
		Date:        copyDatePtr(v.Date),
		Description: v.Description,
		PetID:       copyIntPtr(v.PetID),
	}
}

// VisitFromDTO converts a VisitDTO into a Visit entity. petId sets the
// visit's PetID foreign key. A nil input returns nil.
//
// MIGRATION_NOTE: the source built a Pet holding only the id (target
// "pet.id"). In Go the foreign key Visit.PetID carries that id, which is what
// the store layer persists. Visit.Pet stays nil. Callers that need the loaded
// pet resolve it through the service using PetID, as the source handlers do.
func VisitFromDTO(dto *VisitDTO) *model.Visit {
	if dto == nil {
		return nil
	}
	v := &model.Visit{}
	v.ID = copyIntPtr(dto.ID)
	v.Date = copyDatePtr(dto.Date)
	v.Description = dto.Description
	v.PetID = copyIntPtr(dto.PetID)
	return v
}

// VisitsToDTOs converts a slice of Visit entities into DTOs. A nil slice
// returns nil, and nil elements map to nil elements.
func VisitsToDTOs(visits []*model.Visit) []*VisitDTO {
	if visits == nil {
		return nil
	}
	out := make([]*VisitDTO, 0, len(visits))
	for _, v := range visits {
		out = append(out, VisitToDTO(v))
	}
	return out
}

// visitsFromDTOs converts a slice of VisitDTOs into Visit entities. A nil
// slice returns nil, and nil elements map to nil elements.
func visitsFromDTOs(dtos []*VisitDTO) []*model.Visit {
	if dtos == nil {
		return nil
	}
	out := make([]*model.Visit, 0, len(dtos))
	for _, d := range dtos {
		out = append(out, VisitFromDTO(d))
	}
	return out
}

// ---------------------------------------------------------------------------
// Pets
// ---------------------------------------------------------------------------

// PetToDTO converts a Pet entity into a PetDTO. ownerId comes from the
// owner's id. Visits come from Pet.GetVisits, which returns them newest
// first, as Pet.getVisits() does in the source. A nil input returns nil.
func PetToDTO(p *model.Pet) *PetDTO {
	if p == nil {
		return nil
	}
	return &PetDTO{
		ID:        copyIntPtr(p.ID),
		Name:      p.Name,
		BirthDate: copyDatePtr(p.GetBirthDate()),
		Type:      PetTypeToDTO(p.GetType()),
		OwnerID:   copyIntPtr(p.GetOwnerID()),
		Visits:    VisitsToDTOs(p.GetVisits()),
	}
}

// PetsToDTOs converts a slice of Pet entities into DTOs. A nil slice returns
// nil, and nil elements map to nil elements, as in MapStruct's toPetsDto.
func PetsToDTOs(pets []*model.Pet) []*PetDTO {
	if pets == nil {
		return nil
	}
	out := make([]*PetDTO, 0, len(pets))
	for _, p := range pets {
		out = append(out, PetToDTO(p))
	}
	return out
}

// PetFromDTO converts a PetDTO into a Pet entity. A nil input returns nil.
//
// As in the MapStruct-generated code for target "owner.id", an owner
// reference is always attached and carries only dto.OwnerID (which may be
// nil). Visits are mapped through VisitFromDTO. A nil visits slice leaves the
// pet's visits untouched.
func PetFromDTO(dto *PetDTO) *model.Pet {
	if dto == nil {
		return nil
	}
	p := &model.Pet{}
	p.ID = copyIntPtr(dto.ID)
	p.Name = dto.Name
	p.SetBirthDate(copyDatePtr(dto.BirthDate))
	p.SetType(PetTypeFromDTO(dto.Type))

	owner := &model.Owner{}
	owner.ID = copyIntPtr(dto.OwnerID)
	p.SetOwner(owner)

	if dto.Visits != nil {
		p.SetVisits(visitsFromDTOs(dto.Visits))
	}
	return p
}

// PetsFromDTOs converts a slice of PetDTOs into Pet entities. A nil slice
// returns nil, and nil elements map to nil elements, as in MapStruct's
// toPets.
func PetsFromDTOs(dtos []*PetDTO) []*model.Pet {
	if dtos == nil {
		return nil
	}
	out := make([]*model.Pet, 0, len(dtos))
	for _, d := range dtos {
		out = append(out, PetFromDTO(d))
	}
	return out
}

// PetFromFieldsDTO converts editable pet fields into a new Pet entity. The id,
// owner and visits are not set, matching the ignore=true mappings in the
// source. A nil input returns nil.
func PetFromFieldsDTO(dto *PetFieldsDTO) *model.Pet {
	if dto == nil {
		return nil
	}
	p := &model.Pet{}
	p.Name = dto.Name
	p.SetBirthDate(copyDatePtr(dto.BirthDate))
	p.SetType(PetTypeFromDTO(dto.Type))
	return p
}

// PetPageToDTO builds a PetPageDTO from one page of pets and its pagination
// metadata. page is zero-based, as in Spring Data's Page.getNumber().
//
// MIGRATION_NOTE: the source took a Spring Data Page<Pet>, which is declared
// @NonNull and whose content is never null. Here the caller passes the page
// content and metadata explicitly, so this package does not depend on a
// paging type from the store layer. A nil content slice becomes an empty
// slice, so JSON renders [] as the source did. Nil elements are kept.
func PetPageToDTO(content []*model.Pet, page, size int, totalElements int64, totalPages int) PetPageDTO {
	dtos := PetsToDTOs(content)
	if dtos == nil {
		dtos = []*PetDTO{}
	}
	return PetPageDTO{
		Content:       dtos,
		Page:          page,
		Size:          size,
		TotalElements: totalElements,
		TotalPages:    totalPages,
	}
}
