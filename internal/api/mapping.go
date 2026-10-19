package api

// This file is the port of the MapStruct PetTypeMapper and VisitMapper. It
// converts between the PetType and Visit domain entities and their REST DTOs.
//
// MIGRATION_NOTE: MapStruct builds the mapper implementation at compile
// time and Spring injects it as a bean. In Go these are plain, stateless
// package-level functions, so handlers call them directly and nothing needs
// to be wired up. Java resolves overloaded toXxx methods by parameter type.
// Go has no overloading, so they become XxxFromDTO and XxxFromFieldsDTO.

import "migrated-app/internal/model"

// PetTypeFieldsDTO carries the editable fields of a pet type. It is the
// "PetTypeFields" schema in openapi.yml.
//
// MIGRATION_NOTE: The source DTOs are generated from the OpenAPI spec. Here
// they are written by hand. If a later file adds a separate DTO file, move
// these declarations there instead of declaring them twice.
type PetTypeFieldsDTO struct {
	Name string `json:"name"`
}

// PetTypeDTO is a pet type as the REST API exposes it: the editable fields
// plus the read-only id. It is the "PetType" schema in openapi.yml.
type PetTypeDTO struct {
	ID   *int   `json:"id"`
	Name string `json:"name"`
}

// PetTypeFromDTO converts a full PetTypeDTO into a PetType entity, copying
// both id and name. A nil input returns nil, as MapStruct does.
func PetTypeFromDTO(dto *PetTypeDTO) *model.PetType {
	if dto == nil {
		return nil
	}
	pt := model.NewPetType()
	pt.SetID(copyIntPtr(dto.ID))
	pt.SetName(dto.Name)
	return pt
}

// PetTypeFromFieldsDTO converts a PetTypeFieldsDTO into a PetType entity.
// The id is deliberately left unset (nil), matching
// @Mapping(target = "id", ignore = true) in the source. A nil input returns
// nil.
func PetTypeFromFieldsDTO(dto *PetTypeFieldsDTO) *model.PetType {
	if dto == nil {
		return nil
	}
	pt := model.NewPetType()
	pt.SetName(dto.Name)
	return pt
}

// PetTypeToDTO converts a PetType entity into a PetTypeDTO, copying both id
// and name. A nil input returns nil.
func PetTypeToDTO(pt *model.PetType) *PetTypeDTO {
	if pt == nil {
		return nil
	}
	return &PetTypeDTO{
		ID:   copyIntPtr(pt.GetID()),
		Name: pt.GetName(),
	}
}

// PetTypeToFieldsDTO converts a PetType entity into a PetTypeFieldsDTO,
// which carries only the name. A nil input returns nil.
func PetTypeToFieldsDTO(pt *model.PetType) *PetTypeFieldsDTO {
	if pt == nil {
		return nil
	}
	return &PetTypeFieldsDTO{Name: pt.GetName()}
}

// PetTypesToDTOs converts a slice of PetType entities into PetTypeDTOs,
// mapping each element with PetTypeToDTO. A nil slice returns nil, as
// MapStruct returns null for a null collection. An empty slice returns an
// empty, non-nil slice, so it encodes to JSON as [] and not null. Nil
// elements are skipped.
func PetTypesToDTOs(petTypes []*model.PetType) []PetTypeDTO {
	if petTypes == nil {
		return nil
	}
	out := make([]PetTypeDTO, 0, len(petTypes))
	for _, pt := range petTypes {
		dto := PetTypeToDTO(pt)
		if dto == nil {
			continue
		}
		out = append(out, *dto)
	}
	return out
}

// VisitFieldsDTO carries the editable fields of a visit. It is the
// "VisitFields" schema in openapi.yml.
//
// MIGRATION_NOTE: Hand-written replacement for the OpenAPI-generated
// VisitFieldsDto. Date is a pointer so an absent/null JSON date is
// distinguishable from a real date.
type VisitFieldsDTO struct {
	Date        *model.Date `json:"date"`
	Description string      `json:"description"`
}

// VisitDTO is a visit as the REST API exposes it: the editable fields plus
// the read-only id and the owning pet's id. It is the "Visit" schema in
// openapi.yml.
type VisitDTO struct {
	ID          *int        `json:"id"`
	Date        *model.Date `json:"date"`
	Description string      `json:"description"`
	PetID       *int        `json:"petId"`
}

// VisitFromDTO converts a full VisitDTO into a Visit entity, copying id,
// date and description and mapping petId onto the visit's pet reference
// (@Mapping(source = "petId", target = "pet.id")). A nil input returns nil.
//
// MIGRATION_NOTE: the source builds a transient Pet holding only the id. The
// Go Visit models the pet_id foreign key directly as PetID, so the id is
// stored there and Pet is left unloaded; repositories persist PetID. A nil
// date is copied as nil, as MapStruct does.
func VisitFromDTO(dto *VisitDTO) *model.Visit {
	if dto == nil {
		return nil
	}
	v := model.NewVisit()
	v.SetID(copyIntPtr(dto.ID))
	v.SetDate(copyDatePtr(dto.Date))
	v.SetDescription(dto.Description)
	v.PetID = copyIntPtr(dto.PetID)
	return v
}

// VisitFromFieldsDTO converts a VisitFieldsDTO into a Visit entity. The id
// and pet are deliberately left unset, matching
// @Mapping(target = "id", ignore = true) and
// @Mapping(target = "pet", ignore = true) in the source. A nil input
// returns nil.
//
// MIGRATION_NOTE: deliberate deviation (agreed plan): when the DTO carries
// no date, the visit keeps the "today" default from NewVisit instead of
// being set to null as the MapStruct implementation does.
func VisitFromFieldsDTO(dto *VisitFieldsDTO) *model.Visit {
	if dto == nil {
		return nil
	}
	v := model.NewVisit()
	if dto.Date != nil {
		v.SetDate(copyDatePtr(dto.Date))
	}
	v.SetDescription(dto.Description)
	return v
}

// VisitToDTO converts a Visit entity into a VisitDTO, copying id, date and
// description and flattening the pet's id into petId
// (@Mapping(source = "pet.id", target = "petId")). A nil input returns nil.
func VisitToDTO(v *model.Visit) *VisitDTO {
	if v == nil {
		return nil
	}
	return &VisitDTO{
		ID:          copyIntPtr(v.GetID()),
		Date:        copyDatePtr(v.GetDate()),
		Description: v.GetDescription(),
		PetID:       copyIntPtr(v.GetPetID()),
	}
}

// VisitsToDTOs converts a slice of Visit entities into VisitDTOs, mapping
// each element with VisitToDTO. A nil slice returns nil, as MapStruct
// returns null for a null collection; an empty slice returns an empty,
// non-nil slice so it encodes to JSON as []. Nil elements are kept as nil
// entries (encoded as JSON null), as MapStruct does.
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

// copyIntPtr returns a new pointer holding the same value as p, so that the
// entity and the DTO never share the same id storage. It returns nil if p is
// nil.
func copyIntPtr(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// copyDatePtr returns a new pointer holding the same date as p, or nil if p
// is nil.
func copyDatePtr(p *model.Date) *model.Date {
	if p == nil {
		return nil
	}
	d := *p
	return &d
}
