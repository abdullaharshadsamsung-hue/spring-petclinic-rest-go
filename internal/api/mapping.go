package api

// This file is the Go port of the MapStruct VetMapper interface
// (org.springframework.samples.petclinic.mapper.VetMapper) and of the REST
// DTOs it converts between (VetDto and VetFieldsDto).
//
// MIGRATION_NOTE: MapStruct generated VetMapperImpl at compile time, matching
// properties by name. Here every mapping is a hand-written function, so each
// field copy is explicit. The overloaded Java toVet(VetDto) and
// toVet(VetFieldsDto) methods become VetFromDTO and VetFromFieldsDTO, because
// Go has no method overloading. @Mapper(uses = SpecialtyMapper.class) becomes
// direct calls to SpecialtiesToDTOs and SpecialtiesFromDTOs in
// specialty_mapping.go. MapStruct returned null for null input, and these
// functions return nil for nil input.
//
// MANUAL REVIEW: these functions assume model.Vet exposes ID (int),
// FirstName and LastName through its embedded Person/BaseEntity, and the
// GetSpecialties/SetSpecialties accessors, matching internal/model/vet.go.
// model.Vet.GetSpecialties keeps the source's ordering contract: specialties
// are sorted by name.

import (
	"migrated-app/internal/model"
)

// VetFieldsDTO is the editable part of a vet, without the id. It mirrors the
// OpenAPI VetFields schema.
type VetFieldsDTO struct {
	// FirstName is the vet's first name.
	FirstName string `json:"firstName"`
	// LastName is the vet's last name.
	LastName string `json:"lastName"`
	// Specialties lists the specialties of the vet.
	Specialties []SpecialtyDTO `json:"specialties"`
}

// VetDTO is the REST representation of a vet, including its id. It mirrors
// the OpenAPI Vet schema.
type VetDTO struct {
	// ID is the vet's identifier. It is nil when the vet has not been persisted.
	ID *int `json:"id,omitempty"`
	// FirstName is the vet's first name.
	FirstName string `json:"firstName"`
	// LastName is the vet's last name.
	LastName string `json:"lastName"`
	// Specialties lists the specialties of the vet.
	Specialties []SpecialtyDTO `json:"specialties"`
}

// VetFromDTO converts a VetDTO, including its id, into a Vet entity. It maps
// id, firstName, lastName and specialties. A nil input returns nil.
func VetFromDTO(dto *VetDTO) *model.Vet {
	if dto == nil {
		return nil
	}
	vet := model.NewVet()
	if dto.ID != nil {
		vet.ID = *dto.ID
	}
	vet.FirstName = dto.FirstName
	vet.LastName = dto.LastName
	if dto.Specialties != nil {
		vet.SetSpecialties(SpecialtiesFromDTOs(dto.Specialties))
	}
	return vet
}

// VetFromFieldsDTO converts a VetFieldsDTO into a new Vet entity. The id is
// deliberately left unset, matching @Mapping(target = "id", ignore = true) in
// the source. A nil input returns nil.
func VetFromFieldsDTO(dto *VetFieldsDTO) *model.Vet {
	if dto == nil {
		return nil
	}
	vet := model.NewVet()
	// The id is intentionally not mapped and keeps its zero (new-entity) value.
	vet.FirstName = dto.FirstName
	vet.LastName = dto.LastName
	if dto.Specialties != nil {
		vet.SetSpecialties(SpecialtiesFromDTOs(dto.Specialties))
	}
	return vet
}

// VetToDTO converts a Vet entity into a VetDTO, including id, firstName,
// lastName and specialties. A nil input returns nil.
func VetToDTO(vet *model.Vet) *VetDTO {
	if vet == nil {
		return nil
	}
	id := vet.ID
	specialties := SpecialtiesToDTOs(vet.GetSpecialties())
	if specialties == nil {
		// The JSON body shows an empty array rather than null, as the
		// Jackson-serialized Java DTO did.
		specialties = []SpecialtyDTO{}
	}
	return &VetDTO{
		ID:          &id,
		FirstName:   vet.FirstName,
		LastName:    vet.LastName,
		Specialties: specialties,
	}
}

// VetsToDTOs converts a slice of Vet entities into VetDTOs, element by element
// via VetToDTO, keeping the input order. A nil input returns nil. Nil elements
// are skipped.
func VetsToDTOs(vets []*model.Vet) []VetDTO {
	if vets == nil {
		return nil
	}
	dtos := make([]VetDTO, 0, len(vets))
	for _, vet := range vets {
		dto := VetToDTO(vet)
		if dto == nil {
			continue
		}
		dtos = append(dtos, *dto)
	}
	return dtos
}
