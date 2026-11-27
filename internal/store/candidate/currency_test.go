package candidate

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/candidatecur"
)

// curDB is a fake db whose stored rows have the currency as column 35 (nil is null). It reads the currency clause of
// the query text: a query without one sees every row.
type curDB struct{ rows [][]any }

func (f *curDB) InsertAll(_ context.Context, sql string, rows [][]any) (int, error) {
	for _, r := range rows {
		switch {
		case len(r) == columnCount:
			r = append(append([]any{}, r...), (*string)(nil))
		case !strings.Contains(sql, ", currency)"):
			panic("a 36 column row needs the currency insert")
		default:
			r = append(append([]any{}, r[:columnCount]...), new(r[columnCount].(string)))
		}
		f.rows = append(f.rows, r)
	}
	return len(rows), nil
}

func (f *curDB) Select(_ context.Context, sql string, args []any, scan func(scanner) error) error {
	for _, r := range f.rows {
		if !r[0].(time.Time).Equal(args[0].(time.Time)) || r[2].(string) != args[1].(string) {
			continue
		}
		cur := r[columnCount].(*string)
		want := "USD" // Candidates names USD in the text and passes no argument
		if len(args) > 2 {
			want = args[2].(string)
		}
		switch {
		case strings.Contains(sql, "currency IS NULL"):
			if cur != nil && *cur != want {
				continue
			}
		case strings.Contains(sql, "currency = $3"):
			if cur == nil || *cur != want {
				continue
			}
		}
		if err := scan(argScanner(r[:columnCount])); err != nil {
			return err
		}
	}
	return nil
}

func (f *curDB) Close() error { return nil }

func curRow(runID string, at time.Time, cur string) candidatecur.Row {
	return candidatecur.Row{Candidate: sample(runID, at, new(1)), Currency: cur}
}

func TestCandidatesUSDOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := &Store{db: &curDB{}}
	us := time.Date(2026, 11, 6, 22, 58, 0, 0, time.UTC)
	jp := time.Date(2026, 11, 7, 8, 10, 0, 0, time.UTC) // newer than the US run
	if _, err := s.InsertCandidates(ctx, []store.Candidate{sample("us", us, new(1))}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertCandidatesIn(ctx, []candidatecur.Row{curRow("jp", jp, "JPY")}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Candidates(ctx, "2026-11-06", "short_put")
	if err != nil || len(got) != 1 || got[0].RunID != "us" {
		t.Fatalf("Candidates = %+v, %v; want the US run", got, err)
	}
}

func TestCandidatesInNewestRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := &Store{db: &curDB{}}
	d := func(h, m int) time.Time { return time.Date(2026, 11, 6, h, m, 0, 0, time.UTC) }
	if _, err := s.InsertCandidatesIn(ctx, []candidatecur.Row{
		curRow("us", d(22, 58), "USD"), curRow("jp-old", d(1, 0), "JPY"), curRow("jp", d(8, 10), "JPY"),
	}); err != nil {
		t.Fatal(err)
	}
	for cur, want := range map[string]string{"USD": "us", "JPY": "jp"} {
		got, err := s.CandidatesIn(ctx, "2026-11-06", "short_put", cur)
		if err != nil || len(got) != 1 || got[0].RunID != want || got[0].Currency != cur {
			t.Errorf("CandidatesIn %s = %+v, %v; want run %s", cur, got, err, want)
		}
	}
	none, err := s.CandidatesIn(ctx, "2026-11-06", "short_put", "HKD")
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("HKD = %#v, %v; want empty", none, err)
	}
	for _, bad := range []string{"", "jpy", "YEN1"} {
		if _, err := s.CandidatesIn(ctx, "2026-11-06", "short_put", bad); err == nil {
			t.Errorf("currency %q accepted", bad)
		}
	}
}

func TestInsertCandidatesInRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := &curDB{}
	s := &Store{db: db}
	at := time.Date(2026, 11, 6, 8, 10, 0, 0, time.UTC)
	if n, err := s.InsertCandidatesIn(ctx, []candidatecur.Row{curRow("jp", at, "JPY")}); err != nil || n != 1 {
		t.Fatalf("insert = %d, %v", n, err)
	}
	if cur := db.rows[0][columnCount].(*string); cur == nil || *cur != "JPY" {
		t.Errorf("stored currency = %v, want JPY", cur)
	}
	got, err := s.CandidatesIn(ctx, "2026-11-06", "short_put", "JPY")
	if err != nil || len(got) != 1 || got[0].Currency != "JPY" {
		t.Fatalf("read back = %+v, %v", got, err)
	}
	if _, err := s.InsertCandidates(ctx, []store.Candidate{sample("us", at, new(1))}); err != nil {
		t.Fatal(err)
	}
	if cur := db.rows[1][columnCount].(*string); cur != nil {
		t.Errorf("InsertCandidates wrote currency %q, want null", *cur)
	}
	if _, err := s.InsertCandidatesIn(ctx, []candidatecur.Row{curRow("x", at, "")}); err == nil {
		t.Error("an empty currency was accepted")
	}
}
