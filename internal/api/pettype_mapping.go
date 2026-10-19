package api

// This file is the port of the MapStruct PetTypeMapper. It converts between
// the PetType domain entity and its REST DTOs.

import "migrated-app/internal/model"

// PetTypeFieldsDTO carries the editable fields of a pet type
// (the "PetTypeFields" schema).
type PetTypeFieldsDTO struct {
	Name string `json:"name"`
}

// PetTypeDTO is a pet type as exposed by the REST API (the "PetType" schema).
type PetTypeDTO struct {
	ID   *int   `json:"id"`
	Name string `json:"name"`
}

// PetTypeFromDTO converts a PetTypeDTO into a PetType entity. Nil in, nil out.
func PetTypeFromDTO(dto *PetTypeDTO) *model.PetType {
	if dto == nil {
		return nil
	}
	pt := model.NewPetType()
	pt.SetID(copyIntPtr(dto.ID))
	pt.SetName(dto.Name)
	return pt
}

// PetTypeFromFieldsDTO converts a PetTypeFieldsDTO into a new PetType entity
// without an id. Nil in, nil out.
func PetTypeFromFieldsDTO(dto *PetTypeFieldsDTO) *model.PetType {
	if dto == nil {
		return nil
	}
	pt := model.NewPetType()
	pt.SetName(dto.Name)
	return pt
}

// PetTypeToDTO converts a PetType entity into a PetTypeDTO. Nil in, nil out.
func PetTypeToDTO(pt *model.PetType) *PetTypeDTO {
	if pt == nil {
		return nil
	}
	return &PetTypeDTO{ID: copyIntPtr(pt.GetID()), Name: pt.GetName()}
}

// PetTypeToFieldsDTO converts a PetType entity into a PetTypeFieldsDTO.
func PetTypeToFieldsDTO(pt *model.PetType) *PetTypeFieldsDTO {
	if pt == nil {
		return nil
	}
	return &PetTypeFieldsDTO{Name: pt.GetName()}
}

// PetTypesToDTOs converts a slice of PetType entities. A nil slice yields
// nil; nil elements are skipped.
func PetTypesToDTOs(pts []*model.PetType) []*PetTypeDTO {
	if pts == nil {
		return nil
	}
	out := make([]*PetTypeDTO, 0, len(pts))
	for _, pt := range pts {
		if pt == nil {
			continue
		}
		out = append(out, PetTypeToDTO(pt))
	}
	return out
}

// copyIntPtr returns an independent copy of p; nil stays nil.
func copyIntPtr(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
