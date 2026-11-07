package migrate_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"os"
	"path/filepath"
	"slices"
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

func TestUpEmptyFSIsAnError(t *testing.T) {
	t.Parallel()
	d := &fakeDriver{}
	db := sql.OpenDB(d)
	t.Cleanup(func() { _ = db.Close() })
	if err := migrate.Up(context.Background(), db, fstest.MapFS{}); err == nil {
		t.Fatal("Up on an empty filesystem returned nil")
	}
}

// TestMigration0008 lists the migration files: 0008 follows 0007 and has an Up and a Down part, the Down part
// dropping the foreign keys before the tables.
func TestMigration0008(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("../../../migrations/*.sql")
	if err != nil || len(files) < 9 {
		t.Fatalf("migrations: %v, %v", files, err)
	}
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = filepath.Base(f)
	}
	n := slices.Index(names, "0008_master_instrument.sql")
	if n < 1 {
		t.Fatalf("0008 not found in %v", names)
	}
	if names[n-1] != "0007_quote_snapshot.sql" || names[n] != "0008_master_instrument.sql" {
		t.Fatalf("0008 follows %v", names[n-1:])
	}
	raw, err := os.ReadFile(files[n])
	if err != nil {
		t.Fatal(err)
	}
	up, down, ok := strings.Cut(string(raw), "-- +goose Down")
	if !ok || !strings.HasPrefix(up, "-- +goose Up") {
		t.Fatal("0008 lacks an Up or a Down part")
	}
	if !strings.Contains(up, "CREATE TABLE master.instrument ") || !strings.Contains(up, "CREATE TABLE master.instrument_identifier") {
		t.Fatal("Up part does not create the two tables")
	}
	lastKey := strings.LastIndex(down, "DROP CONSTRAINT")
	firstTable := strings.Index(down, "DROP TABLE")
	if lastKey < 0 || firstTable < 0 || lastKey > firstTable || strings.Count(down, "DROP CONSTRAINT") != 3 {
		t.Fatalf("Down part must drop the three keys before the tables:\n%s", down)
	}
}

// TestMigrationsEndWith0011 lists the migration files: 0010 and 0011 follow 0009; 0010 creates the name view for the
// page role and drops it, 0011 grants INSERT on the terms table to the master writer and revokes it.
func TestMigrationsEndWith0011(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("../../../migrations/*.sql")
	if err != nil || len(files) < 11 {
		t.Fatalf("migrations: %v, %v", files, err)
	}
	n := len(files)
	if filepath.Base(files[n-3]) != "0009_master_option_terms.sql" || filepath.Base(files[n-2]) != "0010_instrument_names_view.sql" ||
		filepath.Base(files[n-1]) != "0011_option_terms_writer.sql" {
		t.Fatalf("migrations end with %v", files[n-3:])
	}
	parts := func(path string) (string, string) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		up, down, ok := strings.Cut(string(raw), "-- +goose Down")
		if !ok || !strings.HasPrefix(up, "-- +goose Up") {
			t.Fatalf("%s lacks an Up or a Down part", path)
		}
		return up, down
	}
	up, down := parts(files[n-2])
	for _, want := range []string{"CREATE VIEW rd.instrument_r1", "instrument_id, instrument_type, name, currency",
		"FROM master.instrument", "ALTER VIEW rd.instrument_r1 OWNER TO ot_owner", "GRANT SELECT ON rd.instrument_r1 TO ot_ro_pages"} {
		if !strings.Contains(up, want) {
			t.Errorf("0010 Up part lacks %q", want)
		}
	}
	if strings.Count(up, "GRANT") != 1 || !strings.Contains(down, "DROP VIEW rd.instrument_r1") {
		t.Fatalf("0010: one grant expected; Down must drop the view")
	}
	up, down = parts(files[n-1])
	if strings.Count(up, "GRANT") != 1 || !strings.Contains(up, "GRANT INSERT ON master.option_terms TO ot_w_master;") ||
		strings.Count(down, "REVOKE") != 1 || !strings.Contains(down, "REVOKE INSERT ON master.option_terms FROM ot_w_master;") {
		t.Fatalf("0011: one grant of INSERT in Up and its revoke in Down expected")
	}
}
