package api

// This file is the Go port of the MapStruct VetMapper interface
// (org.springframework.samples.petclinic.mapper.VetMapper) and of the REST
// DTOs it converts between (VetDto and VetFieldsDto).
//
// MIGRATION_NOTE: MapStruct generated VetMapperImpl at compile time, matching
// properties by name. Here every mapping is a hand-written function, so each
// field copy is explicit. Go has no method overloading, so the two Java
// toVet(VetDto) and toVet(VetFieldsDto) methods become VetFromDTO and
// VetFromFieldsDTO. @Mapper(uses = SpecialtyMapper.class) becomes calls to
// SpecialtiesToDTOs and SpecialtiesFromDTOs (specialty_mapping.go), which work
// on pointer slices. The value/pointer slice adaptation is done in the
// unexported helpers at the bottom of this file. MapStruct returned null for
// null input, and these functions return nil for nil input.
//
// Type contracts relied on (from internal/model):
//   - model.Vet.ID is *int (nil means "not persisted yet")
//   - model.Vet.GetSpecialties() []model.Specialty (sorted by name, as in the source)
//   - model.Vet.SetSpecialties([]model.Specialty)

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
		// Copy the value so the entity does not alias the DTO's memory.
		id := *dto.ID
		vet.ID = &id
	}
	vet.FirstName = dto.FirstName
	vet.LastName = dto.LastName
	if dto.Specialties != nil {
		vet.SetSpecialties(specialtiesFromDTOValues(dto.Specialties))
	}
	return vet
}

// VetFromFieldsDTO converts a VetFieldsDTO into a new Vet entity. The id is
// deliberately left unset (nil), matching @Mapping(target = "id", ignore = true)
// in the source. A nil input returns nil.
func VetFromFieldsDTO(dto *VetFieldsDTO) *model.Vet {
	if dto == nil {
		return nil
	}
	vet := model.NewVet()
	// The id is intentionally not mapped and stays nil (new entity).
	vet.FirstName = dto.FirstName
	vet.LastName = dto.LastName
	if dto.Specialties != nil {
		vet.SetSpecialties(specialtiesFromDTOValues(dto.Specialties))
	}
	return vet
}

// VetToDTO converts a Vet entity into a VetDTO, including id, firstName,
// lastName and specialties. A nil input returns nil.
func VetToDTO(vet *model.Vet) *VetDTO {
	if vet == nil {
		return nil
	}
	dto := &VetDTO{
		FirstName:   vet.FirstName,
		LastName:    vet.LastName,
		Specialties: specialtiesToDTOValues(vet.GetSpecialties()),
	}
	if vet.ID != nil {
		// Copy the value so the DTO does not alias the entity's memory.
		id := *vet.ID
		dto.ID = &id
	}
	return dto
}

// VetsToDTOs converts a slice of Vet entities into VetDTOs, element by element
// via VetToDTO, keeping the input order and length. A nil input returns nil.
// A nil element produces a nil entry, exactly as MapStruct added a null entry
// for a null element.
func VetsToDTOs(vets []*model.Vet) []*VetDTO {
	if vets == nil {
		return nil
	}
	dtos := make([]*VetDTO, 0, len(vets))
	for _, vet := range vets {
		dtos = append(dtos, VetToDTO(vet))
	}
	return dtos
}

// specialtiesFromDTOValues converts the value-typed DTO slice used in the vet
// DTOs into the value-typed entity slice that model.Vet.SetSpecialties takes,
// delegating each element to the shared specialty mapping.
func specialtiesFromDTOValues(dtos []SpecialtyDTO) []model.Specialty {
	if dtos == nil {
		return nil
	}
	ptrs := make([]*SpecialtyDTO, 0, len(dtos))
	for i := range dtos {
		ptrs = append(ptrs, &dtos[i])
	}
	converted := SpecialtiesFromDTOs(ptrs)
	specialties := make([]model.Specialty, 0, len(converted))
	for _, s := range converted {
		if s == nil {
			// Cannot happen for non-nil input. A Vet's specialty set holds no
			// null entries, so this one is skipped.
			continue
		}
		specialties = append(specialties, *s)
	}
	return specialties
}

// specialtiesToDTOValues converts the value-typed entity slice returned by
// model.Vet.GetSpecialties into the value-typed DTO slice used in VetDTO,
// delegating each element to the shared specialty mapping. It always returns a
// non-nil slice, so JSON shows [] rather than null. This matches the Java
// getter, which never returned null.
func specialtiesToDTOValues(specialties []model.Specialty) []SpecialtyDTO {
	ptrs := make([]*model.Specialty, 0, len(specialties))
	for i := range specialties {
		ptrs = append(ptrs, &specialties[i])
	}
	converted := SpecialtiesToDTOs(ptrs)
	dtos := make([]SpecialtyDTO, 0, len(converted))
	for _, d := range converted {
		if d == nil {
			continue
		}
		dtos = append(dtos, *d)
	}
	return dtos
}
