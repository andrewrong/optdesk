package barsrefresh

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

type atClock struct{ t time.Time }

func (c atClock) Now() time.Time { return c.t }

// fakeCal makes every weekday a session that closes at closeHour UTC (US) or 06:00 UTC (JP); early maps a
// day to its early close.
type fakeCal struct{ early map[contracts.Day]time.Time }

func (f fakeCal) Session(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return clock.Session{}, err
	}
	s := clock.Session{CalendarID: id, Day: day, IsSession: d.Weekday() != time.Saturday && d.Weekday() != time.Sunday}
	s.CloseAt = d.Add(21 * time.Hour)
	if id == "XTKS" {
		s.CloseAt = d.Add(6 * time.Hour)
	}
	if e, ok := f.early[day]; ok {
		s.CloseAt, s.EarlyClose = e, true
	}
	return s, nil
}

func (fakeCal) NextSession(context.Context, string, contracts.Day) (clock.Session, error) {
	return clock.Session{}, errors.New("unused")
}

type fakeBarSource struct {
	mu    sync.Mutex
	calls []sources.BarRequest
	fn    func(sources.BarRequest) (sources.BarResult, error)
}

func (f *fakeBarSource) DailyBars(_ context.Context, req sources.BarRequest) (sources.BarResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req)
	f.mu.Unlock()
	return f.fn(req)
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

var errDeadline = newErr(contracts.Retryable, "history deadline exceeded")

// yahooAll answers every pair with the same bars.
func yahooAll(bars ...sources.RefBar) func(sources.BarRequest) (sources.BarResult, error) {
	return func(req sources.BarRequest) (sources.BarResult, error) {
		var out sources.BarResult
		for _, p := range req.Pairs {
			for _, b := range bars {
				b.Pair = p
				out.Bars = append(out.Bars, b)
			}
		}
		return out, nil
	}
}

func yb(day, close string) sources.RefBar {
	return sources.RefBar{Day: contracts.Day(day), Open: close, High: close, Low: close, Close: close, Volume: "1000"}
}

// buildFB builds a job with the fallback; the weekly step is off unless a test turns it on.
func buildFB(
	t *testing.T, now time.Time, cal clock.Calendar, h *fakeHistory, st bar.PriceBarWriter, src sources.BarSource, entries ...jobs.WatchEntry,
) (*job, *syncBuf) {
	t.Helper()
	rd, _ := st.(bar.PriceBarReader)
	j, err := newJob(&fakeBroker{listed: true}, h, fakeResolver{}, st, rd, entries, atClock{now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WithFallback(j, src, cal); err != nil {
		t.Fatal(err)
	}
	jb := j.(*job)
	jb.m.AdjustedCadenceDays = 0
	buf := &syncBuf{}
	jb.log = slog.New(slog.NewTextHandler(buf, nil))
	return jb, buf
}

var closed = time.Date(2026, 11, 9, 21, 40, 0, 0, time.UTC)

func failAll(string) func(broker.BarQuery) ([]broker.Bar, error) {
	return func(broker.BarQuery) ([]broker.Bar, error) { return nil, errDeadline }
}

func countQueries(h *fakeHistory, conID int64) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, q := range h.queries {
		if q.Contract.ConID == conID {
			n++
		}
	}
	return n
}

func TestFallbackAfterRetryableBars(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: failAll("")}
	st := &memBars{}
	src := &fakeBarSource{fn: yahooAll(yb("2026-11-06", "10.0000"), yb("2026-11-09", "11.0000"))}
	j, _ := buildFB(t, closed, fakeCal{}, h, st, src,
		entry("cik:1", "AAA", 101), entry("cik:2", "BBB", 102), entry("cik:3", "CCC", 103), entry("cik:4", "DDD", 104))
	res := j.Run(context.Background(), runDay, keysOf("cik:1", "cik:2", "cik:3", "cik:4"))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	for id, want := range map[int64]int{101: historyAttempts, 102: historyAttempts, 103: 0, 104: 0} {
		if got := countQueries(h, id); got != want {
			t.Fatalf("Bars calls of %d: %d, want %d", id, got, want)
		}
	}
	if len(src.calls) != 1 || len(src.calls[0].Pairs) != 4 {
		t.Fatalf("DailyBars calls %+v", src.calls)
	}
	if len(st.rows) != 8 {
		t.Fatalf("rows %d", len(st.rows))
	}
	for _, r := range st.rows {
		if r.Source != "yfinance" || r.Basis != "trades" {
			t.Fatalf("row %+v", r)
		}
	}
	if res.Returned != 8 || res.Usable != 8 {
		t.Fatalf("result %+v", res)
	}
}

