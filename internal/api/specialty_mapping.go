package api

// This file is the port of the MapStruct SpecialtyMapper. It converts
// between the Specialty domain entity and its REST DTO.
//
// MIGRATION_NOTE: MapStruct builds the mapper implementation at compile
// time and Spring injects it as a bean. In Go these are plain, stateless
// package-level functions that handlers call directly.

import "migrated-app/internal/model"

// SpecialtyDTO is a vet specialty as the REST API exposes it: the read-only
// id plus the name. It is the "Specialty" schema in openapi.yml.
//
// MIGRATION_NOTE: Hand-written replacement for the OpenAPI-generated
// SpecialtyDto.
type SpecialtyDTO struct {
	ID   *int   `json:"id"`
	Name string `json:"name"`
}

// SpecialtyFromDTO converts a SpecialtyDTO into a Specialty entity, copying
// id and name (MapStruct toSpecialty). A nil input returns nil.
func SpecialtyFromDTO(dto *SpecialtyDTO) *model.Specialty {
	if dto == nil {
		return nil
	}
	s := model.NewSpecialty()
	s.SetID(copyIntPtr(dto.ID))
	s.SetName(dto.Name)
	return s
}

// SpecialtyToDTO converts a Specialty entity into a SpecialtyDTO, copying id
// and name (MapStruct toSpecialtyDto). A nil input returns nil.
func SpecialtyToDTO(s *model.Specialty) *SpecialtyDTO {
	if s == nil {
		return nil
	}
	return &SpecialtyDTO{
		ID:   copyIntPtr(s.GetID()),
		Name: s.GetName(),
	}
}

// SpecialtiesToDTOs converts a slice of Specialty entities element-wise with
// SpecialtyToDTO (MapStruct toSpecialtyDtos). A nil slice returns nil; an
// empty slice returns an empty, non-nil slice (encodes as []). The result
// always has the same length as the input: nil elements map to nil entries,
// exactly as the MapStruct-generated loop does.
func SpecialtiesToDTOs(specialties []*model.Specialty) []*SpecialtyDTO {
	if specialties == nil {
		return nil
	}
	out := make([]*SpecialtyDTO, len(specialties))
	for i, s := range specialties {
		out[i] = SpecialtyToDTO(s)
	}
	return out
}

// SpecialtiesFromDTOs converts a slice of SpecialtyDTOs element-wise with
// SpecialtyFromDTO (MapStruct toSpecialtys). A nil slice returns nil; nil
// elements map to nil entries, so the result has the input's length.
func SpecialtiesFromDTOs(dtos []*SpecialtyDTO) []*model.Specialty {
	if dtos == nil {
		return nil
	}
	out := make([]*model.Specialty, len(dtos))
	for i, dto := range dtos {
		out[i] = SpecialtyFromDTO(dto)
	}
	return out
}
