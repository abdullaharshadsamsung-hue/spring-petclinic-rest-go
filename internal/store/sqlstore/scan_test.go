package sqlstore

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"migrated-app/internal/model"
)

// fakeRows is an in-memory rowIterator / rowScanner that mimics *sql.Rows.
type fakeRows struct {
	rows    [][]any
	pos     int
	scanErr error
	iterErr error
}

func (f *fakeRows) Next() bool {
	if f.pos >= len(f.rows) {
		return false
	}
	f.pos++
	return true
}

func (f *fakeRows) Err() error { return f.iterErr }

func (f *fakeRows) Scan(dest ...any) error {
	if f.scanErr != nil {
		return f.scanErr
	}
	idx := f.pos - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(f.rows) {
		return errors.New("no row")
	}
	row := f.rows[idx]
	if len(row) != len(dest) {
		return fmt.Errorf("sql: expected %d destination arguments in Scan, not %d", len(row), len(dest))
	}
	for i, d := range dest {
		v := row[i]
		switch p := d.(type) {
		case *int:
			if v == nil {
				return fmt.Errorf("converting NULL to int is unsupported")
			}
			*p = v.(int)
		case *sql.NullString:
			if v == nil {
				*p = sql.NullString{}
			} else {
				*p = sql.NullString{String: v.(string), Valid: true}
			}
		case *sql.NullInt64:
			if v == nil {
				*p = sql.NullInt64{}
			} else {
				*p = sql.NullInt64{Int64: int64(v.(int)), Valid: true}
			}
		case **model.Date:
			if v == nil {
				*p = nil
			} else {
				dd := v.(model.Date)
				*p = &dd
			}
		default:
			return fmt.Errorf("unsupported dest %T", d)
		}
	}
	return nil
}

// single wraps a single row for QueryRow-style scanning.
func single(row ...any) *fakeRows { return &fakeRows{rows: [][]any{row}, pos: 1} }

func petVisitRow(petID int, name string, visitsPetID any, visitID any, date any, desc any) []any {
	return []any{petID, name, nil, 1, 10, visitsPetID, visitID, date, desc}
}

// visitsOf finds the []*model.Visit stored inside a pet via reflection, so the
// test does not depend on Pet's accessor names or sort order.
func visitsOf(t *testing.T, p *jdbcPet) []*model.Visit {
	t.Helper()
	target := reflect.TypeOf([]*model.Visit{})
	var found []*model.Visit
	var ok bool
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		if ok {
			return
		}
		for v.Kind() == reflect.Ptr {
			if v.IsNil() {
				return
			}
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			if f.Type() == target {
				found = reflect.NewAt(f.Type(), f.Addr().UnsafePointer()).Elem().Interface().([]*model.Visit)
				ok = true
				return
			}
			if f.Kind() == reflect.Struct {
				walk(f)
			}
		}
	}
	v := reflect.ValueOf(&p.Pet).Elem()
	walk(v)
	if !ok {
		return nil
	}
	return found
}

func petID(p *jdbcPet) int {
	if id := p.GetID(); id != nil {
		return *id
	}
	return -1
}

