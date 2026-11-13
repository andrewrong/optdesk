package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// heldFakeStore is a store of its own for these tests: it reads rows, checks and adds like underlying.HeldAdder.
// The embedded Store is nil: only the methods below are called.
type heldFakeStore struct {
	underlying.Store
	rows    []underlying.Row
	adds    []string
	addErr  error
	readErr error
}

func (s *heldFakeStore) Current(context.Context) ([]underlying.Row, error) {
	return s.rows, s.readErr
}

type heldInvalid struct{}

func (heldInvalid) Error() string { return "invalid" }
func (heldInvalid) Field() string { return "currency" }
func (heldInvalid) Why() string   { return "no market" }

func (s *heldFakeStore) Check(symbol, currency string) (string, error) {
	switch currency {
	case "USD":
		return "US:" + strings.ToUpper(symbol), nil
	case "JPY":
		return "JP:" + strings.ToUpper(symbol), nil
	}
	return "", heldInvalid{}
}

func (s *heldFakeStore) Add(_ context.Context, symbol, currency string) (bool, error) {
	if s.addErr != nil {
		return false, s.addErr
	}
	key, err := s.Check(symbol, currency)
	if err != nil {
		return false, err
	}
	s.adds = append(s.adds, key)
	s.rows = append(s.rows, underlying.Row{Key: key, Symbol: symbol})
	return true, nil
}

// heldFakeAccount answers the positions of every alias on one day, and counts the reads.
type heldFakeAccount struct {
	store.AccountReader
	day   string
	rows  []store.Position
	err   error
	reads int
}

func (a *heldFakeAccount) Positions(_ context.Context, alias, day string) ([]store.Position, error) {
	a.reads++
	if a.err != nil {
		return nil, a.err
	}
	if alias == "A" && day == a.day {
		return a.rows, nil
	}
	return nil, nil
}

// heldFakeInner stands for the resolve job: it plans one pending key and resolves the keys that have a row.
type heldFakeInner struct {
	st      *heldFakeStore
	ran     [][]contracts.ExpectedKey
	planErr error
}

func (j *heldFakeInner) ID() string { return "underlying_resolve" }
func (j *heldFakeInner) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return []contracts.ExpectedKey{{DatasetID: "ds", SubjectKey: "US:OLD"}}, "inner-hash", j.planErr
}
func (j *heldFakeInner) Run(_ context.Context, _ contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	j.ran = append(j.ran, keys)
	var res contracts.JobResult
	for _, k := range keys {
		for _, r := range j.st.rows {
			if r.Key == k.SubjectKey {
				res.Returned++
				res.Usable++
			}
		}
	}
	return res
}
func (*heldFakeInner) Schedule() (string, string, string, time.Duration) {
	return "XNYS", "session_close", "America/New_York", 45 * time.Minute
}
func (*heldFakeInner) RunBound() time.Duration { return 10 * time.Minute }
func (*heldFakeInner) RestartSensitive() bool  { return true }

type heldRig struct {
	st    *heldFakeStore
	acct  *heldFakeAccount
	inner *heldFakeInner
	job   contracts.Job
	log   *bytes.Buffer
}

const heldToday = "2026-11-10"

