package sqlstore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"sync"
	"testing"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

// ---- fake database/sql driver ----

type fakeResult struct {
	cols []string
	data [][]driver.Value
	err  error
}

type handlerFunc func(query string, args []driver.Value) fakeResult

type fakeState struct {
	mu      sync.Mutex
	handler handlerFunc
	log     []string
	args    [][]driver.Value
}

func (s *fakeState) record(e string, a []driver.Value) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log = append(s.log, e)
	s.args = append(s.args, a)
}

func (s *fakeState) queries() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.log...)
}

type fakeConnector struct{ s *fakeState }

func (c *fakeConnector) Connect(context.Context) (driver.Conn, error) { return &fakeConn{s: c.s}, nil }
func (c *fakeConnector) Driver() driver.Driver                       { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return nil, errors.New("not supported") }

type fakeConn struct{ s *fakeState }

func (c *fakeConn) Prepare(q string) (driver.Stmt, error) { return &fakeStmt{s: c.s, q: q}, nil }
func (c *fakeConn) Close() error                          { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) {
	c.s.record("BEGIN", nil)
	r := c.s.handler("BEGIN", nil)
	if r.err != nil {
		return nil, r.err
	}
	return &fakeTx{s: c.s}, nil
}

func toValues(nv []driver.NamedValue) []driver.Value {
	out := make([]driver.Value, len(nv))
	for i, v := range nv {
		out[i] = v.Value
	}
	return out
}

func (c *fakeConn) QueryContext(_ context.Context, q string, nv []driver.NamedValue) (driver.Rows, error) {
	a := toValues(nv)
	c.s.record(q, a)
	r := c.s.handler(q, a)
	if r.err != nil {
		return nil, r.err
	}
	return &fakeRows{cols: r.cols, data: r.data}, nil
}

func (c *fakeConn) ExecContext(_ context.Context, q string, nv []driver.NamedValue) (driver.Result, error) {
	a := toValues(nv)
	c.s.record(q, a)
	r := c.s.handler(q, a)
	if r.err != nil {
		return nil, r.err
	}
	return driver.RowsAffected(1), nil
}

type fakeStmt struct {
	s *fakeState
	q string
}

func (st *fakeStmt) Close() error  { return nil }
func (st *fakeStmt) NumInput() int { return -1 }
func (st *fakeStmt) Exec(a []driver.Value) (driver.Result, error) {
	st.s.record(st.q, a)
	r := st.s.handler(st.q, a)
	if r.err != nil {
		return nil, r.err
	}
	return driver.RowsAffected(1), nil
}
func (st *fakeStmt) Query(a []driver.Value) (driver.Rows, error) {
	st.s.record(st.q, a)
	r := st.s.handler(st.q, a)
	if r.err != nil {
		return nil, r.err
	}
	return &fakeRows{cols: r.cols, data: r.data}, nil
}

type fakeTx struct{ s *fakeState }

func (t *fakeTx) Commit() error {
	t.s.record("COMMIT", nil)
	return t.s.handler("COMMIT", nil).err
}
func (t *fakeTx) Rollback() error {
	t.s.record("ROLLBACK", nil)
	return nil
}

type fakeRows struct {
	cols []string
	data [][]driver.Value
	i    int
}

func (r *fakeRows) Columns() []string { return r.cols }
func (r *fakeRows) Close() error      { return nil }

func newFakeRepo(t *testing.T, h handlerFunc) (*SpecialtyRepository, *fakeState) {
	t.Helper()
	s := &fakeState{handler: h}
	db := sql.OpenDB(&fakeConnector{s: s})
	t.Cleanup(func() { db.Close() })
	return NewSpecialtyRepository(db), s
}

func idName(rows ...[]driver.Value) fakeResult {
	return fakeResult{cols: []string{"id", "name"}, data: rows}
}

func intPtr(i int) *int { return &i }

var errDB = errors.New("db boom")

// ---- tests ----

