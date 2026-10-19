package api

import (
	"encoding/json"
	"reflect"
	"testing"

	"migrated-app/internal/model"
)

func intPtr(i int) *int { return &i }

type wireSpecialty struct {
	ID   *int   `json:"id"`
	Name string `json:"name"`
}

type wireVet struct {
	ID          *int            `json:"id"`
	FirstName   string          `json:"firstName"`
	LastName    string          `json:"lastName"`
	Specialties []wireSpecialty `json:"specialties"`
}

// vetWire inspects a Vet through its JSON form so the tests do not depend on
// the concrete type of the embedded ID field.
func vetWire(t *testing.T, v *model.Vet) wireVet {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal vet: %v", err)
	}
	var w wireVet
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatalf("unmarshal vet: %v", err)
	}
	return w
}

func buildVet(t *testing.T, w wireVet) *model.Vet {
	t.Helper()
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal wire: %v", err)
	}
	v := model.NewVet()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("unmarshal into vet: %v", err)
	}
	return v
}

func idVal(p *int) int {
	if p == nil {
		return -1
	}
	return *p
}

func TestVetFromDTO(t *testing.T) {
	if got := VetFromDTO(nil); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}

	tests := []struct {
		name      string
		dto       VetDTO
		wantID    int
		wantFirst string
		wantLast  string
		wantSpecs []wireSpecialty
	}{
		{
			name:      "no specialties",
			dto:       VetDTO{ID: intPtr(1), FirstName: "James", LastName: "Carter"},
			wantID:    1,
			wantFirst: "James",
			wantLast:  "Carter",
		},
		{
			name: "with specialties",
			dto: VetDTO{ID: intPtr(3), FirstName: "Linda", LastName: "Douglas", Specialties: []SpecialtyDTO{
				{ID: intPtr(2), Name: "surgery"}, {ID: intPtr(1), Name: "radiology"},
			}},
			wantID:    3,
			wantFirst: "Linda",
			wantLast:  "Douglas",
			wantSpecs: []wireSpecialty{{ID: intPtr(1), Name: "radiology"}, {ID: intPtr(2), Name: "surgery"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.dto
			beforeSpecs := append([]SpecialtyDTO(nil), tc.dto.Specialties...)
			v := VetFromDTO(&tc.dto)
			if v == nil {
				t.Fatal("expected vet")
			}
			if v.NrOfSpecialties() != len(tc.wantSpecs) {
				t.Fatalf("NrOfSpecialties = %d, want %d", v.NrOfSpecialties(), len(tc.wantSpecs))
			}
			w := vetWire(t, v)
			if idVal(w.ID) != tc.wantID || w.FirstName != tc.wantFirst || w.LastName != tc.wantLast {
				t.Fatalf("got %+v", w)
			}
			if len(w.Specialties) != len(tc.wantSpecs) {
				t.Fatalf("specialties = %+v", w.Specialties)
			}
			for i, s := range tc.wantSpecs {
				if idVal(w.Specialties[i].ID) != idVal(s.ID) || w.Specialties[i].Name != s.Name {
					t.Errorf("specialty %d = %+v, want %+v", i, w.Specialties[i], s)
				}
			}
			if before.FirstName != tc.dto.FirstName || before.LastName != tc.dto.LastName ||
				idVal(before.ID) != idVal(tc.dto.ID) || !reflect.DeepEqual(beforeSpecs, append([]SpecialtyDTO(nil), tc.dto.Specialties...)) {
				t.Error("input mutated")
			}
		})
	}
}

func TestVetFromFieldsDTO(t *testing.T) {
	if got := VetFromFieldsDTO(nil); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
	tests := []struct {
		name  string
		dto   VetFieldsDTO
		specs int
	}{
		{"one specialty", VetFieldsDTO{FirstName: "Helen", LastName: "Leary", Specialties: []SpecialtyDTO{{ID: intPtr(1), Name: "radiology"}}}, 1},
		{"no specialties", VetFieldsDTO{FirstName: "Helen", LastName: "Leary"}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := VetFromFieldsDTO(&tc.dto)
			if v == nil {
				t.Fatal("expected vet")
			}
			w := vetWire(t, v)
			if w.ID != nil && *w.ID != 0 {
				t.Errorf("id should be unset, got %d", *w.ID)
			}
			if w.FirstName != "Helen" || w.LastName != "Leary" {
				t.Errorf("names = %q %q", w.FirstName, w.LastName)
			}
			if len(w.Specialties) != tc.specs || v.NrOfSpecialties() != tc.specs {
				t.Fatalf("specialties = %+v", w.Specialties)
			}
			if tc.specs == 1 && (idVal(w.Specialties[0].ID) != 1 || w.Specialties[0].Name != "radiology") {
				t.Errorf("specialty = %+v", w.Specialties[0])
			}
		})
	}
}

