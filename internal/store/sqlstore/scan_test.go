package sqlstore

import (
	"errors"
	"math"
	"strings"
	"testing"

	"migrated-app/internal/model"
)

// fakeRow is an in-memory rowScanner used to mock *sql.Row / *sql.Rows.
type fakeRow struct {
	id      int
	name    string
	birth   *model.Date
	typeID  int
	ownerID int
	err     error
}

func (f *fakeRow) Scan(dest ...any) error {
	if f.err != nil {
		return f.err
	}
	if len(dest) != 5 {
		return errors.New("unexpected column count")
	}
	*(dest[0].(*int)) = f.id
	*(dest[1].(*string)) = f.name
	*(dest[2].(**model.Date)) = f.birth
	*(dest[3].(*int)) = f.typeID
	*(dest[4].(*int)) = f.ownerID
	return nil
}

func TestPetColumns(t *testing.T) {
	if petColumns != "id, name, birth_date, type_id, owner_id" {
		t.Fatalf("unexpected petColumns %q", petColumns)
	}
}

func TestNewJdbcPetDefaults(t *testing.T) {
	p := newJdbcPet()
	if p == nil || p.Pet == nil {
		t.Fatal("newJdbcPet must wrap a non-nil Pet")
	}
	if got := p.TypeID(); got != 0 {
		t.Errorf("TypeID() = %d, want 0", got)
	}
	if got := p.OwnerIDValue(); got != 0 {
		t.Errorf("OwnerIDValue() = %d, want 0", got)
	}
	if p.ID != nil {
		t.Errorf("ID should be unset, got %v", *p.ID)
	}
	if p.Name != "" || p.BirthDate != nil || p.Type != nil || p.OwnerID != nil {
		t.Error("inherited Pet state should be default")
	}
}

func TestJdbcPetSubstitutableForPet(t *testing.T) {
	p := newJdbcPet()
	usePet := func(pet *model.Pet) *model.Pet { return pet }
	pet := usePet(p.Pet)

	pet.Name = "Leo"
	if p.Name != "Leo" {
		t.Errorf("Name = %q, want Leo", p.Name)
	}
	d := new(model.Date)
	p.BirthDate = d
	if p.BirthDate != d {
		t.Error("birth date not set")
	}
	pt := &model.PetType{}
	p.Type = pt
	if p.Type != pt {
		t.Error("type not set")
	}
	id := 9
	p.ID = &id
	if p.ID == nil || *p.ID != 9 {
		t.Error("id not set")
	}
	if p.TypeID() != 0 || p.OwnerIDValue() != 0 {
		t.Error("inherited operations must not touch transient fields")
	}
}

func TestSetTypeID(t *testing.T) {
	tests := []struct {
		name string
		vals []int
		want int
	}{
		{"zero", []int{0}, 0},
		{"positive", []int{7}, 7},
		{"negative", []int{-3}, -3},
		{"max", []int{math.MaxInt}, math.MaxInt},
		{"min", []int{math.MinInt}, math.MinInt},
		{"overwrite", []int{1, 2, 3}, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := newJdbcPet()
			p.SetOwnerIDValue(42)
			pt := &model.PetType{}
			p.Type = pt
			for _, v := range tc.vals {
				p.SetTypeID(v)
			}
			if got := p.TypeID(); got != tc.want {
				t.Errorf("TypeID() = %d, want %d", got, tc.want)
			}
			if p.OwnerIDValue() != 42 {
				t.Error("SetTypeID changed ownerID")
			}
			if p.Type != pt {
				t.Error("SetTypeID changed Type reference")
			}
			if p.OwnerID != nil || p.Name != "" {
				t.Error("SetTypeID changed other Pet fields")
			}
		})
	}
}

