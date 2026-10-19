package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func spec(id *int, name string) Specialty {
	s := Specialty{}
	s.ID = id
	s.Name = name
	return s
}

func names(ss []Specialty) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.Name
	}
	return out
}

func TestTableConstants(t *testing.T) {
	if VetsTable != "vets" {
		t.Errorf("VetsTable = %q", VetsTable)
	}
	if VetSpecialtiesTable != "vet_specialties" {
		t.Errorf("VetSpecialtiesTable = %q", VetSpecialtiesTable)
	}
}

func TestGetSpecialties(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *Vet
		want  []string
	}{
		{"nil vet", func() *Vet { return nil }, []string{}},
		{"new vet", NewVet, []string{}},
		{"zero value vet", func() *Vet { return &Vet{} }, []string{}},
		{"sorted case-insensitive", func() *Vet {
			v := NewVet()
			v.AddSpecialty(spec(IntPtr(1), "surgery"))
			v.AddSpecialty(spec(IntPtr(2), "Dentistry"))
			v.AddSpecialty(spec(IntPtr(3), "radiology"))
			return v
		}, []string{"Dentistry", "radiology", "surgery"}},
		{"empty names do not panic", func() *Vet {
			v := NewVet()
			v.AddSpecialty(spec(IntPtr(1), "b"))
			v.AddSpecialty(spec(IntPtr(2), ""))
			return v
		}, []string{"", "b"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := tc.setup()
			got := v.GetSpecialties()
			if got == nil {
				t.Fatal("GetSpecialties returned nil")
			}
			if !reflect.DeepEqual(names(got), tc.want) {
				t.Errorf("got %v, want %v", names(got), tc.want)
			}
			if len(got) != v.NrOfSpecialties() {
				t.Errorf("len %d != NrOfSpecialties %d", len(got), v.NrOfSpecialties())
			}
		})
	}
}

func TestGetSpecialtiesIsCopyAndSnapshot(t *testing.T) {
	v := NewVet()
	v.AddSpecialty(spec(IntPtr(1), "surgery"))
	got := v.GetSpecialties()
	got[0].Name = "changed"
	_ = append(got, spec(IntPtr(9), "x"))
	if v.GetSpecialties()[0].Name != "surgery" || v.NrOfSpecialties() != 1 {
		t.Error("modifying returned slice changed vet")
	}
	v.AddSpecialty(spec(IntPtr(2), "dentistry"))
	if len(got) != 1 {
		t.Error("snapshot reflected later change")
	}
}

func TestSetSpecialties(t *testing.T) {
	tests := []struct {
		name  string
		input []Specialty
		want  []string
	}{
		{"duplicates collapsed", []Specialty{spec(IntPtr(1), "a"), spec(IntPtr(1), "a"), spec(IntPtr(2), "b")}, []string{"a", "b"}},
		{"empty list", []Specialty{}, []string{}},
		{"nil list", nil, []string{}},
		{"unpersisted not deduped", []Specialty{spec(nil, "a"), spec(nil, "a")}, []string{"a", "a"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := NewVet()
			v.AddSpecialty(spec(IntPtr(99), "old"))
			v.SetSpecialties(tc.input)
			if !reflect.DeepEqual(names(v.GetSpecialties()), tc.want) {
				t.Errorf("got %v, want %v", names(v.GetSpecialties()), tc.want)
			}
		})
	}
}

func TestSetSpecialtiesInputMutationIsolated(t *testing.T) {
	in := []Specialty{spec(IntPtr(1), "a")}
	v := NewVet()
	v.SetSpecialties(in)
	in[0].Name = "z"
	if v.GetSpecialties()[0].Name != "a" {
		t.Error("input mutation affected vet")
	}
}

func TestNrOfSpecialties(t *testing.T) {
	var nilVet *Vet
	if nilVet.NrOfSpecialties() != 0 {
		t.Error("nil vet should be 0")
	}
	v := NewVet()
	if v.NrOfSpecialties() != 0 {
		t.Error("new vet should be 0")
	}
	v.AddSpecialty(spec(IntPtr(1), "a"))
	v.AddSpecialty(spec(IntPtr(2), "b"))
	if v.NrOfSpecialties() != 2 {
		t.Errorf("got %d", v.NrOfSpecialties())
	}
}

