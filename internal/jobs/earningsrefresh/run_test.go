package earningsrefresh

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/symbol"
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 11, 9, 21, 30, 0, 0, time.UTC) }

// memStore is a writer and a reader over one slice; a key that exists is left as it is.
type memStore struct {
	mu       sync.Mutex
	rows     []contracts.EarningsDate
	readErr  error
	writeErr error
}

func (m *memStore) InsertEarningsDate(_ context.Context, r contracts.EarningsDate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	for _, x := range m.rows {
		if x.Issuer == r.Issuer && x.Attempt == r.Attempt && x.ObservedOn.Equal(r.ObservedOn.Time) {
			return nil
		}
	}
	m.rows = append(m.rows, r)
	return nil
}

func (m *memStore) EarningsDates(_ context.Context, issuer string, from, to contracts.Day) ([]contracts.EarningsDate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readErr != nil {
		return nil, m.readErr
	}
	var out []contracts.EarningsDate
	for _, x := range m.rows {
		d := x.ObservedOn.Format(dayLayout)
		if x.Issuer == issuer && d >= string(from) && d <= string(to) {
			out = append(out, x)
		}
	}
	return out, nil
}

type fakeSource struct {
	calls    int
	listings []sources.EarningsListing
	err      error
}

func (f *fakeSource) Fetch(context.Context) ([]sources.EarningsListing, error) {
	f.calls++
	return f.listings, f.err
}

type codedErr struct{ code string }

func (e codedErr) Error() string             { return "boom" }
func (e codedErr) Kind() contracts.ErrorKind { return contracts.Retryable }
func (e codedErr) ErrorCode() string         { return e.code }

const day = contracts.Day("2026-11-09")

func us(issuer, sym string) jobs.WatchEntry {
	return jobs.WatchEntry{Issuer: issuer, Market: "US", Contract: broker.Contract{Symbol: sym}}
}

func jp(issuer, sym string) jobs.WatchEntry {
	return jobs.WatchEntry{Issuer: issuer, Market: "JP", Contract: broker.Contract{Symbol: sym}}
}