func TestOneDeadSymbolDoesNotFlipOthers(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		if q.Contract.ConID == 101 {
			return nil, errDeadline
		}
		return []broker.Bar{mkbar("2026-11-09", "20.0000")}, nil
	}}
	st := &memBars{}
	src := &fakeBarSource{fn: yahooAll(yb("2026-11-09", "10.0000"))}
	j, _ := buildFB(t, closed, fakeCal{}, h, st, src, entry("cik:1", "AAA", 101), entry("cik:2", "BBB", 102), entry("cik:3", "CCC", 103))
	if res := j.Run(context.Background(), runDay, keysOf("cik:1", "cik:2", "cik:3")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(src.calls) != 1 || len(src.calls[0].Pairs) != 1 || src.calls[0].Pairs[0].Symbol != "AAA" {
		t.Fatalf("DailyBars %+v", src.calls)
	}
	if countQueries(h, 101) != historyAttempts || countQueries(h, 102) != 1 || countQueries(h, 103) != 1 {
		t.Fatalf("queries %+v", h.queries)
	}
	by := map[int]string{}
	for _, r := range st.rows {
		by[r.InstrumentID] = string(r.Source)
	}
	if by[101] != "yfinance" || by[102] != "ibkr" || by[103] != "ibkr" {
		t.Fatalf("sources %v", by)
	}
}

func TestNoFallbackForPermanent(t *testing.T) {
	t.Parallel()
	cases := map[string]func(broker.BarQuery) ([]broker.Bar, error){
		"permanent error": func(broker.BarQuery) ([]broker.Bar, error) { return nil, newErr(contracts.Permanent, "no permission") },
		"bad bar day":     fixed(broker.Bar{Day: "2026-11-0!", Open: "1", High: "1", Low: "1", Close: "1"}),
	}
	for name, ans := range cases {
		src := &fakeBarSource{fn: yahooAll(yb("2026-11-09", "10.0000"))}
		j, _ := buildFB(t, closed, fakeCal{}, &fakeHistory{answer: ans}, &memBars{}, src, entry("cik:1", "AAA", 101))
		res := j.Run(context.Background(), runDay, keysOf("cik:1"))
		if res.Err == nil || res.Err.Kind() != contracts.Permanent || len(src.calls) != 0 {
			t.Fatalf("%s: err %v, DailyBars %d", name, res.Err, len(src.calls))
		}
	}
	src := &fakeBarSource{fn: yahooAll(yb("2026-11-09", "10.0000"))}
	j, _ := buildFB(t, closed, fakeCal{}, &fakeHistory{answer: failAll("")}, &memBars{}, src, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), runDay, keysOf("cik:unknown")); res.Err == nil || res.Err.Kind() != contracts.Permanent || len(src.calls) != 0 {
		t.Fatalf("unknown key: %v, DailyBars %d", res.Err, len(src.calls))
	}
}

func TestBothSourcesFail(t *testing.T) {
	t.Parallel()
	unanswered := func(code string) func(sources.BarRequest) (sources.BarResult, error) {
		return func(req sources.BarRequest) (sources.BarResult, error) {
			return sources.BarResult{Unanswered: []sources.BarUnanswered{{Pair: req.Pairs[0], Code: code}}}, nil
		}
	}
	cases := map[string]func(sources.BarRequest) (sources.BarResult, error){
		"rate_limited": unanswered(sources.BarCodeRateLimited),
		"network":      unanswered(sources.BarCodeNetwork),
		"timeout":      unanswered(sources.BarCodeTimeout),
		"no_data":      func(sources.BarRequest) (sources.BarResult, error) { return sources.BarResult{}, nil },
	}
	for code, fn := range cases {
		j, _ := buildFB(t, closed, fakeCal{}, &fakeHistory{answer: failAll("")}, &memBars{}, &fakeBarSource{fn: fn}, entry("cik:1", "AAA", 101))
		res := j.Run(context.Background(), runDay, keysOf("cik:1"))
		if res.Err == nil || res.Err.Kind() != contracts.Retryable ||
			!strings.Contains(res.Err.Error(), "history deadline exceeded") || !strings.Contains(res.Err.Error(), code) {
			t.Fatalf("%s: %v", code, res.Err)
		}
	}
	eur := entry("cik:2", "EEE", 102)
	eur.Contract.Currency = "EUR"
	src := &fakeBarSource{fn: yahooAll(yb("2026-11-09", "10.0000"))}
	j, _ := buildFB(t, closed, fakeCal{}, &fakeHistory{answer: failAll("")}, &memBars{}, src, eur)
	// an index key (SPX) and a EUR name: both fail Retryable and nothing is sent
	res := j.Run(context.Background(), runDay, keysOf("cik:2", "index:SPX"))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || len(src.calls) != 0 {
		t.Fatalf("unmapped: err %v, DailyBars %d", res.Err, len(src.calls))
	}
	if !strings.Contains(res.Err.Error(), codeUnmapped) {
		t.Fatalf("err %v", res.Err)
	}
}

