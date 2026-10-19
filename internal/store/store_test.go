package store

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestErrNotFoundMessage(t *testing.T) {
	if ErrNotFound == nil {
		t.Fatal("ErrNotFound must not be nil")
	}
	if got, want := ErrNotFound.Error(), "store: entity not found"; got != want {
		t.Fatalf("ErrNotFound.Error() = %q, want %q", got, want)
	}
}

func TestErrNotFoundWrapping(t *testing.T) {
	wrapped := fmt.Errorf("pet type 42: %w", ErrNotFound)
	if !errors.Is(wrapped, ErrNotFound) {
		t.Fatal("wrapped ErrNotFound should match via errors.Is")
	}
	doubly := fmt.Errorf("outer: %w", wrapped)
	if !errors.Is(doubly, ErrNotFound) {
		t.Fatal("doubly wrapped ErrNotFound should match via errors.Is")
	}
	if errors.Is(errors.New("store: entity not found"), ErrNotFound) {
		t.Fatal("distinct error with same text must not match ErrNotFound")
	}
}

func TestPetTypeRepositoryMethodSet(t *testing.T) {
	typ := reflect.TypeOf((*PetTypeRepository)(nil)).Elem()
	if typ.Kind() != reflect.Interface {
		t.Fatalf("PetTypeRepository kind = %v, want interface", typ.Kind())
	}
	want := map[string]struct{ in, out int }{
		"FindByID":   {2, 2},
		"FindByName": {2, 2},
		"FindAll":    {1, 2},
		"Save":       {2, 1},
		"Delete":     {2, 1},
	}
	if typ.NumMethod() != len(want) {
		t.Fatalf("NumMethod = %d, want %d", typ.NumMethod(), len(want))
	}
	errType := reflect.TypeOf((*error)(nil)).Elem()
	for name, sig := range want {
		m, ok := typ.MethodByName(name)
		if !ok {
			t.Errorf("missing method %s", name)
			continue
		}
		if m.Type.NumIn() != sig.in || m.Type.NumOut() != sig.out {
			t.Errorf("%s: in=%d out=%d, want in=%d out=%d",
				name, m.Type.NumIn(), m.Type.NumOut(), sig.in, sig.out)
			continue
		}
		if last := m.Type.Out(m.Type.NumOut() - 1); last != errType {
			t.Errorf("%s: last result is %v, want error", name, last)
		}
	}
}