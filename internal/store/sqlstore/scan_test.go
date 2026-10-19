package sqlstore

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"migrated-app/internal/model"
)

// fakeRow is an in-memory rowScanner that mimics the conversions
// database/sql performs for the destination types used by the mappers.
type fakeRow struct {
	values    []any
	scanErr   error
	scanCalls int
}

var errFakeScan = errors.New("fake scan failure")

func (f *fakeRow) Scan(dest ...any) error {
	f.scanCalls++
	if f.scanErr != nil {
		return f.scanErr
	}
	if len(dest) != len(f.values) {
		return fmt.Errorf("expected %d destination arguments in Scan, not %d", len(f.values), len(dest))
	}
	for i, d := range dest {
		v := f.values[i]
		switch t := d.(type) {
		case sql.Scanner:
			if err := t.Scan(v); err != nil {
				return fmt.Errorf("column %d: %w", i, err)
			}
		case *int:
			switch n := v.(type) {
			case int:
				*t = n
			case int64:
				*t = int(n)
			case nil:
				return fmt.Errorf("column %d: converting NULL to int is unsupported", i)
			default:
				return fmt.Errorf("column %d: unsupported type %T", i, v)
			}
		default:
			return fmt.Errorf("column %d: unsupported destination %T", i, d)
		}
	}
	return nil
}

func TestScanVisit(t *testing.T) {
	d2013 := model.NewDate(2013, time.January, 1)
	tests := []struct {
		name     string
		row      *fakeRow
		wantErr  bool
		wantID   int
		wantDate model.Date
		wantDesc string
	}{
		{
			name:     "full row from time.Time",
			row:      &fakeRow{values: []any{int64(1), time.Date(2013, 1, 1, 0, 0, 0, 0, time.UTC), "rabies shot"}},
			wantID:   1,
			wantDate: d2013,
			wantDesc: "rabies shot",
		},
		{
			name:     "date as string and bytes description",
			row:      &fakeRow{values: []any{int64(7), "2013-01-01", []byte("neutered")}},
			wantID:   7,
			wantDate: d2013,
			wantDesc: "neutered",
		},
		{
			name:     "timestamp text is truncated to date",
			row:      &fakeRow{values: []any{int64(2), "2013-01-01 10:11:12", "x"}},
			wantID:   2,
			wantDate: d2013,
			wantDesc: "x",
		},
		{
			name:     "NULL description becomes empty string",
			row:      &fakeRow{values: []any{int64(3), d2013.Time(), nil}},
			wantID:   3,
			wantDate: d2013,
			wantDesc: "",
		},
		{name: "NULL visit_id is an error", row: &fakeRow{values: []any{nil, d2013.Time(), "x"}}, wantErr: true},
		{name: "NULL visit_date is an error", row: &fakeRow{values: []any{int64(1), nil, "x"}}, wantErr: true},
		{name: "unconvertible visit_date", row: &fakeRow{values: []any{int64(1), "not-a-date", "x"}}, wantErr: true},
		{name: "unsupported date type", row: &fakeRow{values: []any{int64(1), 42, "x"}}, wantErr: true},
		{name: "missing column", row: &fakeRow{values: []any{int64(1), d2013.Time()}}, wantErr: true},
		{name: "underlying scan error", row: &fakeRow{scanErr: errFakeScan}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, err := scanVisit(tc.row)
			if tc.row.scanCalls != 1 {
				t.Errorf("Scan called %d times, want 1", tc.row.scanCalls)
			}
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got visit %+v", v)
				}
				if v != nil {
					t.Errorf("expected nil visit on error")
				}
				if !strings.Contains(err.Error(), "scan visit row") {
					t.Errorf("error %q lacks context", err)
				}
				if tc.row.scanErr != nil && !errors.Is(err, tc.row.scanErr) {
					t.Errorf("error not wrapped: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id := v.GetID(); id == nil || *id != tc.wantID {
				t.Errorf("id = %v, want %d", id, tc.wantID)
			}
			if d := v.GetDate(); d == nil || !d.Equal(tc.wantDate) {
				t.Errorf("date = %v, want %v", d, tc.wantDate)
			}
			if v.GetDescription() != tc.wantDesc {
				t.Errorf("description = %q, want %q", v.GetDescription(), tc.wantDesc)
			}
			if v.GetPet() != nil {
				t.Errorf("pet should be unset")
			}
			if v.PetID != nil {
				t.Errorf("petID should be unset, got %v", *v.PetID)
			}
		})
	}
}

func TestScanVisitReturnsDistinctInstances(t *testing.T) {
	vals := []any{int64(1), "2013-01-01", "rabies shot"}
	a, err := scanVisit(&fakeRow{values: vals})
	if err != nil {
		t.Fatal(err)
	}
	b, err := scanVisit(&fakeRow{values: vals})
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("expected distinct instances")
	}
	if *a.GetID() != *b.GetID() || a.GetDescription() != b.GetDescription() || !a.GetDate().Equal(*b.GetDate()) {
		t.Error("same row should map identically")
	}
	a.SetDescription("changed")
	if b.GetDescription() != "rabies shot" {
		t.Error("instances share state")
	}
}

func TestVisitColumnsLabels(t *testing.T) {
	for _, want := range []string{"AS visit_id", "visit_date", "description"} {
		if !strings.Contains(visitColumns, want) {
			t.Errorf("visitColumns missing %q", want)
		}
	}
	if strings.Index(visitColumns, "visit_id") > strings.Index(visitColumns, "visit_date") ||
		strings.Index(visitColumns, "visit_date") > strings.Index(visitColumns, "description") {
		t.Error("visitColumns order does not match scanVisit")
	}
}

func TestMapPetRow(t *testing.T) {
	tests := []struct {
		name      string
		row       *fakeRow
		wantErr   bool
		wantID    int
		wantType  int
		wantOwner int
	}{
		{name: "full row", row: &fakeRow{values: []any{int64(5), "Leo", "2010-09-07", int64(1), int64(2)}}, wantID: 5, wantType: 1, wantOwner: 2},
		{name: "NULL fks map to zero", row: &fakeRow{values: []any{int64(6), nil, "2010-09-07", nil, nil}}, wantID: 6},
		{name: "scan error", row: &fakeRow{scanErr: errFakeScan}, wantErr: true},
		{name: "bad date", row: &fakeRow{values: []any{int64(6), "x", "bad", nil, nil}}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := mapPetRow(tc.row)
			if tc.wantErr {
				if err == nil || p != nil {
					t.Fatalf("expected error, got %v %v", p, err)
				}
				if tc.row.scanErr != nil && !errors.Is(err, tc.row.scanErr) {
					t.Errorf("error not wrapped: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if id := p.GetID(); id == nil || *id != tc.wantID {
				t.Errorf("id = %v, want %d", id, tc.wantID)
			}
			if p.TypeID() != tc.wantType || p.OwnerIDValue() != tc.wantOwner {
				t.Errorf("fks = %d/%d, want %d/%d", p.TypeID(), p.OwnerIDValue(), tc.wantType, tc.wantOwner)
			}
		})
	}
}