func TestFallbackNoWeeklyNoDeepen(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: failAll("")}
	st := &memBars{}
	src := &fakeBarSource{fn: yahooAll(yb("2026-11-09", "10.0000"))}
	j, _ := buildFB(t, closed, fakeCal{}, h, st, src, entry("cik:1", "AAA", 101))
	j.m.AdjustedCadenceDays = 7 // the weekly step and the deepening step are on
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != historyAttempts {
		t.Fatalf("history calls %+v: the fallback pulls no weekly window and no deepening piece", h.queries)
	}
	for _, r := range st.rows {
		if r.Basis != "trades" {
			t.Fatalf("row %+v", r)
		}
	}
}

func ibRow(id int, day, close string) contracts.PriceBar { return stored(id, day, close, 1) }

func TestFallbackBasisDiffers(t *testing.T) {
	t.Parallel()
	run := func(t *testing.T, storedRow contracts.PriceBar, yahooClose string) (*memBars, contracts.JobResult, string) {
		st := &memBars{rows: []contracts.PriceBar{storedRow}}
		src := &fakeBarSource{fn: yahooAll(yb("2026-11-06", yahooClose), yb("2026-11-09", yahooClose))}
		j, buf := buildFB(t, closed, fakeCal{}, &fakeHistory{answer: failAll("")}, st, src, entry("cik:1", "AAA", 101))
		return st, j.Run(context.Background(), runDay, keysOf("cik:1")), buf.String()
	}
	t.Run("beyond tolerance", func(t *testing.T) {
		t.Parallel()
		st, res, _ := run(t, ibRow(101, "2026-11-06", "100.0000"), "101.0000")
		if res.Err == nil || res.Err.Kind() != contracts.Retryable || !strings.Contains(res.Err.Error(), "fallback_basis_differs") {
			t.Fatalf("err %v", res.Err)
		}
		if len(st.rows) != 1 || st.rows[0].Source != "ibkr" {
			t.Fatalf("rows %+v", st.rows)
		}
	})
	t.Run("within tolerance", func(t *testing.T) {
		t.Parallel()
		st, res, log := run(t, ibRow(101, "2026-11-06", "100.0000"), "100.2000")
		if res.Err != nil || len(st.rows) != 2 || !strings.Contains(log, "largest difference 0.002000") {
			t.Fatalf("err %v rows %+v log %s", res.Err, st.rows, log)
		}
	})
	t.Run("no overlap", func(t *testing.T) {
		t.Parallel()
		st, res, log := run(t, ibRow(101, "2026-11-03", "100.0000"), "300.0000")
		if res.Err != nil || len(st.rows) != 3 || !strings.Contains(log, "no overlap") {
			t.Fatalf("err %v rows %+v log %s", res.Err, st.rows, log)
		}
	})
}

func TestFallbackIncompleteBar(t *testing.T) {
	t.Parallel()
	good := yb("2026-11-09", "10.0000")
	noOpen, noHigh, noLow, noClose := yb("2026-11-02", "10.0000"), yb("2026-11-03", "10.0000"), yb("2026-11-04", "10.0000"), yb("2026-11-05", "10.0000")
	noOpen.Open, noHigh.High, noLow.Low, noClose.Close = "", "", "", ""
	st := &memBars{}
	src := &fakeBarSource{fn: yahooAll(noOpen, noHigh, noLow, noClose, good)}
	j, buf := buildFB(t, closed, fakeCal{}, &fakeHistory{answer: failAll("")}, st, src, entry("cik:1", "AAA", 101))
	res := j.Run(context.Background(), runDay, keysOf("cik:1"))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || !strings.Contains(res.Err.Error(), "incomplete") {
		t.Fatalf("err %v", res.Err)
	}
	if len(st.rows) != 1 || st.rows[0].Volume != nil || st.rows[0].SessionDay.Format(dayLayout) != "2026-11-09" {
		t.Fatalf("rows %+v", st.rows)
	}
	if !strings.Contains(buf.String(), "incomplete=1") {
		t.Fatalf("log %s", buf.String())
	}
}