func newHeldRig(t *testing.T, rows ...store.Position) *heldRig {
	t.Helper()
	r := &heldRig{st: &heldFakeStore{}, acct: &heldFakeAccount{day: heldToday, rows: rows}, log: &bytes.Buffer{}}
	r.inner = &heldFakeInner{st: r.st}
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := fakeClock{time.Date(2026, 11, 10, 22, 0, 0, 0, time.UTC)}
	r.job, err = heldRows(r.inner, r.st, r.st, r.acct, []string{"A", "B"}, now, slog.New(slog.NewTextHandler(r.log, nil)), "ds", 20, zone)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func heldPos(symbol, secType, currency, qty string) store.Position {
	return store.Position{Symbol: symbol, SecType: secType, Currency: currency, Quantity: qty}
}

func heldSubjects(keys []contracts.ExpectedKey) []string {
	var out []string
	for _, k := range keys {
		out = append(out, k.SubjectKey)
	}
	return out
}

func TestHeldPlanAddsKeyWithoutWriting(t *testing.T) {
	t.Parallel()
	r := newHeldRig(t, heldPos("MSFT", "option", "USD", "-1"), heldPos("AAPL", "stock", "USD", "100"),
		heldPos("AAPL", "option", "USD", "1"), heldPos("ZERO", "stock", "USD", "0"), heldPos("TSLA", "stock", "USD", "5"))
	r.st.rows = []underlying.Row{{Key: "US:TSLA", Symbol: "TSLA", Status: "retired"}}
	keys, hash, err := r.job.Plan(context.Background(), heldToday)
	if err != nil {
		t.Fatal(err)
	}
	if got := heldSubjects(keys); !slices.Equal(got, []string{"US:AAPL", "US:MSFT", "US:OLD"}) {
		t.Fatalf("keys %v", got)
	}
	for _, k := range keys {
		if k.DatasetID != "ds" {
			t.Fatalf("dataset %q", k.DatasetID)
		}
	}
	if hash == "inner-hash" || hash == "" {
		t.Fatalf("hash %q should join the held keys", hash)
	}
	if len(r.st.adds) != 0 {
		t.Fatalf("Plan wrote %v", r.st.adds)
	}
	// The same plan gives the same hash; no held name gives the inner hash.
	_, again, _ := r.job.Plan(context.Background(), heldToday)
	if again != hash {
		t.Fatalf("hash %q then %q", hash, again)
	}
	r.acct.rows = nil
	if _, h, _ := r.job.Plan(context.Background(), heldToday); h != "inner-hash" {
		t.Fatalf("hash without held names %q", h)
	}
}

func TestHeldPlanCapsAtMaxNames(t *testing.T) {
	t.Parallel()
	var ps []store.Position
	for i := 0; i < 25; i++ {
		ps = append(ps, heldPos(string(rune('A'+i)), "stock", "USD", "1"))
	}
	r := newHeldRig(t, ps...)
	keys, _, err := r.job.Plan(context.Background(), heldToday)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 21 { // 20 held, 1 inner
		t.Fatalf("%d keys", len(keys))
	}
}

func TestHeldRunAddsThenRunsInner(t *testing.T) {
	t.Parallel()
	r := newHeldRig(t, heldPos("AAPL", "stock", "USD", "100"))
	keys := []contracts.ExpectedKey{{DatasetID: "ds", SubjectKey: "US:AAPL"}}
	res := r.job.Run(context.Background(), heldToday, keys)
	if res.Err != nil || res.Returned != 1 || res.Usable != 1 {
		t.Fatalf("result %+v", res)
	}
	if !slices.Equal(r.st.adds, []string{"US:AAPL"}) || len(r.inner.ran) != 1 {
		t.Fatalf("adds %v, inner runs %d", r.st.adds, len(r.inner.ran))
	}
}

func TestHeldRunAddsOneNamePerRun(t *testing.T) {
	t.Parallel()
	r := newHeldRig(t, heldPos("AAPL", "stock", "USD", "1"), heldPos("MSFT", "stock", "USD", "1"))
	keys := []contracts.ExpectedKey{{SubjectKey: "US:AAPL"}, {SubjectKey: "US:MSFT"}}
	r.job.Run(context.Background(), heldToday, keys)
	if len(r.st.adds) != 1 {
		t.Fatalf("adds %v", r.st.adds)
	}
}

func TestHeldRunSoldNameDoesNothing(t *testing.T) {
	t.Parallel()
	r := newHeldRig(t) // sold since the plan: no position
	res := r.job.Run(context.Background(), heldToday, []contracts.ExpectedKey{{SubjectKey: "US:AAPL"}})
	if res.Err != nil || res.Returned != 0 || res.Usable != 0 || len(r.st.adds) != 0 {
		t.Fatalf("result %+v adds %v", res, r.st.adds)
	}
}

func TestHeldRunStoreErrorIsRetryable(t *testing.T) {
	t.Parallel()
	r := newHeldRig(t, heldPos("AAPL", "stock", "USD", "1"))
	r.st.addErr = errors.New("down")
	res := r.job.Run(context.Background(), heldToday, []contracts.ExpectedKey{{SubjectKey: "US:AAPL"}})
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("result %+v", res)
	}
}

