package sqlstore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
)

type ptRule struct {
	match string
	cols  []string
	rows  [][]driver.Value
	err   error
}

type ptDB struct {
	rules   []ptRule
	queries []string
}

func (f *ptDB) find(q string) (ptRule, bool) {
	for _, r := range f.rules {
		if strings.Contains(q, r.match) {
			return r, true
		}
	}
	return ptRule{}, false
}

type ptDriver struct{}

func (ptDriver) Open(string) (driver.Conn, error) { return nil, errors.New("unsupported") }

type ptConnector struct{ db *ptDB }

func (c ptConnector) Connect(context.Context) (driver.Conn, error) { return &ptConn{db: c.db}, nil }
func (c ptConnector) Driver() driver.Driver                        { return ptDriver{} }

type ptConn struct{ db *ptDB }

func (c *ptConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no prepare") }
func (c *ptConn) Close() error                        { return nil }
func (c *ptConn) Begin() (driver.Tx, error)           { return ptTx{}, nil }

func (c *ptConn) QueryContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	c.db.queries = append(c.db.queries, q)
	r, ok := c.db.find(q)
	if !ok {
		return nil, errors.New("unexpected query: " + q)
	}
	if r.err != nil {
		return nil, r.err
	}
	return &ptRows{cols: r.cols, data: r.rows}, nil
}

type ptTx struct{}

func (ptTx) Commit() error   { return nil }
func (ptTx) Rollback() error { return nil }

type ptRows struct {
	cols []string
	data [][]driver.Value
	i    int
}

func (r *ptRows) Columns() []string { return r.cols }
func (r *ptRows) Close() error      { return nil }
func (r *ptRows) Next(dest []driver.Value) error {
	if r.i >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.i])
	r.i++
	return nil
}

func newPtDB(t *testing.T, f *ptDB) *sql.DB {
	t.Helper()
	db := sql.OpenDB(ptConnector{db: f})
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestPetRepository_FindPetTypes(t *testing.T) {
	f := &ptDB{rules: []ptRule{{match: "FROM types", cols: []string{"id", "name"},
		rows: [][]driver.Value{{int64(1), "cat"}, {int64(2), "dog"}}}}}
	repo := NewPetRepository(newPtDB(t, f), nil)
	got, err := repo.FindPetTypes(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 || got[0].GetName() != "cat" || got[1].GetID() == nil || *got[1].GetID() != 2 {
		t.Fatalf("unexpected types: %v", got)
	}
}