func TestSetOwnerIDValue(t *testing.T) {
	tests := []struct {
		name string
		vals []int
		want int
	}{
		{"zero", []int{0}, 0},
		{"positive", []int{11}, 11},
		{"negative", []int{-1}, -1},
		{"max", []int{math.MaxInt}, math.MaxInt},
		{"min", []int{math.MinInt}, math.MinInt},
		{"overwrite", []int{5, 6}, 6},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := newJdbcPet()
			p.SetTypeID(13)
			for _, v := range tc.vals {
				p.SetOwnerIDValue(v)
			}
			if got := p.OwnerIDValue(); got != tc.want {
				t.Errorf("OwnerIDValue() = %d, want %d", got, tc.want)
			}
			if p.TypeID() != 13 {
				t.Error("SetOwnerIDValue changed typeID")
			}
			if p.Pet.OwnerID != nil {
				t.Error("SetOwnerIDValue changed inherited owner state")
			}
		})
	}
}

func TestScanPet(t *testing.T) {
	date := new(model.Date)
	scanErr := errors.New("boom")
	tests := []struct {
		name    string
		row     *fakeRow
		wantErr error
	}{
		{"with birth date", &fakeRow{id: 1, name: "Leo", birth: date, typeID: 2, ownerID: 3}, nil},
		{"nil birth date", &fakeRow{id: 4, name: "Max", birth: nil, typeID: 5, ownerID: 6}, nil},
		{"scan error", &fakeRow{err: scanErr}, scanErr},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := scanPet(tc.row)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want wrapping %v", err, tc.wantErr)
				}
				if !strings.Contains(err.Error(), "scan pet") {
					t.Errorf("err message %q missing context", err.Error())
				}
				if p != nil {
					t.Error("expected nil pet on error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.ID == nil || *p.ID != tc.row.id {
				t.Errorf("ID mismatch")
			}
			if p.Name != tc.row.name {
				t.Errorf("Name = %q", p.Name)
			}
			if p.BirthDate != tc.row.birth {
				t.Errorf("BirthDate mismatch")
			}
			if p.TypeID() != tc.row.typeID {
				t.Errorf("TypeID = %d", p.TypeID())
			}
			if p.OwnerIDValue() != tc.row.ownerID {
				t.Errorf("OwnerIDValue = %d", p.OwnerIDValue())
			}
			if p.Pet.OwnerID == nil || *p.Pet.OwnerID != tc.row.ownerID {
				t.Errorf("Pet.OwnerID mismatch")
			}
			if p.Type != nil {
				t.Error("Type should not be resolved yet")
			}
			p.SetOwnerIDValue(999)
			if *p.Pet.OwnerID != tc.row.ownerID {
				t.Error("Pet.OwnerID should be independent of transient ownerID")
			}
		})
	}
}

func TestResolveType(t *testing.T) {
	cat := &model.PetType{}
	dog := &model.PetType{}
	types := map[int]*model.PetType{1: cat, 2: dog}
	tests := []struct {
		name    string
		typeID  int
		types   map[int]*model.PetType
		want    *model.PetType
		wantErr bool
	}{
		{"found cat", 1, types, cat, false},
		{"found dog", 2, types, dog, false},
		{"unknown id", 99, types, nil, true},
		{"empty index", 1, map[int]*model.PetType{}, nil, true},
		{"nil index", 0, nil, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := newJdbcPet()
			p.SetTypeID(tc.typeID)
			err := p.resolveType(tc.types)
			if tc.wantErr {
				if !errors.Is(err, ErrObjectRetrievalFailure) {
					t.Fatalf("err = %v, want ErrObjectRetrievalFailure", err)
				}
				var orf *ObjectRetrievalFailureError
				if !errors.As(err, &orf) {
					t.Fatal("expected *ObjectRetrievalFailureError")
				}
				if orf.ID != tc.typeID {
					t.Errorf("ID = %d, want %d", orf.ID, tc.typeID)
				}
				if !strings.Contains(err.Error(), "resolve type of pet") {
					t.Errorf("missing context in %q", err.Error())
				}
				if p.Type != nil {
					t.Error("Type should remain nil on error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Type != tc.want {
				t.Error("wrong type resolved")
			}
			if p.TypeID() != tc.typeID {
				t.Error("resolveType must not change typeID")
			}
		})
	}
}