func TestFallbackDropsOpenSession(t *testing.T) {
	t.Parallel()
	jp := entry("cik:2", "7203", 102)
	jp.Contract.Currency = "JPY"
	us := entry("cik:1", "AAA", 101)
	cases := []struct {
		name    string
		now     time.Time
		day     contracts.Day
		cal     fakeCal
		e       jobs.WatchEntry
		key     string
		wantDay string // the only day written; the session of the run day is in progress
	}{
		{"us mid-session", time.Date(2026, 11, 9, 15, 0, 0, 0, time.UTC), "2026-11-09", fakeCal{}, us, "cik:1", "2026-11-06"},
		{"jp mid-session", time.Date(2026, 11, 9, 3, 0, 0, 0, time.UTC), "2026-11-09", fakeCal{}, jp, "cik:2", "2026-11-06"},
		{"half day", time.Date(2026, 11, 27, 17, 0, 0, 0, time.UTC), "2026-11-27", fakeCal{early: map[contracts.Day]time.Time{
			"2026-11-27": time.Date(2026, 11, 27, 18, 0, 0, 0, time.UTC)}}, us, "cik:1", "2026-11-25"},
		{"half day after the early close", time.Date(2026, 11, 27, 19, 0, 0, 0, time.UTC), "2026-11-27", fakeCal{early: map[contracts.Day]time.Time{
			"2026-11-27": time.Date(2026, 11, 27, 18, 0, 0, 0, time.UTC)}}, us, "cik:1", "2026-11-27"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			prev := "2026-11-06"
			if string(c.day) == "2026-11-27" {
				prev = "2026-11-25"
			}
			st := &memBars{}
			src := &fakeBarSource{fn: yahooAll(yb(prev, "10.0000"), yb(string(c.day), "11.0000"))}
			j, _ := buildFB(t, c.now, c.cal, &fakeHistory{answer: failAll("")}, st, src, c.e)
			if res := j.Run(context.Background(), c.day, keysOf(c.key)); res.Err != nil {
				t.Fatal(res.Err)
			}
			var got []string
			for _, r := range st.rows {
				got = append(got, r.SessionDay.Format(dayLayout))
			}
			wantRows := []string{prev}
			if c.wantDay == string(c.day) {
				wantRows = []string{prev, string(c.day)}
			}
			if !reflect.DeepEqual(got, wantRows) {
				t.Fatalf("days written %v, want %v", got, wantRows)
			}
		})
	}
}

func yfRow(id int, day, close string) contracts.PriceBar {
	r := stored(id, day, close, 1)
	r.Source = "yfinance"
	return r
}

func TestRecoveryLeavesNoYfinanceRow(t *testing.T) {
	t.Parallel()
	answer := fixed(mkbar("2026-10-01", "10.0000"), mkbar("2026-11-06", "10.0000"), mkbar("2026-11-09", "11.0000"))
	h := &fakeHistory{answer: answer}
	st := &memBars{rows: []contracts.PriceBar{yfRow(101, "2026-10-01", "10.0000"), yfRow(101, "2026-11-06", "10.0000")}}
	j, _ := buildFB(t, closed, fakeCal{}, h, st, &fakeBarSource{fn: yahooAll()}, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || h.queries[0].DurationDays != 40 {
		t.Fatalf("queries %+v", h.queries)
	}
	if len(st.rows) != 3 {
		t.Fatalf("rows %+v", st.rows)
	}
	for _, r := range st.rows {
		if r.Source != "ibkr" {
			t.Fatalf("a yfinance row remains: %+v", r)
		}
	}
	h2 := &fakeHistory{answer: answer}
	st2 := &memBars{rows: []contracts.PriceBar{ibRow(101, "2026-11-06", "10.0000")}}
	j2, _ := buildFB(t, closed, fakeCal{}, h2, st2, &fakeBarSource{fn: yahooAll()}, entry("cik:1", "AAA", 101))
	if res := j2.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h2.queries) != 1 || h2.queries[0].DurationDays != 5 {
		t.Fatalf("queries %+v", h2.queries)
	}
}

