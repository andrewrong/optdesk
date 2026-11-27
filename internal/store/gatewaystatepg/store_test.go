package gatewaystatepg

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/gatewaystate"
)

type fakeDB struct {
	sqls []string
	args [][]any
	rows [][4]any
}

func (f *fakeDB) Exec(_ context.Context, sql string, args ...any) error {
	f.sqls = append(f.sqls, sql)
	f.args = append(f.args, args)
	return nil
}

func (f *fakeDB) Select(_ context.Context, sql string, row func(func(...any) error) error) error {
	f.sqls = append(f.sqls, sql)
	for _, r := range f.rows {
		if err := row(func(dest ...any) error {
			*(dest[0].(*string)) = r[0].(string)
			*(dest[1].(*bool)) = r[1].(bool)
			*(dest[2].(*time.Time)) = r[2].(time.Time)
			*(dest[3].(*time.Time)) = r[3].(time.Time)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) Close() error { return nil }

func TestObserveStatementKeepsSinceOnSameFlag(t *testing.T) {
	t.Parallel()
	for _, want := range []string{
		"INSERT INTO ops.gateway_state",
		"ON CONFLICT (alias) DO UPDATE",
		"since = CASE WHEN g.down <> EXCLUDED.down THEN EXCLUDED.since ELSE g.since END",
		"WHERE g.observed_at <= EXCLUDED.observed_at",
	} {
		if !strings.Contains(observeSQL, want) {
			t.Errorf("observe statement lacks %q", want)
		}
	}
	// since is assigned once, and only inside the CASE keyed on a changed down.
	if n := strings.Count(observeSQL, "since ="); n != 1 {
		t.Errorf("since assigned %d times, want 1", n)
	}
	if strings.Contains(observeSQL, "now()") || strings.Contains(observeSQL, "CURRENT_TIMESTAMP") {
		t.Error("statement reads the database clock")
	}
}

func TestObservePassesTheCallersTime(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	at := time.Date(2026, 11, 27, 9, 30, 0, 0, time.UTC)
	if err := (&Store{db: f}).Observe(context.Background(), "main", true, at); err != nil {
		t.Fatal(err)
	}
	if len(f.args) != 1 || f.args[0][0] != "main" || f.args[0][1] != true || f.args[0][2] != at {
		t.Fatalf("args = %v", f.args)
	}
}

func TestStatesReadsTheViewAndIsNonNil(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	got, err := (&Store{db: f}).States(context.Background())
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
	if f.sqls[0] != selectSQL || !strings.Contains(selectSQL, "rd.gateway_state_r1") || strings.Contains(selectSQL, "ops.") {
		t.Errorf("select = %q", f.sqls[0])
	}
	tm := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	f.rows = [][4]any{{"main", true, tm, tm}}
	got, _ = (&Store{db: f}).States(context.Background())
	if len(got) != 1 || got[0] != (gatewaystate.State{Alias: "main", Down: true, Since: tm, ObservedAt: tm}) {
		t.Errorf("got %+v", got)
	}
}

func TestOpenWithEmptyDSNFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if s, err := Open(ctx, ""); err == nil || s != nil {
		t.Errorf("Open: %v, %v", s, err)
	}
	if r, err := OpenReader(ctx, ""); err == nil || r != nil {
		t.Errorf("OpenReader: %v, %v", r, err)
	}
	if w, err := OpenWriter(ctx, ""); err == nil || w != nil {
		t.Errorf("OpenWriter: %v, %v", w, err)
	}
}