func TestAddSpecialty(t *testing.T) {
	tests := []struct {
		name      string
		existing  []Specialty
		add       Specialty
		wantCount int
	}{
		{"new to empty", nil, spec(IntPtr(1), "a"), 1},
		{"new distinct", []Specialty{spec(IntPtr(1), "a")}, spec(IntPtr(2), "b"), 2},
		{"duplicate id", []Specialty{spec(IntPtr(1), "a")}, spec(IntPtr(1), "a"), 1},
		{"unpersisted added", []Specialty{spec(nil, "a")}, spec(nil, "a"), 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := &Vet{}
			for _, s := range tc.existing {
				v.AddSpecialty(s)
			}
			v.AddSpecialty(tc.add)
			if v.NrOfSpecialties() != tc.wantCount {
				t.Errorf("count %d, want %d", v.NrOfSpecialties(), tc.wantCount)
			}
			found := false
			for _, s := range v.GetSpecialties() {
				if s.Name == tc.add.Name {
					found = true
				}
			}
			if !found {
				t.Error("added specialty not contained")
			}
		})
	}
}

func TestClearSpecialties(t *testing.T) {
	tests := []struct {
		name string
		v    *Vet
	}{
		{"with specialties", func() *Vet {
			v := NewVet()
			v.AddSpecialty(spec(IntPtr(1), "a"))
			v.AddSpecialty(spec(IntPtr(2), "b"))
			return v
		}()},
		{"empty", NewVet()},
		{"zero value", &Vet{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.v.ClearSpecialties()
			if tc.v.NrOfSpecialties() != 0 || len(tc.v.GetSpecialties()) != 0 {
				t.Error("not cleared")
			}
			tc.v.AddSpecialty(spec(IntPtr(3), "c"))
			if tc.v.NrOfSpecialties() != 1 {
				t.Error("add after clear failed")
			}
		})
	}
}

func TestMarshalJSON(t *testing.T) {
	v := NewVet()
	v.ID = IntPtr(5)
	v.FirstName = "James"
	v.LastName = "Carter"
	v.AddSpecialty(spec(IntPtr(1), "surgery"))
	v.AddSpecialty(spec(IntPtr(2), "Dentistry"))
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"nrOfSpecialties", "specialtiesInternal", "NrOfSpecialties"} {
		if _, ok := m[k]; ok {
			t.Errorf("unexpected key %s", k)
		}
	}
	if m["id"].(float64) != 5 || m["firstName"] != "James" || m["lastName"] != "Carter" {
		t.Errorf("bad fields: %s", b)
	}
	sp := m["specialties"].([]any)
	if len(sp) != 2 || sp[0].(map[string]any)["name"] != "Dentistry" {
		t.Errorf("bad specialties: %s", b)
	}

	empty, _ := json.Marshal(NewVet())
	if !strings.Contains(string(empty), `"specialties":[]`) || !strings.Contains(string(empty), `"id":null`) {
		t.Errorf("empty vet json: %s", empty)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
		want    []string
	}{
		{"full", `{"id":3,"firstName":"A","lastName":"B","specialties":[{"id":2,"name":"x"},{"id":2,"name":"x"},{"id":1,"name":"W"}]}`, false, []string{"W", "x"}},
		{"no specialties", `{"id":3,"firstName":"A","lastName":"B"}`, false, []string{}},
		{"invalid", `{bad`, true, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var v Vet
			err := json.Unmarshal([]byte(tc.in), &v)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if v.ID == nil || *v.ID != 3 || v.FirstName != "A" || v.LastName != "B" {
				t.Errorf("bad fields %+v", v.Person)
			}
			if !reflect.DeepEqual(names(v.GetSpecialties()), tc.want) {
				t.Errorf("got %v", names(v.GetSpecialties()))
			}
		})
	}
}

func TestVetPromotedPersonMethods(t *testing.T) {
	v := NewVet()
	if !v.IsNew() {
		t.Error("new vet should be new")
	}
	if v.Validate() == nil {
		t.Error("expected validation error")
	}
	v.SetFirstName("a")
	v.SetLastName("b")
	if v.Validate() != nil {
		t.Error("expected valid")
	}
}