package sqlstore

import (
	"database/sql"
	"testing"
)

type vetTestScanner struct {
	vals []any
	err  error
}

func (s vetTestScanner) Scan(dest ...any) error {
	if s.err != nil {
		return s.err
	}
	for i, d := range dest {
		switch p := d.(type) {
		case *int:
			*p = s.vals[i].(int)
		case *sql.NullString:
			if s.vals[i] == nil {
				*p = sql.NullString{}
			} else {
				*p = sql.NullString{String: s.vals[i].(string), Valid: true}
			}
		}
	}
	return nil
}

func TestVetRepoUnit_ScanVet(t *testing.T) {
	v, err := scanVet(vetTestScanner{vals: []any{7, "Helen", nil}})
	if err != nil {
		t.Fatal(err)
	}
	if v.ID == nil || *v.ID != 7 || v.FirstName != "Helen" || v.LastName != "" {
		t.Fatalf("unexpected vet %+v", v)
	}
	if len(v.GetSpecialties()) != 0 {
		t.Fatal("expected no specialties")
	}
}