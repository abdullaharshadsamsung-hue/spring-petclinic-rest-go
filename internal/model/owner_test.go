package model

import (
	"errors"
	"strings"
	"testing"
)

func newPet(name string, id *int) *Pet {
	p := &Pet{}
	p.Name = name
	p.ID = id
	return p
}

func TestOwnersTable(t *testing.T) {
	if OwnersTable != "owners" {
		t.Fatalf("got %q", OwnersTable)
	}
}

func TestOwnerAccessors(t *testing.T) {
	o := NewOwner()
	if o.GetAddress() != "" || o.GetCity() != "" || o.GetTelephone() != "" {
		t.Fatal("expected empty defaults")
	}
	tests := []struct {
		name string
		set  func(string)
		get  func() string
		val  string
	}{
		{"address", o.SetAddress, o.GetAddress, "110 W. Liberty St."},
		{"address untrimmed", o.SetAddress, o.GetAddress, "  x  "},
		{"city", o.SetCity, o.GetCity, "Madison"},
		{"telephone", o.SetTelephone, o.GetTelephone, "6085551023"},
		{"telephone invalid stored", o.SetTelephone, o.GetTelephone, "608555102"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.set(tc.val)
			if got := tc.get(); got != tc.val {
				t.Fatalf("got %q want %q", got, tc.val)
			}
		})
	}
}

func TestOwnerNilGetters(t *testing.T) {
	var o *Owner
	if o.GetAddress() != "" || o.GetCity() != "" || o.GetTelephone() != "" {
		t.Fatal("nil getters should return empty")
	}
	if p := o.GetPets(); p == nil || len(p) != 0 {
		t.Fatal("nil GetPets should be empty non-nil")
	}
	if o.GetPet("x") != nil || o.GetPetByID(IntPtr(1)) != nil {
		t.Fatal("nil lookups should return nil")
	}
}

func validOwner() *Owner {
	o := NewOwner()
	o.FirstName = "George"
	o.LastName = "Franklin"
	o.Address = "110 W. Liberty St."
	o.City = "Madison"
	o.Telephone = "6085551023"
	return o
}

func TestOwnerValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(o *Owner)
		want    []error
		notWant []error
	}{
		{"valid", func(o *Owner) {}, nil, nil},
		{"empty address", func(o *Owner) { o.Address = "" }, []error{ErrAddressEmpty}, []error{ErrCityEmpty}},
		{"empty city", func(o *Owner) { o.City = "" }, []error{ErrCityEmpty}, []error{ErrAddressEmpty}},
		{"empty telephone", func(o *Owner) { o.Telephone = "" }, []error{ErrTelephoneEmpty, ErrTelephoneInvalid}, nil},
		{"9 digits", func(o *Owner) { o.Telephone = "608555102" }, []error{ErrTelephoneInvalid}, []error{ErrTelephoneEmpty}},
		{"11 digits", func(o *Owner) { o.Telephone = "60855510234" }, []error{ErrTelephoneInvalid}, nil},
		{"dashes", func(o *Owner) { o.Telephone = "608-555-1023" }, []error{ErrTelephoneInvalid}, nil},
		{"letter", func(o *Owner) { o.Telephone = "608555102a" }, []error{ErrTelephoneInvalid}, nil},
		{"non-ascii digits", func(o *Owner) { o.Telephone = "６０８５５５１０２３" }, []error{ErrTelephoneInvalid}, nil},
		{"empty names", func(o *Owner) { o.FirstName = ""; o.LastName = "" }, []error{ErrFirstNameEmpty, ErrLastNameEmpty}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := validOwner()
			tc.mutate(o)
			err := o.Validate()
			if tc.want == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			for _, w := range tc.want {
				if !errors.Is(err, w) {
					t.Errorf("missing %v in %v", w, err)
				}
			}
			for _, nw := range tc.notWant {
				if errors.Is(err, nw) {
					t.Errorf("unexpected %v", nw)
				}
			}
		})
	}
	var nilOwner *Owner
	err := nilOwner.Validate()
	for _, w := range []error{ErrFirstNameEmpty, ErrLastNameEmpty, ErrAddressEmpty, ErrCityEmpty, ErrTelephoneEmpty, ErrTelephoneInvalid} {
		if !errors.Is(err, w) {
			t.Errorf("nil owner missing %v", w)
		}
	}
	if !strings.Contains(ErrTelephoneInvalid.Error(), "Phone number must be exactly 10 digits") {
		t.Error("bad message")
	}
}