func TestFindByID(t *testing.T) {
	tests := []struct {
		name     string
		res      fakeResult
		wantName string
		notFound bool
		wantErr  error
	}{
		{name: "found", res: idName([]driver.Value{int64(3), "dentistry"}), wantName: "dentistry"},
		{name: "null name", res: idName([]driver.Value{int64(3), nil}), wantName: ""},
		{name: "missing", res: idName(), notFound: true},
		{name: "db error", res: fakeResult{err: errDB}, wantErr: errDB},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, st := newFakeRepo(t, func(q string, a []driver.Value) fakeResult { return tc.res })
			sp, err := repo.FindByID(context.Background(), 3)
			for _, q := range st.queries() {
				if !strings.HasPrefix(strings.TrimSpace(q), "SELECT") {
					t.Errorf("non read-only query: %q", q)
				}
			}
			switch {
			case tc.notFound:
				if sp != nil || err == nil {
					t.Fatalf("expected not found, got %v %v", sp, err)
				}
				if !errors.Is(err, store.ErrNotFound) || !errors.Is(err, ErrObjectRetrievalFailure) {
					t.Errorf("error does not match sentinels: %v", err)
				}
				var orf *ObjectRetrievalFailureError
				if !errors.As(err, &orf) || orf.ID != 3 || !strings.Contains(orf.EntityType, "Specialty") {
					t.Errorf("bad retrieval error: %v", err)
				}
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) || errors.Is(err, store.ErrNotFound) {
					t.Errorf("got %v", err)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				if sp.GetID() == nil || *sp.GetID() != 3 || sp.GetName() != tc.wantName {
					t.Errorf("got id=%v name=%q", sp.GetID(), sp.GetName())
				}
			}
		})
	}
}

func TestFindByNameIn(t *testing.T) {
	tests := []struct {
		name          string
		input         []string
		res           fakeResult
		wantNames     []string
		wantQueries   int
		wantArgs      int
		wantPlacehold string
		wantErr       bool
	}{
		{name: "empty input no query", input: nil, wantNames: []string{}, wantQueries: 0},
		{name: "empty slice no query", input: []string{}, wantNames: []string{}, wantQueries: 0},
		{
			name: "some match, deduped", input: []string{"radiology", "surgery", "radiology", "unknown"},
			res:         idName([]driver.Value{int64(1), "radiology"}, []driver.Value{int64(2), "surgery"}),
			wantNames:   []string{"radiology", "surgery"},
			wantQueries: 1, wantArgs: 3, wantPlacehold: "IN ($1, $2, $3)",
		},
		{
			name: "none match", input: []string{"nope"}, res: idName(),
			wantNames: []string{}, wantQueries: 1, wantArgs: 1, wantPlacehold: "IN ($1)",
		},
		{name: "db error", input: []string{"x"}, res: fakeResult{err: errDB}, wantQueries: 1, wantArgs: 1, wantErr: true},
		{
			name: "scan error", input: []string{"x"},
			res:         fakeResult{cols: []string{"id"}, data: [][]driver.Value{{int64(1)}}},
			wantQueries: 1, wantArgs: 1, wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, st := newFakeRepo(t, func(q string, a []driver.Value) fakeResult { return tc.res })
			got, err := repo.FindByNameIn(context.Background(), tc.input)
			qs := st.queries()
			if len(qs) != tc.wantQueries {
				t.Fatalf("queries=%v", qs)
			}
			if tc.wantQueries > 0 {
				if len(st.args[0]) != tc.wantArgs {
					t.Errorf("args=%v", st.args[0])
				}
				if tc.wantPlacehold != "" && !strings.Contains(qs[0], tc.wantPlacehold) {
					t.Errorf("query %q", qs[0])
				}
			}
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || len(got) != len(tc.wantNames) {
				t.Fatalf("got %v", got)
			}
			set := map[string]bool{}
			for _, n := range tc.input {
				set[n] = true
			}
			for i, s := range got {
				if s.GetName() != tc.wantNames[i] || !set[s.GetName()] {
					t.Errorf("unexpected %q", s.GetName())
				}
			}
		})
	}
}

func TestFindAll(t *testing.T) {
	tests := []struct {
		name    string
		res     fakeResult
		want    []string
		wantErr error
	}{
		{name: "rows", res: idName([]driver.Value{int64(1), "radiology"}, []driver.Value{int64(2), "surgery"}, []driver.Value{int64(3), "dentistry"}), want: []string{"radiology", "surgery", "dentistry"}},
		{name: "empty", res: idName(), want: []string{}},
		{name: "db failure", res: fakeResult{err: errDB}, wantErr: errDB},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, st := newFakeRepo(t, func(q string, a []driver.Value) fakeResult { return tc.res })
			got, err := repo.FindAll(context.Background())
			if q := st.queries(); len(q) != 1 || !strings.Contains(q[0], "ORDER BY id") {
				t.Errorf("queries %v", q)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || len(got) != len(tc.want) {
				t.Fatalf("got %v", got)
			}
			for i, s := range got {
				if s.GetName() != tc.want[i] || *s.GetID() != i+1 {
					t.Errorf("row %d: %v %q", i, *s.GetID(), s.GetName())
				}
			}
		})
	}
}

