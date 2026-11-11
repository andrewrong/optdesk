package optionopenpg

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/openquote"
)

var listAt = time.Date(2026, 11, 12, 11, 20, 0, 0, time.UTC)

func TestShortAndErrorKeepListedAtAndSession(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"short", "error"} {
		s, f := newStore()
		err := s.PutLists(context.Background(), []openquote.ListRecord{{UnderlyingID: 5, Expiry: "2026-12-18", Class: "AAPL",
			TriedAt: listAt, Outcome: outcome}})
		if err != nil {
			t.Fatal(err)
		}
		if len(f.sqls) != 1 || strings.Contains(f.sqls[0], "listed_at = EXCLUDED") ||
			strings.Contains(f.sqls[0], "listed_session = EXCLUDED") {
			t.Fatalf("%s: statement overwrites the listing: %v", outcome, f.sqls)
		}
		if got := f.args[0][len(f.args[0])-1]; got != outcome {
			t.Errorf("%s: outcome arg = %v", outcome, got)
		}
	}
}

func TestListedWritesEveryColumn(t *testing.T) {
	t.Parallel()
	s, f := newStore()
	err := s.PutLists(context.Background(), []openquote.ListRecord{{UnderlyingID: 5, Expiry: "2026-12-18", Class: "AAPL",
		ListedAt: listAt, ListedSession: "2026-11-12", Count: 516, TriedAt: listAt, Outcome: "listed"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.sqls[0], "listed_session = EXCLUDED.listed_session") || !strings.Contains(f.sqls[0], "count = EXCLUDED.count") {
		t.Fatalf("listed must update all columns: %s", f.sqls[0])
	}
	if f.args[0][5] != 516 {
		t.Errorf("args = %v", f.args[0])
	}
}

func TestPutListRunsNeverReplacesNonEmptyExpected(t *testing.T) {
	t.Parallel()
	s, f := newStore()
	err := s.PutListRuns(context.Background(), []openquote.ListRunRecord{{UnderlyingID: 5, Session: "2026-11-12",
		Expected: []string{"2026-12-18|AAPL"}, Listed: 1, Reason: "none", UpdatedAt: listAt}})
	if err != nil {
		t.Fatal(err)
	}
	q := f.sqls[0]
	for _, want := range []string{"expected IS NULL", "cardinality(master.option_list_run.expected) = 0",
		"cardinality(EXCLUDED.expected) > 0", "ELSE master.option_list_run.expected"} {
		if !strings.Contains(q, want) {
			t.Errorf("guard lacks %q: %s", want, q)
		}
	}
}

func TestPutListRunsWritesListedAsGiven(t *testing.T) {
	t.Parallel()
	s, f := newStore()
	err := s.PutListRuns(context.Background(), []openquote.ListRunRecord{{UnderlyingID: 5, Session: "2026-11-12",
		Expected: []string{}, Listed: 7, Reason: "some_short", UpdatedAt: listAt}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f.sqls[0], "listed = master.option_list_run.listed") || !strings.Contains(f.sqls[0], "listed = EXCLUDED.listed") {
		t.Errorf("listed is not the given value: %s", f.sqls[0])
	}
	if f.args[0][3] != 7 || f.args[0][4] != "some_short" {
		t.Errorf("args = %v", f.args[0])
	}
}

func TestListRunsAsksOnlyTheSession(t *testing.T) {
	t.Parallel()
	s, f := newStore()
	got, err := s.ListRuns(context.Background(), "2026-11-12")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
	d, _ := f.args[0][0].(time.Time)
	if !strings.Contains(f.sqls[0], "WHERE session = $1") || d.Format(time.DateOnly) != "2026-11-12" {
		t.Errorf("query %s args %v", f.sqls[0], f.args[0])
	}
	if _, err := s.ListRuns(context.Background(), "nope"); kind(err) != "Permanent" && err == nil {
		t.Errorf("bad day must fail: %v", err)
	}
}

func TestEmptyListWritesMakeNoDatabaseCall(t *testing.T) {
	t.Parallel()
	s, f := newStore()
	if err := s.PutLists(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if err := s.PutListRuns(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if f.inTx != 0 || len(f.sqls) != 0 {
		t.Fatalf("calls: %d, %v", f.inTx, f.sqls)
	}
}
