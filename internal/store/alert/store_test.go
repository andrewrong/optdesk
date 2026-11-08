package alert

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenSentFunc = Open

type fakeRow struct {
	rule string
	day  time.Time
}

// fakeDB keeps the marks by key, as the table with its primary key and ON CONFLICT DO NOTHING would.
type fakeDB struct {
	rows   map[fakeRow]bool
	execs  int
	failed error
}

func newFake() *fakeDB { return &fakeDB{rows: map[fakeRow]bool{}} }

func (f *fakeDB) Exec(_ context.Context, _ string, args ...any) error {
	f.execs++
	if f.failed != nil {
		return f.failed
	}
	f.rows[fakeRow{args[0].(string), args[1].(time.Time)}] = true
	return nil
}

func (f *fakeDB) Select(_ context.Context, _ string, args []any, scan func(scanner) error) error {
	if f.failed != nil {
		return f.failed
	}
	from := args[0].(time.Time)
	var got []fakeRow
	for r := range f.rows {
		if !r.day.Before(from) {
			got = append(got, r)
		}
	}
	sort.Slice(got, func(i, j int) bool {
		if !got[i].day.Equal(got[j].day) {
			return got[i].day.Before(got[j].day)
		}
		return got[i].rule < got[j].rule
	})
	for _, r := range got {
		if err := scan(rowScanner{r}); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) Close() error { return nil }

type rowScanner struct{ r fakeRow }

func (s rowScanner) Scan(dest ...any) error {
	*dest[0].(*string) = s.r.rule
	*dest[1].(*time.Time) = s.r.day
	return nil
}

func mark(rule string, day contracts.Day) store.SentMark {
	return store.SentMark{RuleID: rule, Day: day}
}

func TestSentSinceOrdersByDay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := &sentStore{db: newFake()}
	for _, m := range []store.SentMark{mark("b", "2026-11-03"), mark("a", "2026-11-03"), mark("z", "2026-11-01"), mark("c", "2026-10-30")} {
		if err := s.MarkSent(ctx, m.RuleID, m.Day); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.SentSince(ctx, "2026-11-01")
	if err != nil {
		t.Fatal(err)
	}
	want := []store.SentMark{mark("z", "2026-11-01"), mark("a", "2026-11-03"), mark("b", "2026-11-03")}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %v, want %v", i, got[i], want[i])
		}
	}
	none, err := s.SentSince(ctx, "2027-01-01")
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("none = %#v, %v; want an empty slice", none, err)
	}
	if _, err := s.SentSince(ctx, "not-a-day"); err == nil {
		t.Error("a bad day is accepted")
	}
}

func TestMarkSentRepeatKeepsOneMark(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := &sentStore{db: newFake()}
	for range 2 {
		if err := s.MarkSent(ctx, "vix_high", "2026-11-05"); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.SentSince(ctx, "2026-11-01")
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v, %v; want one mark", got, err)
	}
}

func TestMarkSentRejectsEmptyRule(t *testing.T) {
	t.Parallel()
	f := newFake()
	s := &sentStore{db: f}
	for _, c := range []struct {
		rule string
		day  contracts.Day
	}{{"", "2026-11-05"}, {"vix_high", "2026-13-45"}} {
		err := s.MarkSent(context.Background(), c.rule, c.day)
		var te contracts.TypedError
		if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
			t.Errorf("MarkSent(%q, %q) = %v; want a permanent error", c.rule, c.day, err)
		}
	}
	if f.execs != 0 {
		t.Errorf("%d statements ran for rejected arguments", f.execs)
	}
}

func TestStoreErrorsAreRetryable(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.failed = errors.New("connection reset")
	s := &sentStore{db: f}
	for name, err := range map[string]error{
		"mark":  s.MarkSent(context.Background(), "a", "2026-11-05"),
		"since": func() error { _, e := s.SentSince(context.Background(), "2026-11-05"); return e }(),
	} {
		var te contracts.TypedError
		if !errors.As(err, &te) || te.Kind() != contracts.Retryable {
			t.Errorf("%s: %v; want a retryable error", name, err)
		}
	}
}
