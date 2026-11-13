package hoursnoticepg

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store/hoursnotice"
)

// migrationFile is the migration of this store, relative to the package directory.
const migrationFile = "../../../migrations/0040_session_hours_notice.sql"

type fakeDB struct {
	events []string // "begin", the SQL of each statement, "commit" or "rollback"
	failAt int      // 1-based statement that fails, 0 for none
	n      int
	rows   [][4]string
}

type fakeTx struct{ f *fakeDB }

func (t fakeTx) Exec(_ context.Context, sql string, _ ...any) error {
	t.f.n++
	t.f.events = append(t.f.events, sql)
	if t.f.failAt == t.f.n {
		return errors.New("boom")
	}
	return nil
}

func (f *fakeDB) InTx(_ context.Context, fn func(execer) error) error {
	f.events = append(f.events, "begin")
	if err := fn(fakeTx{f}); err != nil {
		f.events = append(f.events, "rollback")
		return err
	}
	f.events = append(f.events, "commit")
	return nil
}

func (f *fakeDB) Select(_ context.Context, sql string, row func(func(...any) error) error) error {
	f.events = append(f.events, sql)
	for _, r := range f.rows {
		r := r
		if err := row(func(dest ...any) error {
			for i, d := range dest {
				*(d.(*string)) = r[i]
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) Close() error { return nil }

func TestReplaceRunsDeleteAndInsertsInOneTransaction(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	s := &Store{db: f}
	rows := []hoursnotice.Notice{{Market: "XNYS", Day: "2026-11-26", Ours: "a", IBKR: "b"}, {Market: "XNYS", Day: "2026-11-27", Ours: "c", IBKR: "d"}}
	if err := s.Replace(context.Background(), "XNYS", rows); err != nil {
		t.Fatal(err)
	}
	want := []string{"begin", deleteSQL, insertSQL, insertSQL, "commit"}
	if strings.Join(f.events, "|") != strings.Join(want, "|") {
		t.Errorf("events = %q, want %q", f.events, want)
	}
}

func TestReplaceRollsBackWhenAnInsertFails(t *testing.T) {
	t.Parallel()
	f := &fakeDB{failAt: 3}
	rows := []hoursnotice.Notice{{Market: "XNYS", Day: "2026-11-26"}, {Market: "XNYS", Day: "2026-11-27"}}
	if err := (&Store{db: f}).Replace(context.Background(), "XNYS", rows); err == nil {
		t.Fatal("no error")
	}
	if f.events[len(f.events)-1] != "rollback" {
		t.Errorf("events = %q, want rollback last", f.events)
	}
}

func TestReplaceRefusesARowOfAnotherMarketBeforeAnyStatement(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	err := (&Store{db: f}).Replace(context.Background(), "XNYS", []hoursnotice.Notice{{Market: "XTKS", Day: "2026-11-26"}})
	if err == nil || len(f.events) != 0 {
		t.Errorf("err = %v, events = %q", err, f.events)
	}
}

func TestNoticesReadsTheViewInOrder(t *testing.T) {
	t.Parallel()
	f := &fakeDB{rows: [][4]string{{"XNYS", "2026-11-27", "a", "b"}}}
	got, err := (&Store{db: f}).Notices(context.Background())
	if err != nil || len(got) != 1 || got[0] != (hoursnotice.Notice{Market: "XNYS", Day: "2026-11-27", Ours: "a", IBKR: "b"}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	if f.events[0] != selectSQL || !strings.Contains(selectSQL, "rd.session_hours_notice_r1") || strings.Contains(selectSQL, "master.") {
		t.Errorf("select = %q", f.events[0])
	}
}

// The grants of the migration, read from its SQL text (no database).
func TestMigrationGrants(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(migrationFile)
	if err != nil {
		t.Fatal(err)
	}
	up, down, ok := strings.Cut(string(raw), "-- +goose Down")
	if !ok || !strings.Contains(down, "DROP TABLE master.session_hours_notice") {
		t.Fatal("migration has no Down")
	}
	for _, want := range []string{
		`GRANT INSERT, DELETE, SELECT ON master.session_hours_notice TO ot_w_master;`,
		`GRANT SELECT ON rd.session_hours_notice_r1 TO ot_ro_pages`,
	} {
		if !strings.Contains(up, want) {
			t.Errorf("missing %q", want)
		}
	}
	// ot_ro_pages has no grant on the table.
	for _, g := range regexp.MustCompile(`(?s)GRANT[^;]*;`).FindAllString(up, -1) {
		if strings.Contains(g, "ot_ro_pages") && strings.Contains(g, "master.session_hours_notice") {
			t.Errorf("ot_ro_pages granted on the table: %s", g)
		}
	}
	if strings.Contains(up, "cleared_at") || strings.Contains(up, "CHECK") {
		t.Error("migration has cleared_at or a CHECK")
	}
}
