package api

import (
	"encoding/json"
	"testing"

	"migrated-app/internal/model"
)

func specTestIntPtr(v int) *int { return &v }

func TestSpecialtyFromDTONil(t *testing.T) {
	if SpecialtyFromDTO(nil) != nil {
		t.Fatal("expected nil")
	}
}

func TestSpecialtyFromDTOCopiesFields(t *testing.T) {
	dto := &SpecialtyDTO{ID: specTestIntPtr(1), Name: "radiology"}
	s := SpecialtyFromDTO(dto)
	if s == nil {
		t.Fatal("unexpected nil")
	}
	if s.GetID() == nil || *s.GetID() != 1 {
		t.Errorf("id = %v, want 1", s.GetID())
	}
	if s.GetName() != "radiology" {
		t.Errorf("name = %q", s.GetName())
	}
	if s.GetID() == dto.ID {
		t.Error("id pointer shared")
	}
}

func TestSpecialtyToDTO(t *testing.T) {
	if SpecialtyToDTO(nil) != nil {
		t.Fatal("expected nil")
	}
	s := model.NewSpecialty()
	s.SetID(specTestIntPtr(2))
	s.SetName("surgery")
	dto := SpecialtyToDTO(s)
	if dto == nil || dto.ID == nil || *dto.ID != 2 || dto.Name != "surgery" {
		t.Fatalf("got %+v", dto)
	}
	*dto.ID = 99
	if *s.GetID() != 2 {
		t.Error("mutating DTO changed entity")
	}
}

func TestSpecialtyToDTONilID(t *testing.T) {
	s := model.NewSpecialty()
	s.SetName("x")
	dto := SpecialtyToDTO(s)
	if dto.ID != nil {
		t.Errorf("expected nil id")
	}
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"id":null,"name":"x"}` {
		t.Errorf("json = %s", b)
	}
}

func TestSpecialtiesToDTOs(t *testing.T) {
	if SpecialtiesToDTOs(nil) != nil {
		t.Error("expected nil")
	}
	empty := SpecialtiesToDTOs([]*model.Specialty{})
	if empty == nil || len(empty) != 0 {
		t.Error("expected empty non-nil")
	}
	b, _ := json.Marshal(empty)
	if string(b) != "[]" {
		t.Errorf("empty encodes as %s", b)
	}
	s := model.NewSpecialty()
	s.SetName("a")
	got := SpecialtiesToDTOs([]*model.Specialty{s, nil})
	if len(got) != 2 || got[0] == nil || got[0].Name != "a" || got[1] != nil {
		t.Errorf("got %+v", got)
	}
}

func TestSpecialtiesFromDTOs(t *testing.T) {
	if SpecialtiesFromDTOs(nil) != nil {
		t.Error("expected nil")
	}
	got := SpecialtiesFromDTOs([]*SpecialtyDTO{nil, {ID: specTestIntPtr(3), Name: "b"}})
	if len(got) != 2 || got[0] != nil || got[1] == nil {
		t.Fatalf("got %+v", got)
	}
	if got[1].GetName() != "b" || got[1].GetID() == nil || *got[1].GetID() != 3 {
		t.Errorf("bad element")
	}
}