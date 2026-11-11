package candidate

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var (
	_ store.CandidateWriter = (*Store)(nil)
	_ store.CandidateReader = (*Store)(nil)
)

// fakeDB keeps the argument lists of the inserted rows and replays them to the scan of a query, filtered by the
// day and the kind the query is given and ordered as the query orders (rank first, nulls last, then the key).
type fakeDB struct {
	rows      [][]any
	inserts   int
	lastSQL   string
	insertErr error
}

// InsertAll applies the primary key rule of the table: a row whose key (columns 0 to 4) exists is skipped.
func (f *fakeDB) InsertAll(_ context.Context, sql string, rows [][]any) (int, error) {
	f.inserts++
	f.lastSQL = sql
	if f.insertErr != nil {
		return 0, f.insertErr
	}
	n := 0
	for _, r := range rows {
		dup := false
		for _, have := range f.rows {
			if reflect.DeepEqual(have[:5], r[:5]) {
				dup = true
				break
			}
		}
		if !dup {
			f.rows = append(f.rows, r)
			n++
		}
	}
	return n, nil
}

func (f *fakeDB) Select(_ context.Context, sql string, args []any, scan func(scanner) error) error {
	f.lastSQL = sql
	var got [][]any
	for _, r := range f.rows {
		if r[0].(time.Time).Equal(args[0].(time.Time)) && r[2].(string) == args[1].(string) {
			got = append(got, r)
		}
	}
	sort.SliceStable(got, func(i, j int) bool {
		ri, rj := got[i][31].(*int), got[j][31].(*int)
		switch {
		case ri == nil:
			return false
		case rj == nil:
			return true
		}
		return *ri < *rj
	})
	for _, r := range got {
		if err := scan(argScanner(r)); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) Close() error { return nil }

// argScanner hands the stored argument of column i to the destination i.
type argScanner []any

func (a argScanner) Scan(dest ...any) error {
	for i, d := range dest {
		reflect.ValueOf(d).Elem().Set(reflect.ValueOf(a[i]))
	}
	return nil
}

func ptr[T any](v T) *T { return &v }

func sample(runID string, at time.Time, rank *int) store.Candidate {
	return store.Candidate{
		SessionDay: "2026-11-06", RunID: runID, Kind: "short_put", InstrumentID: 42, Alias: "",
		RuleVersion: "v1", Issuer: "AAPL", UnderlyingID: 7, Expiry: "2026-12-18", DTE: 42, Right: "put",
		Strike: "180", Multiplier: 100, BidEod: "2.15", Ask: ptr("2.2"), Spot: ptr("190.5"), BuyPrice: ptr("178"),
		BreakEven: "177.85", AnnualizedYield: ptr("0.103855"), Delta: ptr("-0.21"), Iv: ptr("0.31"), Status: "ok",
		Gates: `[{"gate":"buy_price","result":"pass"}]`, Marks: `[]`, Fits: `{"A":"fit","B":"unknown"}`,
		NextEarnings: ptr(contracts.Day("2027-01-28")), EarningsState: "listed", Rank: rank, RankInUnderlying: rank,
		ComputedAt: at,
	}
}

func TestInsertQueryText(t *testing.T) {
	t.Parallel()
	db := &fakeDB{}
	s := &Store{db: db}
	n, err := s.InsertCandidates(context.Background(), []store.Candidate{sample("r1", time.Unix(100, 0).UTC(), ptr(1))})
	if err != nil || n != 1 {
		t.Fatalf("insert = %d, %v", n, err)
	}
	q := db.lastSQL
	if !strings.HasPrefix(q, "INSERT INTO candidates.option_candidate (session_day, run_id, kind, instrument_id, alias,") {
		t.Errorf("insert text = %q", q)
	}
	for _, bad := range []string{"UPDATE", "DELETE"} {
		if strings.Contains(q, bad) {
			t.Errorf("insert text holds %s: insert-only", bad)
		}
	}
	if got := strings.Count(q, "$"); got != len(db.rows[0]) || got != 35 {
		t.Errorf("placeholders = %d, args = %d, want 35 each", got, len(db.rows[0]))
	}

	none, err := s.InsertCandidates(context.Background(), nil)
	if err != nil || none != 0 || db.inserts != 1 {
		t.Errorf("no rows: %d, %v, inserts %d; want 0, nil, 1", none, err, db.inserts)
	}
	if _, err := s.InsertCandidates(context.Background(), []store.Candidate{{RunID: "r"}}); err == nil {
		t.Error("a row with no session day was accepted")
	}
	if _, err := s.InsertCandidates(context.Background(), []store.Candidate{{SessionDay: "2026-11-06", Kind: "short_put"}}); err == nil {
		t.Error("a row with no run id was accepted")
	}
	if _, err := s.Candidates(context.Background(), "2026-11-06", "short_put"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(db.lastSQL, "FROM rd.option_candidate_r1") || strings.Contains(db.lastSQL, "candidates.option_candidate") {
		t.Errorf("select text = %q; it reads the view only", db.lastSQL)
	}
}

func TestRowMapping(t *testing.T) {
	t.Parallel()
	s := &Store{db: &fakeDB{}}
	in := sample("r1", time.Unix(100, 0).UTC(), ptr(1))
	if _, err := s.InsertCandidates(context.Background(), []store.Candidate{in}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Candidates(context.Background(), "2026-11-06", "short_put")
	if err != nil || len(got) != 1 {
		t.Fatalf("read = %v, %v", got, err)
	}
	if !reflect.DeepEqual(got[0], in) {
		t.Errorf("row = %+v\nwant  %+v", got[0], in)
	}
}

func TestEmptyDayEmptySlice(t *testing.T) {
	t.Parallel()
	s := &Store{db: &fakeDB{}}
	got, err := s.Candidates(context.Background(), "2026-11-06", "covered_call")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("got %#v, %v; want an empty non-nil slice", got, err)
	}
}

func TestZeroDayIsError(t *testing.T) {
	t.Parallel()
	s := &Store{db: &fakeDB{}}
	if _, err := s.Candidates(context.Background(), "", "short_put"); err == nil {
		t.Error("a zero day was accepted")
	}
	if _, err := s.Candidates(context.Background(), "2026-11-06", "bogus"); err == nil {
		t.Error("an unknown kind was accepted")
	}
}

func TestTwoRunsNewerWins(t *testing.T) {
	t.Parallel()
	s := &Store{db: &fakeDB{}}
	t0 := time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC)
	older := []store.Candidate{sample("old", t0, ptr(1)), sample("old", t0, ptr(2))}
	older[1].InstrumentID = 43
	newer := []store.Candidate{sample("new", t0.Add(time.Hour), nil), sample("new", t0.Add(time.Hour), ptr(1))}
	newer[0].InstrumentID = 44
	newer[0].Status = "unknown"
	if _, err := s.InsertCandidates(context.Background(), append(older, newer...)); err != nil {
		t.Fatal(err)
	}
	got, err := s.Candidates(context.Background(), "2026-11-06", "short_put")
	if err != nil || len(got) != 2 {
		t.Fatalf("read = %d rows, %v", len(got), err)
	}
	for _, c := range got {
		if c.RunID != "new" {
			t.Errorf("row of run %q; only the newer run is returned", c.RunID)
		}
	}
	if got[0].Rank == nil || *got[0].Rank != 1 || got[1].Rank != nil {
		t.Errorf("ranked rows must come first: %v, %v", got[0].Rank, got[1].Rank)
	}
}

func TestSettingsVersionRowMapping(t *testing.T) {
	t.Parallel()
	s := &Store{db: &fakeDB{}}
	in := sample("r1", time.Unix(100, 0).UTC(), ptr(1))
	in.SettingsVersion = ptr(int64(57))
	if _, err := s.InsertCandidates(context.Background(), []store.Candidate{in}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Candidates(context.Background(), "2026-11-06", "short_put")
	if err != nil || len(got) != 1 {
		t.Fatalf("read = %v, %v", got, err)
	}
	if got[0].SettingsVersion == nil || *got[0].SettingsVersion != 57 {
		t.Errorf("settings version = %v, want 57", got[0].SettingsVersion)
	}
	if !strings.HasSuffix(columns, "computed_at, settings_version") {
		t.Errorf("columns = %q; settings_version is the last column", columns)
	}
}

func TestInsertCandidatesConstraintViolationIsPermanent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want contracts.ErrorKind
	}{
		{"plain", errors.New("boom"), contracts.Retryable},
		{"class 40", &pgconn.PgError{Code: "40001"}, contracts.Retryable},
		{"class 08", &pgconn.PgError{Code: "08006"}, contracts.Retryable},
	}
	for _, code := range []string{"23502", "23503", "23505", "23514"} {
		cases = append(cases,
			struct {
				name string
				err  error
				want contracts.ErrorKind
			}{code, &pgconn.PgError{Code: code}, contracts.Permanent},
			struct {
				name string
				err  error
				want contracts.ErrorKind
			}{code + " wrapped", fmt.Errorf("x: %w", &pgconn.PgError{Code: code}), contracts.Permanent})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := &Store{db: &fakeDB{insertErr: tc.err}}
			_, err := s.InsertCandidates(context.Background(), []store.Candidate{sample("r1", time.Unix(100, 0).UTC(), ptr(1))})
			if err == nil {
				t.Fatal("no error")
			}
			var te contracts.TypedError
			if !errors.As(err, &te) {
				t.Fatalf("not a TypedError: %v", err)
			}
			if te.Kind() != tc.want {
				t.Errorf("kind = %v, want %v", te.Kind(), tc.want)
			}
		})
	}
}

