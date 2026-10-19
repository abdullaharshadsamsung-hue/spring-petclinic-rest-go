package sqlstore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

type vtRows struct{}

func (r *vtRows) Columns() []string              { return []string{} }
func (r *vtRows) Close() error                   { return nil }
func (r *vtRows) Next(dest []driver.Value) error { return io.EOF }

type vtHandler struct {
	mu    sync.Mutex
	execs []string
	args  [][]driver.Value
}

type vtDriver struct{}

var (
	vtMu   sync.Mutex
	vtReg  = map[string]*vtHandler{}
	vtSeq  int
	vtOnce sync.Once
)

func (vtDriver) Open(name string) (driver.Conn, error) {
	vtMu.Lock()
	defer vtMu.Unlock()
	h, ok := vtReg[name]
	if !ok {
		return nil, errors.New("unknown dsn")
	}
	return &vtConn{h: h}, nil
}

type vtConn struct{ h *vtHandler }

func (c *vtConn) Prepare(q string) (driver.Stmt, error) { return &vtStmt{h: c.h, q: q}, nil }
func (c *vtConn) Close() error                          { return nil }
func (c *vtConn) Begin() (driver.Tx, error)             { return nil, errors.New("no tx") }

type vtStmt struct {
	h *vtHandler
	q string
}

type vtResult struct{}

func (vtResult) LastInsertId() (int64, error) { return 0, nil }
func (vtResult) RowsAffected() (int64, error) { return 1, nil }

func (s *vtStmt) Close() error  { return nil }
func (s *vtStmt) NumInput() int { return -1 }
func (s *vtStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.h.mu.Lock()
	defer s.h.mu.Unlock()
	s.h.execs = append(s.h.execs, s.q)
	s.h.args = append(s.h.args, args)
	return vtResult{}, nil
}
func (s *vtStmt) Query(args []driver.Value) (driver.Rows, error) { return &vtRows{}, nil }

func newVTDB(t *testing.T) (*sql.DB, *vtHandler) {
	t.Helper()
	vtOnce.Do(func() { sql.Register("visittestdb", vtDriver{}) })
	h := &vtHandler{}
	vtMu.Lock()
	vtSeq++
	dsn := fmt.Sprintf("vt-%d", vtSeq)
	vtReg[dsn] = h
	vtMu.Unlock()
	db, err := sql.Open("visittestdb", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, h
}

func TestVisitFindByPetIDUnknownPetReturnsEmpty(t *testing.T) {
	db, _ := newVTDB(t)
	got, err := NewVisitRepository(db).FindByPetID(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("want empty non-nil slice, got %v", got)
	}
}

func TestVisitFindByIDNotFound(t *testing.T) {
	db, _ := newVTDB(t)
	_, err := NewVisitRepository(db).FindByID(context.Background(), 5)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if !errors.Is(err, ErrObjectRetrievalFailure) {
		t.Fatalf("want ErrObjectRetrievalFailure, got %v", err)
	}
}

func TestVisitFindAllEmpty(t *testing.T) {
	db, _ := newVTDB(t)
	got, err := NewVisitRepository(db).FindAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %d", len(got))
	}
}

func TestVisitSaveValidation(t *testing.T) {
	db, _ := newVTDB(t)
	repo := NewVisitRepository(db)
	if err := repo.Save(context.Background(), nil); err == nil {
		t.Error("want error for nil visit")
	}
	if err := repo.Save(context.Background(), model.NewVisit()); err == nil {
		t.Error("want error for visit without pet")
	}
}

func TestVisitDelete(t *testing.T) {
	db, h := newVTDB(t)
	repo := NewVisitRepository(db)
	if err := repo.Delete(context.Background(), nil); err == nil {
		t.Error("want error for nil visit")
	}
	if err := repo.Delete(context.Background(), model.NewVisit()); err == nil {
		t.Error("want error for visit without id")
	}
	v := model.NewVisit()
	id := 9
	v.SetID(&id)
	if err := repo.Delete(context.Background(), v); err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.execs) != 1 || h.execs[0] != "DELETE FROM visits WHERE id = $1" {
		t.Fatalf("unexpected execs: %v", h.execs)
	}
	if len(h.args[0]) != 1 || h.args[0][0] != int64(9) {
		t.Fatalf("unexpected args: %v", h.args[0])
	}
}