func build(t *testing.T, srcs map[string]sources.EarningsSource, st *memStore, entries ...jobs.WatchEntry) contracts.Job {
	t.Helper()
	j, err := New(srcs, st, st, entries, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func byIssuer(st *memStore, issuer string) []contracts.EarningsDate {
	var out []contracts.EarningsDate
	for _, r := range st.rows {
		if r.Issuer == issuer {
			out = append(out, r)
		}
	}
	return out
}

func TestListedTakesEarliestOnOrAfterDay(t *testing.T) {
	t.Parallel()
	src := &fakeSource{listings: []sources.EarningsListing{
		{Symbol: "AAPL", ReportDate: "2026-11-01"},
		{Symbol: "AAPL", ReportDate: "2026-12-20", TimeOfDay: "post-market"},
		{Symbol: "AAPL", ReportDate: "2026-11-09", FiscalPeriodEnd: "2026-09-30", Estimate: "1.5", TimeOfDay: "pre-market"},
		{Symbol: "MSFT", ReportDate: "2026-11-10"},
	}}
	st := &memStore{}
	j := build(t, map[string]sources.EarningsSource{"US": src}, st, us("cik:1", "AAPL"), us("cik:2", "NVDA"))
	if res := j.Run(context.Background(), day, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	a := byIssuer(st, "cik:1")
	if len(a) != 1 || a[0].Status != "listed" || a[0].ReportDate.Format(dayLayout) != "2026-11-09" ||
		a[0].Source != "alphavantage" || a[0].TimeOfDay != "pre-market" || a[0].Attempt != 1 ||
		a[0].Estimate == nil || *a[0].Estimate != "1.5" || a[0].FiscalPeriodEnd == nil {
		t.Fatalf("row %+v", a)
	}
	n := byIssuer(st, "cik:2")
	if len(n) != 1 || n[0].Status != "absent" || n[0].ReportDate != nil || n[0].Source != "alphavantage" {
		t.Fatalf("row %+v", n)
	}
}

func TestMarketWithoutSourceIsUnsupported(t *testing.T) {
	t.Parallel()
	us1 := &fakeSource{}
	st := &memStore{}
	j := build(t, map[string]sources.EarningsSource{"US": us1}, st, jp("edinet:E1", "7203"))
	if res := j.Run(context.Background(), day, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	r := byIssuer(st, "edinet:E1")
	if len(r) != 1 || r[0].Status != "unsupported" || r[0].Source != "none" || r[0].Market != "JP" || r[0].ReportDate != nil {
		t.Fatalf("row %+v", r)
	}
	if us1.calls != 0 {
		t.Fatalf("US source called %d times", us1.calls)
	}
}

func TestOwnerOverrideWins(t *testing.T) {
	t.Parallel()
	src := &fakeSource{listings: []sources.EarningsListing{{Symbol: "AAPL", ReportDate: "2026-11-12"}}}
	st := &memStore{}
	e := us("cik:1", "AAPL")
	e.ReportDateOverride = "2026-12-01"
	j := build(t, map[string]sources.EarningsSource{"US": src}, st, e)
	if res := j.Run(context.Background(), day, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	r := byIssuer(st, "cik:1")
	if len(r) != 1 || r[0].Source != "owner" || r[0].Status != "listed" || r[0].Attempt != 1 ||
		r[0].ReportDate.Format(dayLayout) != "2026-12-01" {
		t.Fatalf("rows %+v", r)
	}
}

func TestSourceErrorWritesErrorRows(t *testing.T) {
	t.Parallel()
	usSrc := &fakeSource{err: codedErr{"http_status"}}
	jpSrc := &fakeSource{listings: []sources.EarningsListing{{Symbol: "7203", ReportDate: "2026-11-20"}}}
	st := &memStore{}
	j := build(t, map[string]sources.EarningsSource{"US": usSrc, "JP": jpSrc}, st,
		us("cik:1", "AAPL"), us("cik:2", "MSFT"), jp("edinet:E1", "7203"))
	res := j.Run(context.Background(), day, nil)
	if res.Err == nil || res.Err.Kind() != contracts.Permanent {
		t.Fatalf("err %v", res.Err)
	}
	for _, id := range []string{"cik:1", "cik:2"} {
		r := byIssuer(st, id)
		if len(r) != 1 || r[0].Status != "error" || r[0].ErrorCode != "http_status" || r[0].Attempt != 1 || r[0].ReportDate != nil {
			t.Fatalf("%s: %+v", id, r)
		}
	}
	r := byIssuer(st, "edinet:E1")
	if len(r) != 1 || r[0].Status != "listed" || r[0].Source != "jquants" {
		t.Fatalf("JP rows must be written though US failed: %+v", r)
	}
}

func TestOneFetchPerSourcePerRun(t *testing.T) {
	t.Parallel()
	usSrc, jpSrc := &fakeSource{}, &fakeSource{}
	st := &memStore{}
	j := build(t, map[string]sources.EarningsSource{"US": usSrc, "JP": jpSrc}, st,
		us("cik:1", "A"), us("cik:2", "B"), us("cik:3", "C"), jp("edinet:E1", "1111"), jp("edinet:E2", "2222"))
	if res := j.Run(context.Background(), day, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	if usSrc.calls != 1 || jpSrc.calls != 1 || len(st.rows) != 5 {
		t.Fatalf("calls %d %d, rows %d", usSrc.calls, jpSrc.calls, len(st.rows))
	}
}

func TestSecondRunSameDayMakesNoCall(t *testing.T) {
	t.Parallel()
	usSrc, jpSrc := &fakeSource{}, &fakeSource{}
	st := &memStore{}
	j := build(t, map[string]sources.EarningsSource{"US": usSrc, "JP": jpSrc}, st, us("cik:1", "A"), jp("edinet:E1", "1111"))
	j.Run(context.Background(), day, nil)
	j.Run(context.Background(), day, nil)
	if usSrc.calls != 1 || jpSrc.calls != 1 {
		t.Fatalf("calls %d %d", usSrc.calls, jpSrc.calls)
	}
}

func TestErrorRowIsFetchedAgain(t *testing.T) {
	t.Parallel()
	src := &fakeSource{err: codedErr{"network"}}
	st := &memStore{}
	j := build(t, map[string]sources.EarningsSource{"US": src}, st, us("cik:1", "A"))
	if res := j.Run(context.Background(), day, nil); res.Err == nil {
		t.Fatal("first run must fail")
	}
	if r := byIssuer(st, "cik:1"); len(r) != 1 || r[0].Status != "error" || r[0].Attempt != 1 {
		t.Fatalf("first rows %+v", r)
	}
	src.err, src.listings, src.calls = nil, []sources.EarningsListing{{Symbol: "A", ReportDate: "2026-11-12"}}, 0
	if res := j.Run(context.Background(), day, nil); res.Err != nil {
		t.Fatalf("second run: %v", res.Err)
	}
	if src.calls != 1 {
		t.Fatalf("calls %d", src.calls)
	}
	r := byIssuer(st, "cik:1")
	if len(r) != 2 || r[1].Status != "listed" || r[1].Attempt != 2 {
		t.Fatalf("rows %+v", r)
	}
}

func TestRateLimitedRecordedAsError(t *testing.T) {
	t.Parallel()
	src := &fakeSource{err: codedErr{"rate_limited"}}
	st := &memStore{}
	j := build(t, map[string]sources.EarningsSource{"US": src}, st, us("cik:1", "A"))
	res := j.Run(context.Background(), day, nil)
	if res.Err == nil || src.calls != 1 || len(st.rows) != 1 || st.rows[0].ErrorCode != "rate_limited" || st.rows[0].Status != "error" {
		t.Fatalf("calls %d rows %+v err %v", src.calls, st.rows, res.Err)
	}
}

func TestSecondRunWritesNoRow(t *testing.T) {
	t.Parallel()
	src := &fakeSource{listings: []sources.EarningsListing{{Symbol: "A", ReportDate: "2026-11-12"}}}
	st := &memStore{}
	own := us("cik:2", "B")
	own.ReportDateOverride = "2026-12-01"
	j := build(t, map[string]sources.EarningsSource{"US": src}, st, us("cik:1", "A"), own, jp("edinet:E1", "1111"))
	if res := j.Run(context.Background(), day, nil); res.Err != nil || len(st.rows) != 3 {
		t.Fatalf("first: %v rows %d", res.Err, len(st.rows))
	}
	if res := j.Run(context.Background(), day, nil); res.Err != nil || len(st.rows) != 3 || res.Returned != 0 {
		t.Fatalf("second: %v rows %d", res.Err, len(st.rows))
	}
}

func TestSecondRunStillFailingStoresNextAttempt(t *testing.T) {
	t.Parallel()
	src := &fakeSource{err: errors.New("down")}
	st := &memStore{}
	j := build(t, map[string]sources.EarningsSource{"US": src}, st, us("cik:1", "A"))
	first := j.Run(context.Background(), day, nil)
	second := j.Run(context.Background(), day, nil)
	if first.Err == nil || second.Err == nil || second.Err.Kind() != contracts.Permanent {
		t.Fatalf("first %v second %v", first.Err, second.Err)
	}
	r := byIssuer(st, "cik:1")
	if src.calls != 2 || len(r) != 2 || r[0].Attempt != 1 || r[1].Attempt != 2 || r[1].Status != "error" {
		t.Fatalf("calls %d rows %+v", src.calls, r)
	}
}

func TestPostconditionRowPerKey(t *testing.T) {
	t.Parallel()
	src := &fakeSource{listings: []sources.EarningsListing{{Symbol: "AAPL", ReportDate: "2026-11-09"}}}
	st := &memStore{}
	j := build(t, map[string]sources.EarningsSource{"US": src}, st, us("cik:1", "AAPL"), us("cik:2", "NVDA"))
	if res := j.Run(context.Background(), day, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	pc, ok := j.(interface {
		Postcondition(ctx context.Context, day, datasetID, subjectKey string) (bool, error)
	})
	if !ok {
		t.Fatal("job has no Postcondition")
	}
	ctx := context.Background()
	for _, c := range []struct {
		day, key string
		want     bool
	}{
		{"2026-11-09", "cik:1", true},
		{"2026-11-09", "cik:2", true},
		{"2026-11-09", "cik:3", false},
		{"2026-11-10", "cik:1", false},
	} {
		got, err := pc.Postcondition(ctx, c.day, datasetID, c.key)
		if err != nil || got != c.want {
			t.Fatalf("%s %s: got %v, %v; want %v", c.day, c.key, got, err, c.want)
		}
	}
	if got, err := pc.Postcondition(ctx, "2026-11-09", "obs.other", "cik:1"); err == nil || got {
		t.Fatalf("other dataset: got %v, %v", got, err)
	}
}

func TestSameSymbolShareClass(t *testing.T) {
	t.Parallel()
	for _, p := range [][2]string{{"BRK B", "BRK.B"}, {"BRK-B", "BRK.B"}, {"BRK B", "BRK-B"}, {"brk b", "BRK.B"}} {
		if !sameSymbol(p[0], p[1]) {
			t.Fatalf("%q and %q should match", p[0], p[1])
		}
	}
	if sameSymbol("BRK B", "BRKA") {
		t.Fatal("BRK B must differ from BRKA")
	}
}

func TestListedMatchesSpacedWatchSymbol(t *testing.T) {
	t.Parallel()
	src := &fakeSource{listings: []sources.EarningsListing{{Symbol: "BRK.B", ReportDate: "2026-11-09"}}}
	st := &memStore{}
	j := build(t, map[string]sources.EarningsSource{"US": src}, st, us("cik:9", "BRK B"))
	if res := j.Run(context.Background(), day, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	r := byIssuer(st, "cik:9")
	if len(r) != 1 || r[0].Status != "listed" {
		t.Fatalf("row %+v", r)
	}
}

// fakeTranslator spells every share-class form of a symbol as dash-separated and refuses the symbols in bad.
type fakeTranslator struct{ bad map[string]bool }

func (f fakeTranslator) ToVendor(s string, v symbol.Vendor) (string, error) {
	if f.bad[s] {
		return "", errors.New("refused " + s)
	}
	if v != symbol.AlphaVantage {
		return "", errors.New("unknown vendor")
	}
	return strings.ToUpper(strings.NewReplacer(" ", "-", ".", "-").Replace(s)), nil
}

func (fakeTranslator) OSIRoot(s string) string { return s }

func buildWith(t *testing.T, tr symbol.Translator, src *fakeSource, st *memStore, entries ...jobs.WatchEntry) {
	t.Helper()
	j, err := NewWithSymbols(map[string]sources.EarningsSource{"US": src}, st, st, entries, fixedClock{}, tr)
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), day, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
}

func listedSymbolCase(t *testing.T, listing string) {
	t.Helper()
	src := &fakeSource{listings: []sources.EarningsListing{{Symbol: listing, ReportDate: "2026-11-09"}}}
	st := &memStore{}
	buildWith(t, fakeTranslator{}, src, st, us("cik:9", "BRK B"))
	r := byIssuer(st, "cik:9")
	if len(r) != 1 || r[0].Status != "listed" {
		t.Fatalf("row %+v", r)
	}
}

func TestListedShareClassDash(t *testing.T) {
	t.Parallel()
	listedSymbolCase(t, "BRK-B")
}

func TestListedShareClassDot(t *testing.T) {
	t.Parallel()
	listedSymbolCase(t, "BRK.B")
}

func TestListedNilTranslator(t *testing.T) {
	t.Parallel()
	src := &fakeSource{listings: []sources.EarningsListing{{Symbol: "BRK.B", ReportDate: "2026-11-09"}}}
	st := &memStore{}
	j := build(t, map[string]sources.EarningsSource{"US": src}, st, us("cik:9", "BRK B"))
	if res := j.Run(context.Background(), day, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	r := byIssuer(st, "cik:9")
	if len(r) != 1 || r[0].Status != "listed" {
		t.Fatalf("row %+v", r)
	}
}

func TestListedEntryError(t *testing.T) {
	t.Parallel()
	src := &fakeSource{listings: []sources.EarningsListing{{Symbol: "BRK-B", ReportDate: "2026-11-09"}}}
	st := &memStore{}
	buildWith(t, fakeTranslator{bad: map[string]bool{"BRK B": true}}, src, st, us("cik:9", "BRK B"))
	r := byIssuer(st, "cik:9")
	if len(r) != 1 || r[0].Status != "absent" {
		t.Fatalf("row %+v", r)
	}
}

func TestListedListingError(t *testing.T) {
	t.Parallel()
	src := &fakeSource{listings: []sources.EarningsListing{
		{Symbol: "BAD", ReportDate: "2026-11-09"},
		{Symbol: "BRK.B", ReportDate: "2026-11-12"},
	}}
	st := &memStore{}
	buildWith(t, fakeTranslator{bad: map[string]bool{"BAD": true}}, src, st, us("cik:9", "BRK B"))
	r := byIssuer(st, "cik:9")
	if len(r) != 1 || r[0].Status != "listed" || r[0].Symbol != "BRK.B" {
		t.Fatalf("row %+v", r)
	}
}

// recHandler is a slog.Handler that keeps the attributes of every record it receives.
type recHandler struct {
	mu   *sync.Mutex
	recs *[]map[string]string
}

func (h recHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h recHandler) Handle(_ context.Context, r slog.Record) error {
	m := map[string]string{"msg": r.Message, "level": r.Level.String()}
	r.Attrs(func(a slog.Attr) bool { m[a.Key] = a.Value.String(); return true })
	h.mu.Lock()
	*h.recs = append(*h.recs, m)
	h.mu.Unlock()
	return nil
}
func (h recHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h recHandler) WithGroup(string) slog.Handler      { return h }

type leakyErr struct{}

func (leakyErr) Error() string             { return "store said 99999" }
func (leakyErr) Kind() contracts.ErrorKind { return contracts.Retryable }

func TestRunFailureLogsStepKind(t *testing.T) {
	t.Parallel()
	good := func() map[string]sources.EarningsSource {
		return map[string]sources.EarningsSource{"US": &fakeSource{}}
	}
	cases := []struct {
		name, step, kind string
		day              contracts.Day
		srcs             map[string]sources.EarningsSource
		entry            jobs.WatchEntry
		setup            func(st *memStore)
	}{
		{"bad_day", "bad_day", "permanent", "not-a-day", good(), us("cik:1", "AAPL"), func(*memStore) {}},
		{"read_day", "read_day", "retryable", day, good(), us("cik:1", "AAPL"), func(st *memStore) { st.readErr = leakyErr{} }},
		{"override_date", "override_date", "permanent", day, good(), func() jobs.WatchEntry {
			e := us("cik:1", "AAPL")
			e.ReportDateOverride = "nope"
			return e
		}(), func(*memStore) {}},
		{"write", "write", "retryable", day, good(), us("cik:1", "AAPL"), func(st *memStore) { st.writeErr = leakyErr{} }},
		{"source_failed", "source_failed", "permanent", day,
			map[string]sources.EarningsSource{"US": &fakeSource{err: errors.New("key 99999")}}, us("cik:1", "AAPL"), func(*memStore) {}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			st := &memStore{}
			c.setup(st)
			j := build(t, c.srcs, st, c.entry)
			var recs []map[string]string
			j.(*job).log = slog.New(recHandler{&sync.Mutex{}, &recs})
			if res := j.Run(context.Background(), c.day, nil); res.Err == nil {
				t.Fatal("want an error")
			}
			if len(recs) != 1 {
				t.Fatalf("records = %v", recs)
			}
			m := recs[0]
			if m["msg"] != "run failed" || m["level"] != "WARN" || m["step"] != c.step || m["kind"] != c.kind || len(m) != 4 {
				t.Fatalf("record = %v", m)
			}
			for _, v := range m {
				if strings.Contains(v, "99999") {
					t.Fatalf("record leaks error text: %v", m)
				}
			}
		})
	}
}

func TestPastDayRunsClockDay(t *testing.T) {
	t.Parallel()
	usSrc := &fakeSource{}
	st := &memStore{}
	j := build(t, map[string]sources.EarningsSource{"US": usSrc}, st, us("cik:1", "A"))
	res := j.Run(context.Background(), "2026-11-06", nil)
	if res.Err != nil || usSrc.calls != 1 || len(st.rows) != 1 {
		t.Fatalf("past day: err %v calls %d rows %d", res.Err, usSrc.calls, len(st.rows))
	}
	if got := st.rows[0].ObservedOn.Format(dayLayout); got != string(day) {
		t.Fatalf("ObservedOn = %s, want the clock's date %s", got, day)
	}
}
