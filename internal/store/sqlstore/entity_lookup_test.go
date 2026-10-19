package sqlstore

import (
	"errors"
	"testing"

	"migrated-app/internal/model"
)

type testPet struct {
	model.BaseEntity
	Name string
}

type testVet struct {
	model.BaseEntity
}

func newPet(id *int, name string) *testPet {
	p := &testPet{Name: name}
	p.SetID(id)
	return p
}

func TestByID(t *testing.T) {
	a := newPet(model.IntPtr(1), "a")
	b := newPet(model.IntPtr(2), "b")
	b2 := newPet(model.IntPtr(2), "b2")
	nilID := newPet(nil, "nil")
	zero := newPet(model.IntPtr(0), "zero")

	tests := []struct {
		name     string
		entities []*testPet
		id       int
		want     *testPet
		wantErr  bool
	}{
		{"single match", []*testPet{a, b}, 1, a, false},
		{"first of several matches", []*testPet{a, b, b2}, 2, b, false},
		{"zero id matches", []*testPet{zero}, 0, zero, false},
		{"nil id before match skipped", []*testPet{nilID, a}, 1, a, false},
		{"nil id after match", []*testPet{a, nilID}, 1, a, false},
		{"nil element before match", []*testPet{nil, a}, 1, a, false},
		{"no match", []*testPet{a, b}, 3, nil, true},
		{"empty", []*testPet{}, 1, nil, true},
		{"nil slice", nil, 1, nil, true},
		{"only nil id", []*testPet{nilID}, 0, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := byID(tt.entities, tt.id)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if got != nil {
					t.Errorf("expected zero value, got %v", got)
				}
				checkErr(t, err, "*sqlstore.testPet", tt.id)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %p want %p", got, tt.want)
			}
			if *got.GetID() != tt.id {
				t.Errorf("returned id %d want %d", *got.GetID(), tt.id)
			}
		})
	}
}

func TestByIDDoesNotModifyInput(t *testing.T) {
	a := newPet(model.IntPtr(1), "a")
	b := newPet(model.IntPtr(2), "b")
	in := []*testPet{a, b}
	_, _ = byID(in, 2)
	_, _ = byID(in, 9)
	if in[0] != a || in[1] != b || *a.ID != 1 || *b.ID != 2 || a.Name != "a" {
		t.Errorf("input modified")
	}
}

func TestByIDTypeInError(t *testing.T) {
	_, err := byID([]*testVet{}, 5)
	checkErr(t, err, "*sqlstore.testVet", 5)
}

func checkErr(t *testing.T, err error, typ string, id int) {
	t.Helper()
	if !errors.Is(err, ErrObjectRetrievalFailure) {
		t.Errorf("errors.Is failed for %v", err)
	}
	var orf *ObjectRetrievalFailureError
	if !errors.As(err, &orf) {
		t.Fatalf("not ObjectRetrievalFailureError: %T", err)
	}
	if orf.EntityType != typ || orf.ID != id {
		t.Errorf("got type=%q id=%d want %q %d", orf.EntityType, orf.ID, typ, id)
	}
	want := "Object of class [" + typ + "] with identifier [" + itoa(id) + "]: not found"
	if err.Error() != want {
		t.Errorf("message %q want %q", err.Error(), want)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

func TestObjectRetrievalFailureErrorMessage(t *testing.T) {
	tests := []struct {
		e    *ObjectRetrievalFailureError
		want string
	}{
		{&ObjectRetrievalFailureError{"Owner", 7}, "Object of class [Owner] with identifier [7]: not found"},
		{&ObjectRetrievalFailureError{"Pet", -1}, "Object of class [Pet] with identifier [-1]: not found"},
	}
	for _, tt := range tests {
		if got := tt.e.Error(); got != tt.want {
			t.Errorf("got %q want %q", got, tt.want)
		}
		if tt.e.Unwrap() != ErrObjectRetrievalFailure {
			t.Errorf("unwrap mismatch")
		}
	}
}

func TestIndexByIDAndLookup(t *testing.T) {
	a := newPet(model.IntPtr(1), "a")
	b := newPet(model.IntPtr(2), "b")
	b2 := newPet(model.IntPtr(2), "b2")
	nilID := newPet(nil, "n")
	idx := indexByID([]*testPet{nilID, a, b, b2})
	if len(idx) != 2 {
		t.Fatalf("len %d want 2", len(idx))
	}
	tests := []struct {
		name    string
		id      int
		want    *testPet
		wantErr bool
	}{
		{"found", 1, a, false},
		{"first wins", 2, b, false},
		{"missing", 3, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lookupByID(idx, tt.id)
			if tt.wantErr {
				if got != nil {
					t.Errorf("expected nil")
				}
				checkErr(t, err, "*sqlstore.testPet", tt.id)
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("got %v,%v want %v", got, err, tt.want)
			}
		})
	}
	empty := indexByID[*testPet](nil)
	if _, err := lookupByID(empty, 1); err == nil {
		t.Errorf("expected error on empty index")
	}
}