func TestMapPetRow(t *testing.T) {
	bd := model.NewDate(2010, time.March, 4)
	tests := []struct {
		name      string
		rs        *fakeRows
		wantErr   bool
		wantID    int
		wantType  int
		wantOwner int
		wantBD    *model.Date
	}{
		{"full row", single(7, "Leo", bd, 2, 3), false, 7, 2, 3, &bd},
		{"null fks and birth date", single(8, "Max", nil, nil, nil), false, 8, 0, 0, nil},
		{"zero id", single(0, "Zed", nil, 1, 1), false, 0, 1, 1, nil},
		{"missing column", single(7, "Leo", nil, 2), true, 0, 0, 0, nil},
		{"scan error", &fakeRows{rows: [][]any{{}}, pos: 1, scanErr: errors.New("boom")}, true, 0, 0, 0, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := mapPetRow(tc.rs)
			if tc.wantErr {
				if err == nil || p != nil {
					t.Fatalf("expected error, got %v %v", p, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if petID(p) != tc.wantID {
				t.Errorf("id=%d want %d", petID(p), tc.wantID)
			}
			if p.TypeID() != tc.wantType || p.OwnerIDValue() != tc.wantOwner {
				t.Errorf("fks=%d,%d", p.TypeID(), p.OwnerIDValue())
			}
			got := p.GetBirthDate()
			if (got == nil) != (tc.wantBD == nil) || (got != nil && !got.Equal(*tc.wantBD)) {
				t.Errorf("birth date=%v want %v", got, tc.wantBD)
			}
		})
	}
}

func TestMapPetRowIDNotShared(t *testing.T) {
	a, _ := mapPetRow(single(1, "a", nil, 1, 1))
	b, _ := mapPetRow(single(2, "b", nil, 1, 1))
	if a.GetID() == b.GetID() || *a.GetID() != 1 || *b.GetID() != 2 {
		t.Fatal("ids shared or wrong")
	}
}

func TestScanVisit(t *testing.T) {
	d := model.NewDate(2013, time.January, 1)
	tests := []struct {
		name     string
		rs       *fakeRows
		wantErr  bool
		wantID   *int
		wantDate *model.Date
		wantDesc string
	}{
		{"full", single(5, d, "rabies shot"), false, intp(5), &d, "rabies shot"},
		{"null date and description", single(6, nil, nil), false, intp(6), nil, ""},
		{"null id", single(nil, d, "x"), false, nil, &d, "x"},
		{"missing column", single(5, d), true, nil, nil, ""},
		{"scan error", &fakeRows{rows: [][]any{{}}, pos: 1, scanErr: errors.New("boom")}, true, nil, nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, err := scanVisit(tc.rs)
			if tc.wantErr {
				if err == nil || v != nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			gid := v.GetID()
			if (gid == nil) != (tc.wantID == nil) || (gid != nil && *gid != *tc.wantID) {
				t.Errorf("id=%v want %v", gid, tc.wantID)
			}
			gd := v.GetDate()
			if (gd == nil) != (tc.wantDate == nil) || (gd != nil && !gd.Equal(*tc.wantDate)) {
				t.Errorf("date=%v want %v", gd, tc.wantDate)
			}
			if v.GetDescription() != tc.wantDesc {
				t.Errorf("desc=%q", v.GetDescription())
			}
		})
	}
}

func intp(i int) *int { return &i }

func TestScanErrorWrapped(t *testing.T) {
	sentinel := errors.New("sentinel")
	_, err := scanVisit(&fakeRows{rows: [][]any{{}}, pos: 1, scanErr: sentinel})
	if !errors.Is(err, sentinel) {
		t.Errorf("not wrapped: %v", err)
	}
	_, err = mapPetRow(&fakeRows{rows: [][]any{{}}, pos: 1, scanErr: sentinel})
	if !errors.Is(err, sentinel) {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestExtractPetsWithVisits(t *testing.T) {
	d1 := model.NewDate(2013, time.January, 1)
	d2 := model.NewDate(2013, time.January, 2)
	type wantPet struct {
		id    int
		descs []string
	}
	tests := []struct {
		name string
		rows [][]any
		want []wantPet
	}{
		{"empty result set", nil, []wantPet{}},
		{"one pet no visits", [][]any{petVisitRow(7, "Leo", nil, nil, nil, nil)}, []wantPet{{7, nil}}},
		{"one pet N visits in order", [][]any{
			petVisitRow(7, "Leo", 7, 1, d1, "a"),
			petVisitRow(7, "Leo", 7, 2, d2, "b"),
			petVisitRow(7, "Leo", 7, 3, d1, "c"),
		}, []wantPet{{7, []string{"a", "b", "c"}}}},
		{"multiple pets grouped", [][]any{
			petVisitRow(1, "A", 1, 10, d1, "a1"),
			petVisitRow(2, "B", nil, nil, nil, nil),
			petVisitRow(3, "C", 3, 11, d1, "c1"),
			petVisitRow(3, "C", 3, 12, d2, "c2"),
		}, []wantPet{{1, []string{"a1"}}, {2, nil}, {3, []string{"c1", "c2"}}}},
		{"non-contiguous rows duplicate pet", [][]any{
			petVisitRow(1, "A", 1, 10, d1, "x"),
			petVisitRow(2, "B", 2, 11, d1, "y"),
			petVisitRow(1, "A", 1, 12, d1, "z"),
		}, []wantPet{{1, []string{"x"}}, {2, []string{"y"}}, {1, []string{"z"}}}},
		{"pets_id zero", [][]any{petVisitRow(0, "Z", nil, nil, nil, nil)}, []wantPet{{0, nil}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fr := &fakeRows{rows: tc.rows}
			pets, err := extractPetsWithVisits(fr)
			if err != nil {
				t.Fatal(err)
			}
			if pets == nil {
				t.Fatal("result must not be nil")
			}
			if len(pets) != len(tc.want) {
				t.Fatalf("got %d pets want %d", len(pets), len(tc.want))
			}
			for i, w := range tc.want {
				p := pets[i]
				if petID(p) != w.id {
					t.Errorf("pet %d id=%d want %d", i, petID(p), w.id)
				}
				vs := visitsOf(t, p)
				if len(vs) != len(w.descs) {
					t.Fatalf("pet %d has %d visits want %d", i, len(vs), len(w.descs))
				}
				for j, v := range vs {
					if v == nil {
						t.Fatal("nil visit")
					}
					if v.GetDescription() != w.descs[j] {
						t.Errorf("visit %d desc=%q want %q", j, v.GetDescription(), w.descs[j])
					}
				}
			}
			if len(fr.rows) != len(tc.rows) {
				t.Error("rows modified")
			}
		})
	}
}

func TestExtractPetsWithVisitsErrors(t *testing.T) {
	sentinel := errors.New("iter fail")
	tests := []struct {
		name string
		fr   *fakeRows
		is   error
	}{
		{"missing visits_pet_id column", &fakeRows{rows: [][]any{{7, "Leo", nil, 1, 1, 1, nil, "x"}}}, nil},
		{"missing pets columns", &fakeRows{rows: [][]any{{7}}}, nil},
		{"scan error", &fakeRows{rows: [][]any{petVisitRow(1, "A", nil, nil, nil, nil)}, scanErr: sentinel}, sentinel},
		{"iteration error", &fakeRows{iterErr: sentinel}, sentinel},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pets, err := extractPetsWithVisits(tc.fr)
			if err == nil {
				t.Fatal("expected error")
			}
			if pets != nil {
				t.Error("expected nil pets on error")
			}
			if tc.is != nil && !errors.Is(err, tc.is) {
				t.Errorf("error not wrapped: %v", err)
			}
		})
	}
}

func TestExtractPetsWithVisitsStateless(t *testing.T) {
	rows := [][]any{petVisitRow(1, "A", 1, 10, model.NewDate(2013, time.January, 1), "a")}
	for i := 0; i < 2; i++ {
		pets, err := extractPetsWithVisits(&fakeRows{rows: rows})
		if err != nil || len(pets) != 1 || len(visitsOf(t, pets[0])) != 1 {
			t.Fatalf("run %d: %v %v", i, pets, err)
		}
	}
}

func TestColumnConstants(t *testing.T) {
	tests := []struct{ got, want string }{
		{joinedPetColumns, "pets.id AS pets_id, pets.name, pets.birth_date, pets.type_id, pets.owner_id"},
		{visitColumns, "visits.id AS visit_id, visits.visit_date, visits.description"},
		{petVisitColumns, joinedPetColumns + ", visits.pet_id AS visits_pet_id, " + visitColumns},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("got %q want %q", tc.got, tc.want)
		}
	}
}

func TestJdbcPetAccessors(t *testing.T) {
	p := &jdbcPet{}
	p.SetTypeID(4)
	p.SetOwnerIDValue(9)
	if p.TypeID() != 4 || p.OwnerIDValue() != 9 {
		t.Fatal("accessors broken")
	}
}