func TestHeldPlanSkipsIndexAndRefused(t *testing.T) {
	t.Parallel()
	r := newHeldRig(t, heldPos("SPX", "option", "USD", "1"), heldPos("N225", "option", "JPY", "1"),
		heldPos("GBPCO", "stock", "GBP", "1"), heldPos("OK", "stock", "USD", "1"))
	keys, _, err := r.job.Plan(context.Background(), heldToday)
	if err != nil {
		t.Fatal(err)
	}
	if got := heldSubjects(keys); !slices.Equal(got, []string{"US:OK", "US:OLD"}) {
		t.Fatalf("keys %v", got)
	}
	if heldIndexText("JPY") != "index option, covered by option_eod_jp" || heldIndexText("USD") != "not supported: index option" {
		t.Fatal("index texts")
	}
}

func TestHeldRowForwardsOptionals(t *testing.T) {
	t.Parallel()
	r := newHeldRig(t)
	if r.job.ID() != "underlying_resolve" {
		t.Fatalf("id %q", r.job.ID())
	}
	s, ok := r.job.(interface {
		Schedule() (string, string, string, time.Duration)
	})
	if !ok {
		t.Fatal("no Schedule")
	}
	cal, anchor, zone, off := s.Schedule()
	if cal != "XNYS" || anchor != "session_close" || zone != "America/New_York" || off != 45*time.Minute {
		t.Fatalf("schedule %s %s %s %v", cal, anchor, zone, off)
	}
	if b, ok := r.job.(interface{ RunBound() time.Duration }); !ok || b.RunBound() != 10*time.Minute {
		t.Fatal("RunBound")
	}
	if b, ok := r.job.(interface{ RestartSensitive() bool }); !ok || !b.RestartSensitive() {
		t.Fatal("RestartSensitive")
	}
}

func TestHeldPlanPastDayPlansInnerOnly(t *testing.T) {
	t.Parallel()
	r := newHeldRig(t, heldPos("AAPL", "stock", "USD", "1"))
	keys, hash, err := r.job.Plan(context.Background(), "2026-11-09")
	if err != nil || hash != "inner-hash" || !slices.Equal(heldSubjects(keys), []string{"US:OLD"}) {
		t.Fatalf("keys %v hash %q err %v", heldSubjects(keys), hash, err)
	}
	if r.acct.reads != 0 {
		t.Fatalf("a past day read the positions %d times", r.acct.reads)
	}
}

func TestHeldPlanReadsStoredWords(t *testing.T) {
	t.Parallel()
	r := newHeldRig(t, heldPos("BROKERSTK", "STK", "USD", "1"), heldPos("BROKEROPT", "OPT", "USD", "1"),
		heldPos("SHARE", "stock", "USD", "1"), heldPos("CALL", "option", "USD", "1"), heldPos("IDX", "index", "USD", "1"))
	keys, _, err := r.job.Plan(context.Background(), heldToday)
	if err != nil {
		t.Fatal(err)
	}
	if got := heldSubjects(keys); !slices.Equal(got, []string{"US:CALL", "US:SHARE", "US:OLD"}) {
		t.Fatalf("keys %v", got)
	}
}

func TestHeldPlanReadFailureKeepsInnerKeys(t *testing.T) {
	t.Parallel()
	r := newHeldRig(t, heldPos("AAPL", "stock", "USD", "1"))
	r.acct.err = errors.New("secret-text")
	keys, hash, err := r.job.Plan(context.Background(), heldToday)
	if err != nil || hash != "inner-hash" || !slices.Equal(heldSubjects(keys), []string{"US:OLD"}) {
		t.Fatalf("keys %v hash %q err %v", heldSubjects(keys), hash, err)
	}
	out := r.log.String()
	if strings.Count(out, "level=WARN") != 1 || !strings.Contains(out, "underlying_resolve") || strings.Contains(out, "secret-text") {
		t.Fatalf("log %q", out)
	}
}
