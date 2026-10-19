package api

// This file is the port of the MapStruct PetTypeMapper. It converts between
// the PetType domain entity and its REST DTOs.
//
// MIGRATION_NOTE: MapStruct builds the mapper implementation at compile
// time and Spring injects it as a bean. In Go these are plain, stateless
// package-level functions, so handlers call them directly and nothing needs
// to be wired up. Java resolves the two overloaded toPetType methods by
// parameter type. Go has no overloading, so they become PetTypeFromDTO and
// PetTypeFromFieldsDTO.

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
