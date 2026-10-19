package sqlstore

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"migrated-app/internal/model"
)

// fakePetRow is a minimal rowScanner returning fixed positional values.
type fakePetRow struct {
	values []any
	err    error
}

func (r *fakePetRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("expected %d destinations, got %d", len(r.values), len(dest))
	}
	for i, d := range dest {
		src := r.values[i]
		switch t := d.(type) {
		case *int:
			v, ok := src.(int)
			if !ok {
				return fmt.Errorf("column %d: cannot scan %T into int", i, src)
			}
			*t = v
		case **model.Date:
			if src != nil {
				return fmt.Errorf("column %d: only NULL dates supported in fake", i)
			}
			*t = nil
		case sql.Scanner:
			if err := t.Scan(src); err != nil {
				return err
			}
		default:
			return fmt.Errorf("column %d: unsupported destination %T", i, d)
		}
	}
	return nil
}

func TestMapPetRowFullValues(t *testing.T) {
	p, err := mapPetRow(&fakePetRow{values: []any{7, "Samantha", nil, int64(1), int64(6)}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil || p.ID == nil || *p.ID != 7 {
		t.Fatalf("unexpected id: %+v", p)
	}
	if p.Name != "Samantha" {
		t.Errorf("name = %q, want Samantha", p.Name)
	}
	if p.BirthDate != nil {
		t.Errorf("birth date = %v, want nil", p.BirthDate)
	}
}

func TestMapPetRowNullValues(t *testing.T) {
	p, err := mapPetRow(&fakePetRow{values: []any{3, nil, nil, nil, nil}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *p.ID != 3 {
		t.Errorf("id = %d, want 3", *p.ID)
	}
	if p.Name != "" {
		t.Errorf("name = %q, want empty", p.Name)
	}
	if p.BirthDate != nil {
		t.Errorf("birth date should be nil")
	}
}

func TestMapPetRowScanError(t *testing.T) {
	sentinel := errors.New("boom")
	p, err := mapPetRow(&fakePetRow{err: sentinel})
	if p != nil {
		t.Errorf("expected nil pet, got %+v", p)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap sentinel", err)
	}
	if err != nil && !strings.Contains(err.Error(), "map pet row") {
		t.Errorf("error %q missing context", err)
	}
}

func TestMapPetRowDistinctInstances(t *testing.T) {
	vals := []any{7, "Samantha", nil, int64(1), int64(6)}
	a, err := mapPetRow(&fakePetRow{values: vals})
	if err != nil {
		t.Fatal(err)
	}
	b, err := mapPetRow(&fakePetRow{values: vals})
	if err != nil {
		t.Fatal(err)
	}
	if a == b || a.ID == b.ID {
		t.Error("expected distinct instances and id pointers")
	}
}

func TestJoinedPetColumns(t *testing.T) {
	want := []string{"pets.id AS pets_id", "pets.name", "pets.birth_date", "pets.type_id", "pets.owner_id"}
	cols := strings.Split(joinedPetColumns, ",")
	if len(cols) != len(want) {
		t.Fatalf("got %d columns, want %d", len(cols), len(want))
	}
	for i, c := range cols {
		if strings.TrimSpace(c) != want[i] {
			t.Errorf("column %d = %q, want %q", i, strings.TrimSpace(c), want[i])
		}
	}
}