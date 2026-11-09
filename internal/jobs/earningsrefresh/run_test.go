package earningsrefresh

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 11, 9, 21, 30, 0, 0, time.UTC) }

// memStore is a writer and a reader over one slice; a key that exists is left as it is.
type memStore struct {
	mu   sync.Mutex
	rows []contracts.EarningsDate
}

func (m *memStore) InsertEarningsDate(_ context.Context, r contracts.EarningsDate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
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

func TestErrorDayNotRetried(t *testing.T) {
	t.Parallel()
	src := &fakeSource{err: codedErr{"network"}}
	st := &memStore{}
	j := build(t, map[string]sources.EarningsSource{"US": src}, st, us("cik:1", "A"))
	j.Run(context.Background(), day, nil)
	j.Run(context.Background(), day, nil)
	if src.calls != 1 || len(st.rows) != 1 {
		t.Fatalf("calls %d rows %d", src.calls, len(st.rows))
	}
	// the next day starts again
	j.Run(context.Background(), "2026-11-10", nil)
	if src.calls != 2 {
		t.Fatalf("next day calls %d", src.calls)
	}
}

func TestRateLimitedRecordedNoRetry(t *testing.T) {
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

func TestSecondRunAfterErrorReturnsPermanent(t *testing.T) {
	t.Parallel()
	src := &fakeSource{err: errors.New("down")}
	st := &memStore{}
	j := build(t, map[string]sources.EarningsSource{"US": src}, st, us("cik:1", "A"))
	first := j.Run(context.Background(), day, nil)
	second := j.Run(context.Background(), day, nil)
	if first.Err == nil || second.Err == nil || second.Err.Kind() != contracts.Permanent || second.Err.Error() != first.Err.Error() {
		t.Fatalf("first %v second %v", first.Err, second.Err)
	}
	if src.calls != 1 || len(st.rows) != 1 || st.rows[0].ErrorCode != "unknown" {
		t.Fatalf("calls %d rows %+v", src.calls, st.rows)
	}
}
