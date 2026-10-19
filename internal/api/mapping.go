// Package api contains the HTTP layer of the petclinic application: DTOs,
// entity<->DTO mappers, validation, problem responses and routing.
//
// This file holds the hand-written replacements for the MapStruct mappers
// PetMapper, PetTypeMapper and VisitMapper.
//
// MIGRATION_NOTE: MapStruct generated these mappers at compile time from
// annotations. In Go they are plain functions. Name-based implicit mapping is
// written out field by field. Collection mapping is written as range loops.
// The source's overloaded toPet(PetDto) / toPet(PetFieldsDto) become
// PetFromDTO and PetFromFieldsDTO.
//
// Behaviour preserved from the generated code:
//   - A nil entity or DTO maps to nil.
//   - Response slices are never nil, so JSON renders [] rather than null.
//   - Nullable scalars (id, birthDate, date, type, ownerId, petId) are
//     pointers. The mapper copies the source value as is, including nil, and
//     never substitutes a constructor default.
//
// REQUIRES MANUAL REVIEW: this file was written without access to the
// model/*.go sources. Check the following accessor names and types against
// internal/model:
//   - Pet.Get/SetBirthDate (*time.Time)
//   - Pet.Get/SetType (*PetType)
//   - Pet.SetOwner(OwnerRef)
//   - Pet.GetOwnerID (*int)
//   - Pet.Get/SetVisits ([]*Visit)
//   - Owner, PetType and Visit fields: ID *int, Name string, Date *time.Time,
//     Description string, Pet *Pet
package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"migrated-app/internal/model"
)

// localDateLayout is the ISO-8601 calendar date layout used by the REST API.
// It corresponds to java.time.LocalDate.
const localDateLayout = "2006-01-02"

// LocalDate is a calendar date without a time zone. In JSON it is
// serialised as "yyyy-MM-dd", matching Jackson's LocalDate handling.
type LocalDate struct {
	time.Time
}

// NewLocalDate truncates t to a calendar date.
func NewLocalDate(t time.Time) LocalDate {
	y, m, d := t.Date()
	return LocalDate{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}

// MarshalJSON renders the date as "yyyy-MM-dd".
func (d LocalDate) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Format(localDateLayout))
}

// UnmarshalJSON parses a "yyyy-MM-dd" date.
func (d *LocalDate) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		return nil
	}
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return fmt.Errorf("local date: %w", err)
	}
	t, err := time.Parse(localDateLayout, str)
	if err != nil {
		return fmt.Errorf("local date %q: %w", str, err)
	}
	d.Time = t
	return nil
}

// toLocalDate converts a nullable time into a nullable LocalDate.
func toLocalDate(t *time.Time) *LocalDate {
	if t == nil {
		return nil
	}
	d := NewLocalDate(*t)
	return &d
}

// fromLocalDate converts a nullable LocalDate into a nullable time.
func fromLocalDate(d *LocalDate) *time.Time {
	if d == nil {
		return nil
	}
	t := d.Time
	return &t
}