func TestNilSettingsVersionIsNull(t *testing.T) {
	t.Parallel()
	db := &fakeDB{}
	s := &Store{db: db}
	in := sample("r1", time.Unix(100, 0).UTC(), ptr(1))
	if _, err := s.InsertCandidates(context.Background(), []store.Candidate{in}); err != nil {
		t.Fatal(err)
	}
	arg, ok := db.rows[0][34].(*int64)
	if !ok || arg != nil {
		t.Errorf("argument 35 = %#v, want a nil *int64 (null)", db.rows[0][34])
	}
	got, err := s.Candidates(context.Background(), "2026-11-06", "short_put")
	if err != nil || len(got) != 1 || got[0].SettingsVersion != nil {
		t.Errorf("read = %+v, %v; want a nil settings version", got, err)
	}
}

func TestInsertCandidatesSecondWriteStoresNothing(t *testing.T) {
	t.Parallel()
	db := &fakeDB{}
	s := &Store{db: db}
	at := time.Unix(100, 0).UTC()
	a := sample("r1", at, ptr(1))
	b := sample("r1", at, ptr(2))
	b.InstrumentID = 43
	rows := []store.Candidate{a, b}
	n1, err1 := s.InsertCandidates(context.Background(), rows)
	n2, err2 := s.InsertCandidates(context.Background(), rows)
	if err1 != nil || err2 != nil || n1 != 2 || n2 != 0 || len(db.rows) != 2 {
		t.Fatalf("counts = (%d, %v), (%d, %v), stored %d; want (2, nil), (0, nil), 2", n1, err1, n2, err2, len(db.rows))
	}
	c := sample("r1", at, ptr(3))
	c.InstrumentID = 44
	n3, err3 := s.InsertCandidates(context.Background(), []store.Candidate{a, c})
	if err3 != nil || n3 != 1 || len(db.rows) != 3 {
		t.Errorf("mixed call = (%d, %v), stored %d; want (1, nil), 3", n3, err3, len(db.rows))
	}
	other := sample("r2", at.Add(time.Hour), ptr(1))
	if n, err := s.InsertCandidates(context.Background(), []store.Candidate{other}); err != nil || n != 1 || len(db.rows) != 4 {
		t.Errorf("another run = (%d, %v), stored %d; want (1, nil), 4", n, err, len(db.rows))
	}
}

func TestInsertCandidatesSqlSkipsExistingKeys(t *testing.T) {
	t.Parallel()
	db := &fakeDB{}
	s := &Store{db: db}
	if _, err := s.InsertCandidates(context.Background(), []store.Candidate{sample("r1", time.Unix(100, 0).UTC(), ptr(1))}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(db.lastSQL, "ON CONFLICT DO NOTHING") {
		t.Errorf("insert text = %q; want ON CONFLICT DO NOTHING", db.lastSQL)
	}
}
