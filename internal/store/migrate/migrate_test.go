package migrate_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/andrewrong/optdesk/internal/store/migrate"
)

// fakeDriver records every statement it is given and answers every query with no rows.
type fakeDriver struct {
	mu    sync.Mutex
	calls []string
}

func (d *fakeDriver) record(kind, q string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, kind+": "+strings.Join(strings.Fields(q), " "))
}

func (d *fakeDriver) log() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.calls...)
}

func (d *fakeDriver) Connect(context.Context) (driver.Conn, error) { return &fakeConn{d: d}, nil }
func (d *fakeDriver) Driver() driver.Driver                        { return nil }

type fakeConn struct{ d *fakeDriver }

func (c *fakeConn) Prepare(q string) (driver.Stmt, error) { return &fakeStmt{c: c, q: q}, nil }
func (c *fakeConn) Close() error                          { return nil }
func (c *fakeConn) Begin() (driver.Tx, error)             { c.d.record("begin", ""); return fakeTx{c.d}, nil }

type fakeTx struct{ d *fakeDriver }

func (t fakeTx) Commit() error   { t.d.record("commit", ""); return nil }
func (t fakeTx) Rollback() error { t.d.record("rollback", ""); return nil }

type fakeStmt struct {
	c *fakeConn
	q string
}

func (s *fakeStmt) Close() error  { return nil }
func (s *fakeStmt) NumInput() int { return -1 }
func (s *fakeStmt) Exec([]driver.Value) (driver.Result, error) {
	s.c.d.record("exec", s.q)
	return driver.RowsAffected(0), nil
}
func (s *fakeStmt) Query([]driver.Value) (driver.Rows, error) {
	s.c.d.record("query", s.q)
	if strings.Contains(s.q, "pg_tables") {
		return &existsRows{}, nil
	}
	if strings.Contains(s.q, "max(version_id)") {
		return &versionRows{cols: []string{"max"}}, nil
	}
	return &versionRows{cols: []string{"version_id", "is_applied"}}, nil
}

// existsRows answers the "does the version table exist" query with one true row.
type existsRows struct{ done bool }

func (*existsRows) Columns() []string { return []string{"exists"} }
func (*existsRows) Close() error      { return nil }
func (r *existsRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = true
	return nil
}

// versionRows answers the version-table queries with the initial zero version row, as goose inserts it.
type versionRows struct {
	cols []string
	n    int
}

func (r *versionRows) Columns() []string { return r.cols }
func (*versionRows) Close() error        { return nil }
func (r *versionRows) Next(dest []driver.Value) error {
	if r.n > 0 {
		return io.EOF
	}
	r.n++
	for i := range dest {
		dest[i] = int64(0)
	}
	if len(dest) > 1 {
		dest[1] = true
	}
	return nil
}

func TestUpRunsMigrationSQLThroughDriver(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"0001_one.sql": {Data: []byte("-- +goose Up\nCREATE TABLE one_marker (a int);\n-- +goose Down\nDROP TABLE one_marker;\n")},
		"0002_two.sql": {Data: []byte("-- +goose Up\nCREATE TABLE two_marker (a int);\n-- +goose Down\nDROP TABLE two_marker;\n")},
	}
	d := &fakeDriver{}
	db := sql.OpenDB(d)
	t.Cleanup(func() { _ = db.Close() })

	if err := migrate.Up(context.Background(), db, fsys); err != nil {
		t.Fatalf("Up: %v", err)
	}

	calls := d.log()
	idx := func(sub string) int {
		for i, c := range calls {
			if strings.Contains(c, sub) {
				return i
			}
		}
		return -1
	}
	one, two := idx("CREATE TABLE one_marker"), idx("CREATE TABLE two_marker")
	if one < 0 || two < 0 {
		t.Fatalf("migration statements not executed, calls: %q", calls)
	}
	if one > two {
		t.Errorf("0001 ran after 0002, calls: %q", calls)
	}
	if idx("goose_db_version") < 0 {
		t.Errorf("version table never touched, calls: %q", calls)
	}
}

func TestUpConnectorMigratesAndCloses(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"0001_one.sql": {Data: []byte("-- +goose Up\nCREATE TABLE one_marker (a int);\n-- +goose Down\nDROP TABLE one_marker;\n")},
	}
	d := &fakeDriver{}
	if err := migrate.UpConnector(context.Background(), d, fsys); err != nil {
		t.Fatalf("UpConnector: %v", err)
	}
	found := false
	for _, c := range d.log() {
		if strings.Contains(c, "CREATE TABLE one_marker") {
			found = true
		}
	}
	if !found {
		t.Errorf("migration not executed: %q", d.log())
	}
}

func TestUpEmptyFSIsAnError(t *testing.T) {
	t.Parallel()
	d := &fakeDriver{}
	db := sql.OpenDB(d)
	t.Cleanup(func() { _ = db.Close() })
	if err := migrate.Up(context.Background(), db, fstest.MapFS{}); err == nil {
		t.Fatal("Up on an empty filesystem returned nil")
	}
}