func TestDroppedNameKeepsYfinanceRows(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "11.0000"))}
	st := &memBars{rows: []contracts.PriceBar{
		yfRow(999, "2026-11-06", "10.0000"), // a name no longer in the list
		yfRow(102, "2024-01-02", "10.0000"), // not backfilled, and past the cap of one first pull
	}}
	j, _ := buildFB(t, closed, fakeCal{}, h, st, &fakeBarSource{fn: yahooAll()}, entry("cik:1", "AAA", 101), entry("cik:2", "BBB", 102))
	j.m.BackfillNamesPerRun = 1
	if res := j.Run(context.Background(), runDay, keysOf("cik:1", "cik:2")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if countQueries(h, 102) != 0 || countQueries(h, 101) != 1 {
		t.Fatalf("queries %+v", h.queries)
	}
	for _, id := range []int{999, 102} {
		rows, _ := st.PriceBars(context.Background(), int64(id), "trades", firstDay, "2026-12-31")
		if len(rows) != 1 || rows[0].Source != "yfinance" {
			t.Fatalf("name %d rows %+v", id, rows)
		}
	}
}

// precedenceBars applies the rule of the store: a yfinance row never replaces an ibkr row.
type precedenceBars struct{ *memBars }

func (p precedenceBars) UpsertPriceBars(ctx context.Context, rows []contracts.PriceBar, by string) (bar.BarWriteCount, error) {
	var n bar.BarWriteCount
	var pass []contracts.PriceBar
	for _, r := range rows {
		kept := false
		if r.Source != "ibkr" {
			day := contracts.Day(r.SessionDay.Format(dayLayout))
			old, _ := p.PriceBars(ctx, int64(r.InstrumentID), string(r.Basis), day, day)
			kept = len(old) == 1 && old[0].Source == "ibkr"
		}
		if kept {
			n.Unchanged++
		} else {
			pass = append(pass, r)
		}
	}
	c, err := p.memBars.UpsertPriceBars(ctx, pass, by)
	n.Written += c.Written
	n.Unchanged += c.Unchanged
	return n, err
}

func TestFallbackIdempotent(t *testing.T) {
	t.Parallel()
	mem := &memBars{rows: []contracts.PriceBar{ibRow(101, "2026-11-06", "100.0000")}}
	src := &fakeBarSource{fn: yahooAll(yb("2026-11-06", "100.1000"), yb("2026-11-09", "101.0000"))}
	j, buf := buildFB(t, closed, fakeCal{}, &fakeHistory{answer: failAll("")}, precedenceBars{mem}, src, entry("cik:1", "AAA", 101))
	j.r = mem
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	first := append([]contracts.PriceBar(nil), mem.rows...)
	if len(first) != 2 || first[0].Source != "ibkr" || first[0].Close != "100.0000" || first[1].Source != "yfinance" {
		t.Fatalf("rows %+v", first)
	}
	if !strings.Contains(buf.String(), "written=1 unchanged=1") {
		t.Fatalf("first run log %s", buf.String())
	}
	buf.b.Reset()
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if !reflect.DeepEqual(first, mem.rows) {
		t.Fatalf("rows changed: %+v", mem.rows)
	}
	if !strings.Contains(buf.String(), "written=0 unchanged=2") {
		t.Fatalf("second run log %s", buf.String())
	}
}

func TestWithFallbackRefusals(t *testing.T) {
	t.Parallel()
	j, _ := buildFB(t, closed, fakeCal{}, &fakeHistory{}, &memBars{}, &fakeBarSource{}, entry("cik:1", "AAA", 101))
	if _, err := WithFallback(j, nil, fakeCal{}); err == nil {
		t.Fatal("nil source")
	}
	if _, err := WithFallback(j, &fakeBarSource{}, nil); err == nil {
		t.Fatal("nil calendar")
	}
	if _, err := WithFallback(otherJob{}, &fakeBarSource{}, fakeCal{}); err == nil {
		t.Fatal("job of another package")
	}
}

type otherJob struct{ contracts.Job }

func TestAttemptsCountKeyOnceForHistDown(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: failAll("")}
	src := &fakeBarSource{fn: yahooAll(yb("2026-11-09", "10.0000"))}
	j, _ := buildFB(t, closed, fakeCal{}, h, &memBars{}, src,
		entry("cik:1", "AAA", 101), entry("cik:2", "BBB", 102), entry("cik:3", "CCC", 103))
	if res := j.Run(context.Background(), runDay, keysOf("cik:1", "cik:2", "cik:3")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if countQueries(h, 101) != historyAttempts || countQueries(h, 102) != historyAttempts || countQueries(h, 103) != 0 {
		t.Fatalf("queries %+v", h.queries)
	}
}