func TestGetPetsSortingAndCopy(t *testing.T) {
	o := NewOwner()
	if p := o.GetPets(); p == nil || len(p) != 0 {
		t.Fatal("expected empty non-nil")
	}
	maxP, bella, charlie := newPet("max", nil), newPet("Bella", nil), newPet("charlie", nil)
	o.AddPet(maxP)
	o.AddPet(bella)
	o.AddPet(charlie)
	got := o.GetPets()
	want := []*Pet{bella, charlie, maxP}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %s", i, got[i].Name)
		}
	}
	got[0] = nil
	_ = append(got, newPet("zed", nil))
	if again := o.GetPets(); len(again) != 3 || again[0] != bella {
		t.Fatal("returned slice must be a copy")
	}
	o.AddPet(newPet("Aaron", nil))
	if len(want) != 3 {
		t.Fatal("snapshot changed")
	}
}

func TestGetPetsNilEntriesSortLast(t *testing.T) {
	o := NewOwner()
	a := newPet("a", nil)
	o.SetPets([]*Pet{nil, a})
	got := o.GetPets()
	if len(got) != 2 || got[0] != a || got[1] != nil {
		t.Fatalf("got %v", got)
	}
}

func TestSetPets(t *testing.T) {
	a, b := newPet("b", nil), newPet("a", nil)
	tests := []struct {
		name  string
		input []*Pet
		want  []*Pet
	}{
		{"two distinct", []*Pet{a, b}, []*Pet{b, a}},
		{"duplicate", []*Pet{a, a}, []*Pet{a}},
		{"nil list", nil, []*Pet{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := NewOwner()
			o.SetPets(tc.input)
			got := o.GetPets()
			if len(got) != len(tc.want) {
				t.Fatalf("len %d want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("index %d mismatch", i)
				}
			}
		})
	}
	o := NewOwner()
	in := []*Pet{a}
	o.SetPets(in)
	in[0] = b
	if o.GetPets()[0] != a {
		t.Fatal("input mutation leaked")
	}
	if a.GetOwner() != nil {
		t.Fatal("SetPets must not set back-reference")
	}
}

func TestAddPet(t *testing.T) {
	o := NewOwner()
	o.SetID(IntPtr(5))
	leo := newPet("Leo", nil)
	o.AddPet(leo)
	o.AddPet(leo)
	o.AddPet(nil)
	pets := o.GetPets()
	if len(pets) != 1 || pets[0] != leo {
		t.Fatalf("got %v", pets)
	}
	if leo.GetOwner() != OwnerRef(o) {
		t.Fatal("owner not set")
	}
	if id := leo.GetOwnerID(); id == nil || *id != 5 {
		t.Fatal("owner id not set")
	}
}

func TestGetPetByName(t *testing.T) {
	o := NewOwner()
	leo := newPet("Leo", nil)
	saved := newPet("Rex", IntPtr(3))
	o.SetPets([]*Pet{nil, leo, saved})
	tests := []struct {
		name      string
		query     string
		ignoreNew bool
		want      *Pet
	}{
		{"case insensitive", "leo", false, leo},
		{"no match", "max", false, nil},
		{"no partial", "le", false, nil},
		{"no trim", " leo", false, nil},
		{"ignoreNew skips new", "leo", true, nil},
		{"ignoreNew keeps saved", "REX", true, saved},
		{"include new", "LEO", false, leo},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := o.GetPetIgnoreNew(tc.query, tc.ignoreNew); got != tc.want {
				t.Fatalf("got %v", got)
			}
			if !tc.ignoreNew {
				if got := o.GetPet(tc.query); got != tc.want {
					t.Fatalf("GetPet got %v", got)
				}
			}
		})
	}
	if NewOwner().GetPet("leo") != nil || NewOwner().GetPetIgnoreNew("leo", true) != nil {
		t.Fatal("empty owner should return nil")
	}
}

func TestGetPetByID(t *testing.T) {
	o := NewOwner()
	seven := newPet("Seven", IntPtr(7))
	o.SetPets([]*Pet{nil, newPet("New", nil), seven})
	tests := []struct {
		name string
		id   *int
		want *Pet
	}{
		{"found", IntPtr(7), seven},
		{"not found", IntPtr(8), nil},
		{"nil id", nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := o.GetPetByID(tc.id); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestOwnerString(t *testing.T) {
	o := validOwner()
	o.SetID(IntPtr(1))
	o.AddPet(newPet("Leo", nil))
	tests := []struct {
		name string
		o    *Owner
		want string
	}{
		{"saved", o, "[Owner id = 1, new = false, lastName = 'Franklin', firstName = 'George', address = '110 W. Liberty St.', city = 'Madison', telephone = '6085551023']"},
		{"unsaved", NewOwner(), "[Owner id = [null], new = true, lastName = '', firstName = '', address = '', city = '', telephone = '']"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.o.String()
			if got != tc.want {
				t.Fatalf("got %q", got)
			}
			if strings.Contains(got, "Leo") {
				t.Fatal("pets must not appear")
			}
		})
	}
	if o.IsNew() {
		t.Fatal("owner with id must not be new")
	}
	if !NewOwner().IsNew() {
		t.Fatal("new owner must be new")
	}
}