func TestSave(t *testing.T) {
	tests := []struct {
		name      string
		sp        func() *model.Specialty
		res       fakeResult
		wantErr   bool
		wantID    int
		wantQuery string
		wantArgs  int
	}{
		{name: "nil", sp: func() *model.Specialty { return nil }, wantErr: true},
		{
			name: "insert new", sp: func() *model.Specialty { s := model.NewSpecialty(); s.SetName("oncology"); return s },
			res: fakeResult{cols: []string{"id"}, data: [][]driver.Value{{int64(42)}}}, wantID: 42,
			wantQuery: "INSERT INTO specialties", wantArgs: 1,
		},
		{
			name: "insert failure", sp: func() *model.Specialty { s := model.NewSpecialty(); s.SetName("x"); return s },
			res: fakeResult{err: errDB}, wantErr: true, wantQuery: "INSERT INTO specialties", wantArgs: 1,
		},
		{
			name: "update existing", sp: func() *model.Specialty { s := model.NewSpecialty(); s.SetID(intPtr(7)); s.SetName("new"); return s },
			wantID: 7, wantQuery: "UPDATE specialties", wantArgs: 2,
		},
		{
			name: "update failure", sp: func() *model.Specialty { s := model.NewSpecialty(); s.SetID(intPtr(7)); s.SetName("n"); return s },
			res: fakeResult{err: errDB}, wantErr: true, wantQuery: "UPDATE specialties", wantArgs: 2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, st := newFakeRepo(t, func(q string, a []driver.Value) fakeResult { return tc.res })
			sp := tc.sp()
			err := repo.Save(context.Background(), sp)
			qs := st.queries()
			if tc.wantQuery != "" {
				if len(qs) != 1 || !strings.Contains(qs[0], tc.wantQuery) || len(st.args[0]) != tc.wantArgs {
					t.Fatalf("queries %v args %v", qs, st.args)
				}
				if st.args[0][0] != sp.GetName() {
					t.Errorf("name arg %v", st.args[0][0])
				}
			} else if len(qs) != 0 {
				t.Errorf("unexpected queries %v", qs)
			}
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if sp.GetID() == nil || *sp.GetID() != tc.wantID {
				t.Errorf("id %v", sp.GetID())
			}
		})
	}
}

func TestDelete(t *testing.T) {
	withID := func() *model.Specialty { s := model.NewSpecialty(); s.SetID(intPtr(5)); return s }
	tests := []struct {
		name    string
		sp      *model.Specialty
		failOn  string
		wantErr bool
		wantLog []string
	}{
		{name: "nil", sp: nil, wantErr: true},
		{name: "no id", sp: model.NewSpecialty(), wantErr: true},
		{name: "success", sp: withID(), wantLog: []string{"BEGIN", "DELETE FROM vet_specialties", "DELETE FROM specialties", "COMMIT"}},
		{name: "begin fails", sp: withID(), failOn: "BEGIN", wantErr: true, wantLog: []string{"BEGIN"}},
		{name: "join delete fails", sp: withID(), failOn: "DELETE FROM vet_specialties", wantErr: true, wantLog: []string{"BEGIN", "DELETE FROM vet_specialties", "ROLLBACK"}},
		{name: "specialty delete fails", sp: withID(), failOn: "DELETE FROM specialties", wantErr: true, wantLog: []string{"BEGIN", "DELETE FROM vet_specialties", "DELETE FROM specialties", "ROLLBACK"}},
		{name: "commit fails", sp: withID(), failOn: "COMMIT", wantErr: true, wantLog: []string{"BEGIN", "DELETE FROM vet_specialties", "DELETE FROM specialties", "COMMIT"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, st := newFakeRepo(t, func(q string, a []driver.Value) fakeResult {
				if tc.failOn != "" && strings.HasPrefix(q, tc.failOn) {
					return fakeResult{err: errDB}
				}
				return fakeResult{}
			})
			err := repo.Delete(context.Background(), tc.sp)
			if tc.wantErr != (err != nil) {
				t.Fatalf("err=%v", err)
			}
			qs := st.queries()
			if len(qs) != len(tc.wantLog) {
				t.Fatalf("log %v want %v", qs, tc.wantLog)
			}
			for i, w := range tc.wantLog {
				if !strings.HasPrefix(qs[i], w) {
					t.Errorf("step %d: %q want %q", i, qs[i], w)
				}
				if strings.HasPrefix(w, "DELETE") && (len(st.args[i]) != 1 || st.args[i][0] != int64(5)) {
					t.Errorf("args %v", st.args[i])
				}
				if strings.Contains(qs[i], "FROM vets") {
					t.Errorf("vets must not be deleted")
				}
			}
		})
	}
}