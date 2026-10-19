package model

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestSpecialtiesTable(t *testing.T) {
	if SpecialtiesTable != "specialties" {
		t.Fatalf("SpecialtiesTable = %q, want %q", SpecialtiesTable, "specialties")
	}
}

func TestNewSpecialty(t *testing.T) {
	s := NewSpecialty()
	if s == nil {
		t.Fatal("NewSpecialty returned nil")
	}
	if s.GetID() != nil {
		t.Errorf("GetID() = %v, want nil", s.GetID())
	}
	if s.GetName() != "" {
		t.Errorf("GetName() = %q, want empty", s.GetName())
	}
	if !s.IsNew() {
		t.Error("IsNew() = false, want true")
	}
}

func TestSpecialtyNameAndID(t *testing.T) {
	s := NewSpecialty()
	s.SetName("radiology")
	if s.GetName() != "radiology" || s.String() != "radiology" {
		t.Errorf("name = %q / %q", s.GetName(), s.String())
	}
	s.SetID(IntPtr(0))
	if s.IsNew() {
		t.Error("IsNew() with id 0 = true, want false")
	}
	if id := s.GetID(); id == nil || *id != 0 {
		t.Errorf("GetID() = %v, want 0", id)
	}
	s.SetID(nil)
	if !s.IsNew() {
		t.Error("IsNew() after nil id = false")
	}
}

func TestSpecialtyValidate(t *testing.T) {
	s := NewSpecialty()
	if err := s.Validate(); !errors.Is(err, ErrNameEmpty) {
		t.Errorf("Validate() = %v, want ErrNameEmpty", err)
	}
	s.SetName(" ")
	if err := s.Validate(); err != nil {
		t.Errorf("Validate() whitespace = %v, want nil", err)
	}
}

func TestSpecialtyJSON(t *testing.T) {
	s := NewSpecialty()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"id":null,"name":""}` {
		t.Errorf("json = %s", b)
	}
	var got Specialty
	if err := json.Unmarshal([]byte(`{"id":7,"name":"dentistry"}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.GetID() == nil || *got.GetID() != 7 || got.GetName() != "dentistry" {
		t.Errorf("unmarshal got %v %q", got.GetID(), got.GetName())
	}
}