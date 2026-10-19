package api

import (
	"testing"

	"migrated-app/internal/model"
)

func mapIntPtr(v int) *int { return &v }

func mapNewPT(id *int, name string) *model.PetType {
	pt := model.NewPetType()
	pt.SetID(id)
	pt.SetName(name)
	return pt
}

func TestPetTypeFromDTO(t *testing.T) {
	if PetTypeFromDTO(nil) != nil {
		t.Fatal("expected nil")
	}
	in := &PetTypeDTO{ID: mapIntPtr(1), Name: "cat"}
	got := PetTypeFromDTO(in)
	if got == nil || got.GetID() == nil || *got.GetID() != 1 || got.GetName() != "cat" {
		t.Fatalf("unexpected result %+v", got)
	}
	if got.GetID() == in.ID {
		t.Error("id pointer shared")
	}
	got2 := PetTypeFromDTO(&PetTypeDTO{Name: "dog"})
	if got2.GetID() != nil || got2.GetName() != "dog" {
		t.Errorf("unexpected %+v", got2)
	}
}

func TestPetTypeFromFieldsDTO(t *testing.T) {
	if PetTypeFromFieldsDTO(nil) != nil {
		t.Fatal("expected nil")
	}
	got := PetTypeFromFieldsDTO(&PetTypeFieldsDTO{Name: "hamster"})
	if got == nil || got.GetID() != nil || got.GetName() != "hamster" {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestPetTypeToDTO(t *testing.T) {
	if PetTypeToDTO(nil) != nil {
		t.Fatal("expected nil")
	}
	pt := mapNewPT(mapIntPtr(2), "dog")
	got := PetTypeToDTO(pt)
	if got == nil || got.ID == nil || *got.ID != 2 || got.Name != "dog" {
		t.Fatalf("unexpected %+v", got)
	}
	*got.ID = 999
	if *pt.GetID() != 2 {
		t.Error("DTO shares id storage with entity")
	}
}

func TestPetTypeToFieldsDTO(t *testing.T) {
	if PetTypeToFieldsDTO(nil) != nil {
		t.Fatal("expected nil")
	}
	got := PetTypeToFieldsDTO(mapNewPT(mapIntPtr(3), "bird"))
	if got == nil || got.Name != "bird" {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestPetTypesToDTOs(t *testing.T) {
	if PetTypesToDTOs(nil) != nil {
		t.Fatal("expected nil")
	}
	empty := PetTypesToDTOs([]*model.PetType{})
	if empty == nil || len(empty) != 0 {
		t.Fatal("expected empty non-nil slice")
	}
	got := PetTypesToDTOs([]*model.PetType{mapNewPT(mapIntPtr(1), "cat"), nil, mapNewPT(nil, "dog")})
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].ID == nil || *got[0].ID != 1 || got[0].Name != "cat" {
		t.Errorf("bad first %+v", got[0])
	}
	if got[1].ID != nil || got[1].Name != "dog" {
		t.Errorf("bad second %+v", got[1])
	}
}