func TestVetToDTO(t *testing.T) {
	if got := VetToDTO(nil); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
	tests := []struct {
		name  string
		in    wireVet
		specs []wireSpecialty
	}{
		{"one specialty", wireVet{ID: intPtr(2), FirstName: "Helen", LastName: "Leary",
			Specialties: []wireSpecialty{{ID: intPtr(1), Name: "radiology"}}}, []wireSpecialty{{ID: intPtr(1), Name: "radiology"}}},
		{"no specialties", wireVet{ID: intPtr(2), FirstName: "Helen", LastName: "Leary"}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := buildVet(t, tc.in)
			before := vetWire(t, v)
			dto := VetToDTO(v)
			if dto == nil {
				t.Fatal("expected dto")
			}
			if idVal(dto.ID) != 2 || dto.FirstName != "Helen" || dto.LastName != "Leary" {
				t.Errorf("dto = %+v", dto)
			}
			if dto.Specialties == nil {
				t.Fatal("specialties must be non-nil")
			}
			if len(dto.Specialties) != len(tc.specs) {
				t.Fatalf("specialties = %+v", dto.Specialties)
			}
			for i, s := range tc.specs {
				if idVal(dto.Specialties[i].ID) != idVal(s.ID) || dto.Specialties[i].Name != s.Name {
					t.Errorf("specialty %d = %+v", i, dto.Specialties[i])
				}
			}
			b, _ := json.Marshal(dto)
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			if _, ok := m["specialties"].([]any); !ok {
				t.Errorf("specialties not JSON array: %s", b)
			}
			if !reflect.DeepEqual(before, vetWire(t, v)) {
				t.Error("entity mutated")
			}
		})
	}
}

func TestVetRoundTrip(t *testing.T) {
	in := VetDTO{ID: intPtr(7), FirstName: "Rafael", LastName: "Ortega",
		Specialties: []SpecialtyDTO{{ID: intPtr(1), Name: "radiology"}, {ID: intPtr(2), Name: "surgery"}}}
	out := VetToDTO(VetFromDTO(&in))
	if out == nil || idVal(out.ID) != 7 || out.FirstName != in.FirstName || out.LastName != in.LastName {
		t.Fatalf("round trip = %+v", out)
	}
	if len(out.Specialties) != 2 {
		t.Fatalf("specialties = %+v", out.Specialties)
	}
	for i := range in.Specialties {
		if idVal(out.Specialties[i].ID) != idVal(in.Specialties[i].ID) || out.Specialties[i].Name != in.Specialties[i].Name {
			t.Errorf("specialty %d = %+v", i, out.Specialties[i])
		}
	}
}

func TestVetsToDTOs(t *testing.T) {
	if got := VetsToDTOs(nil); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
	v1 := buildVet(t, wireVet{ID: intPtr(1), FirstName: "James", LastName: "Carter"})
	v2 := buildVet(t, wireVet{ID: intPtr(2), FirstName: "Helen", LastName: "Leary",
		Specialties: []wireSpecialty{{ID: intPtr(1), Name: "radiology"}}})

	tests := []struct {
		name string
		in   []*model.Vet
		want int
	}{
		{"empty", []*model.Vet{}, 0},
		{"two", []*model.Vet{v1, v2}, 2},
		{"nil element skipped", []*model.Vet{v1, nil}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inCopy := append([]*model.Vet(nil), tc.in...)
			got := VetsToDTOs(tc.in)
			if got == nil {
				t.Fatal("expected non-nil slice")
			}
			if len(got) != tc.want {
				t.Fatalf("len = %d, want %d", len(got), tc.want)
			}
			idx := 0
			for _, v := range tc.in {
				if v == nil {
					continue
				}
				if !reflect.DeepEqual(got[idx], *VetToDTO(v)) {
					t.Errorf("element %d = %+v", idx, got[idx])
				}
				idx++
			}
			if !reflect.DeepEqual(inCopy, tc.in) {
				t.Error("input mutated")
			}
		})
	}
}