// copyIntPtr returns an independent copy of a nullable int.
func copyIntPtr(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// ---------------------------------------------------------------------------
// Pet types
// ---------------------------------------------------------------------------

// PetTypeFieldsDTO holds the editable fields of a pet type.
type PetTypeFieldsDTO struct {
	Name string `json:"name"`
}

// PetTypeDTO is the REST representation of a pet type.
type PetTypeDTO struct {
	Name string `json:"name"`
	ID   *int   `json:"id"`
}

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

// PetTypeToFieldsDTO converts a PetType entity into its editable fields. A
// nil input returns nil.
func PetTypeToFieldsDTO(pt *model.PetType) *PetTypeFieldsDTO {
	if pt == nil {
		return nil
	}
	return &PetTypeFieldsDTO{Name: pt.Name}
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

// PetTypeFromFieldsDTO converts editable pet type fields into a new PetType
// entity with no id. A nil input returns nil.
func PetTypeFromFieldsDTO(dto *PetTypeFieldsDTO) *model.PetType {
	if dto == nil {
		return nil
	}
	pt := &model.PetType{}
	pt.Name = dto.Name
	return pt
}

// PetTypesToDTOs converts a slice of PetType entities into DTOs. The result
// is never nil. Nil elements are skipped.
func PetTypesToDTOs(types []*model.PetType) []PetTypeDTO {
	out := make([]PetTypeDTO, 0, len(types))
	for _, pt := range types {
		if dto := PetTypeToDTO(pt); dto != nil {
			out = append(out, *dto)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Visits
// ---------------------------------------------------------------------------

// VisitFieldsDTO holds the editable fields of a visit.
type VisitFieldsDTO struct {
	Date        *LocalDate `json:"date"`
	Description string     `json:"description"`
}

// VisitDTO is the REST representation of a visit.
type VisitDTO struct {
	ID          *int       `json:"id"`
	Date        *LocalDate `json:"date"`
	Description string     `json:"description"`
	PetID       *int       `json:"petId"`
}

// VisitToDTO converts a Visit entity into a VisitDTO. petId is copied from
// the visit's pet. A nil input returns nil.
func VisitToDTO(v *model.Visit) *VisitDTO {
	if v == nil {
		return nil
	}
	dto := &VisitDTO{
		ID:          copyIntPtr(v.ID),
		Date:        toLocalDate(v.Date),
		Description: v.Description,
	}
	if v.Pet != nil {
		dto.PetID = copyIntPtr(v.Pet.ID)
	}
	return dto
}

// VisitFromDTO converts a VisitDTO into a Visit entity. The pet reference
// carries only the id taken from petId. A nil input returns nil.
func VisitFromDTO(dto *VisitDTO) *model.Visit {
	if dto == nil {
		return nil
	}
	v := &model.Visit{}
	v.ID = copyIntPtr(dto.ID)
	v.Date = fromLocalDate(dto.Date)
	v.Description = dto.Description
	if dto.PetID != nil {
		p := &model.Pet{}
		p.ID = copyIntPtr(dto.PetID)
		v.Pet = p
	}
	return v
}

// VisitFromFieldsDTO converts editable visit fields into a new Visit entity
// with no id and no pet. A nil input returns nil.
func VisitFromFieldsDTO(dto *VisitFieldsDTO) *model.Visit {
	if dto == nil {
		return nil
	}
	v := &model.Visit{}
	v.Date = fromLocalDate(dto.Date)
	v.Description = dto.Description
	return v
}

// VisitsToDTOs converts a slice of Visit entities into DTOs. The result is
// never nil. Nil elements are skipped.
func VisitsToDTOs(visits []*model.Visit) []VisitDTO {
	out := make([]VisitDTO, 0, len(visits))
	for _, v := range visits {
		if dto := VisitToDTO(v); dto != nil {
			out = append(out, *dto)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Pets
// ---------------------------------------------------------------------------

// PetFieldsDTO holds the editable fields of a pet.
type PetFieldsDTO struct {
	Name      string      `json:"name"`
	BirthDate *LocalDate  `json:"birthDate"`
	Type      *PetTypeDTO `json:"type"`
}

// PetDTO is the REST representation of a pet.
type PetDTO struct {
	Name      string      `json:"name"`
	BirthDate *LocalDate  `json:"birthDate"`
	Type      *PetTypeDTO `json:"type"`
	ID        *int        `json:"id"`
	OwnerID   *int        `json:"ownerId"`
	Visits    []VisitDTO  `json:"visits"`
}

// PetPageDTO is a page of pets plus pagination metadata.
type PetPageDTO struct {
	Content       []PetDTO `json:"content"`
	Page          int      `json:"page"`
	Size          int      `json:"size"`
	TotalElements int64    `json:"totalElements"`
	TotalPages    int      `json:"totalPages"`
}

// sortedVisitsDesc returns a copy of visits ordered by date, newest first.
// Visits without a date go last. This matches Pet.getVisits() in the source.
func sortedVisitsDesc(visits []*model.Visit) []*model.Visit {
	out := make([]*model.Visit, 0, len(visits))
	for _, v := range visits {
		if v != nil {
			out = append(out, v)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		di, dj := out[i].Date, out[j].Date
		switch {
		case di == nil:
			return false
		case dj == nil:
			return true
		default:
			return di.After(*dj)
		}
	})
	return out
}

// PetToDTO converts a Pet entity into a PetDTO. It copies the scalar fields,
// maps the pet type, maps the visits (newest first) and sets ownerId from the
// owner's id. A nil input returns nil.
func PetToDTO(p *model.Pet) *PetDTO {
	if p == nil {
		return nil
	}
	return &PetDTO{
		ID:        copyIntPtr(p.ID),
		Name:      p.Name,
		BirthDate: toLocalDate(p.GetBirthDate()),
		Type:      PetTypeToDTO(p.GetType()),
		OwnerID:   copyIntPtr(p.GetOwnerID()),
		Visits:    VisitsToDTOs(sortedVisitsDesc(p.GetVisits())),
	}
}

// PetsToDTOs converts a slice of Pet entities into DTOs. The result is never
// nil. Nil elements are skipped.
func PetsToDTOs(pets []*model.Pet) []PetDTO {
	out := make([]PetDTO, 0, len(pets))
	for _, p := range pets {
		if dto := PetToDTO(p); dto != nil {
			out = append(out, *dto)
		}
	}
	return out
}

// PetFromDTO converts a PetDTO into a Pet entity. When ownerId is set, it
// attaches an owner reference that carries only that id. Visits are mapped
// through VisitFromDTO. A nil input returns nil.
//
// MIGRATION_NOTE: MapStruct always created the nested owner for
// target="owner.id". Here the owner is created only when ownerId is non-nil,
// which avoids an empty owner with a null id. This does not change any
// observable behaviour.
func PetFromDTO(dto *PetDTO) *model.Pet {
	if dto == nil {
		return nil
	}
	p := &model.Pet{}
	p.ID = copyIntPtr(dto.ID)
	p.Name = dto.Name
	p.SetBirthDate(fromLocalDate(dto.BirthDate))
	p.SetType(PetTypeFromDTO(dto.Type))
	if dto.OwnerID != nil {
		owner := &model.Owner{}
		owner.ID = copyIntPtr(dto.OwnerID)
		p.SetOwner(owner)
	}
	visits := make([]*model.Visit, 0, len(dto.Visits))
	for i := range dto.Visits {
		if v := VisitFromDTO(&dto.Visits[i]); v != nil {
			visits = append(visits, v)
		}
	}
	p.SetVisits(visits)
	return p
}

// PetsFromDTOs converts a slice of PetDTOs into Pet entities. The result is
// never nil.
func PetsFromDTOs(dtos []PetDTO) []*model.Pet {
	out := make([]*model.Pet, 0, len(dtos))
	for i := range dtos {
		if p := PetFromDTO(&dtos[i]); p != nil {
			out = append(out, p)
		}
	}
	return out
}

// PetFromFieldsDTO converts editable pet fields into a new Pet entity. The
// id, owner and visits are left unset. A nil input returns nil.
func PetFromFieldsDTO(dto *PetFieldsDTO) *model.Pet {
	if dto == nil {
		return nil
	}
	p := &model.Pet{}
	p.Name = dto.Name
	p.SetBirthDate(fromLocalDate(dto.BirthDate))
	p.SetType(PetTypeFromDTO(dto.Type))
	return p
}

// PetPageToDTO builds a PetPageDTO from one page of pets and its pagination
// metadata. page is zero-based, as in Spring Data's Page.getNumber().
//
// MIGRATION_NOTE: the source took a Spring Data Page<Pet>. Here the caller
// passes the page's content and metadata explicitly, so this package does not
// depend on any particular paging type in the store layer.
func PetPageToDTO(content []*model.Pet, page, size int, totalElements int64, totalPages int) PetPageDTO {
	return PetPageDTO{
		Content:       PetsToDTOs(content),
		Page:          page,
		Size:          size,
		TotalElements: totalElements,
		TotalPages:    totalPages,
	}
}
