package ivhistory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store/readingkey"
)

const runDay = contracts.Day("2026-11-09")

// fakeClock is a fixed clock.
type fakeClock struct{}

func (fakeClock) Now() time.Time { return time.Date(2026, 11, 9, 21, 40, 0, 0, time.UTC) }

// fakeHistory answers with its function and records every query.
type fakeHistory struct {
	mu      sync.Mutex
	queries []broker.BarQuery
	answer  func(q broker.BarQuery) ([]broker.Bar, error)
}

func (f *fakeHistory) Bars(_ context.Context, q broker.BarQuery) ([]broker.Bar, error) {
	f.mu.Lock()
	f.queries = append(f.queries, q)
	f.mu.Unlock()
	return f.answer(q)
}
func (f *fakeHistory) Close() error { return nil }

// memReadings is a fake reading store: insert-only, idempotent on the key, newest rev per day on read.
type memReadings struct {
	mu    sync.Mutex
	rows  []contracts.ReadingRecord
	calls [][]contracts.ReadingRecord // the batch of every UpsertReadings call
}

// plainWriter has InsertReadings only: it is a store.ReadingWriter without the upsert.
type plainWriter struct{ m *memReadings }

func (p plainWriter) InsertReadings(ctx context.Context, rows []contracts.ReadingRecord) (int, error) {
	return p.m.InsertReadings(ctx, rows)
}

// UpsertReadings keeps the promises of readingkey.ReadingUpserter: a key twice in a batch is a Permanent error,
// a differing reading replaces the stored one of its key, an equal one is skipped and a missing_input yields to a
// stored ok.
func (m *memReadings) UpsertReadings(_ context.Context, rows []contracts.ReadingRecord) (readingkey.WriteCount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, append([]contracts.ReadingRecord(nil), rows...))
	seen := map[string]bool{}
	for _, r := range rows {
		k := r.Name + "|" + r.SubjectKind + "|" + r.Subject + "|" + r.Day.Format(dayLayout)
		if seen[k] {
			return readingkey.WriteCount{}, newErr(contracts.Permanent, "duplicate key %s", k)
		}
		seen[k] = true
		if r.Rev < 1 {
			return readingkey.WriteCount{}, newErr(contracts.Permanent, "reading check (rev >= 1) violated by %s", k)
		}
	}
	var wc readingkey.WriteCount
	for _, r := range rows {
		at := -1
		for i, o := range m.rows {
			if o.Name == r.Name && o.SubjectKind == r.SubjectKind && o.Subject == r.Subject && o.Day.Equal(r.Day.Time) {
				at = i
			}
		}
		switch {
		case at < 0:
			m.rows = append(m.rows, r)
			wc.Written++
		case string(m.rows[at].State) == "ok" && string(r.State) != "ok":
			wc.Skipped++
		default:
			m.rows[at] = r
			wc.Written++
		}
	}
	return wc, nil
}

func recKey(r contracts.ReadingRecord) string {
	return r.Name + "|" + r.SubjectKind + "|" + r.Subject + "|" + r.Day.Format(dayLayout) + "|" + string(rune('0'+r.Rev))
}

func (m *memReadings) InsertReadings(_ context.Context, rows []contracts.ReadingRecord) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range rows {
		if r.Rev < 1 {
			return 0, newErr(contracts.Permanent, "reading check (rev >= 1) violated by %s", recKey(r))
		}
	}
	n := 0
	for _, r := range rows {
		dup := false
		for _, o := range m.rows {
			dup = dup || recKey(o) == recKey(r)
		}
		if !dup {
			m.rows = append(m.rows, r)
			n++
		}
	}
	return n, nil
}

func (m *memReadings) LatestReadings(_ context.Context, name, kind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	newest := map[string]contracts.ReadingRecord{}
	for _, r := range m.rows {
		d := r.Day.Format(dayLayout)
		if r.Name != name || r.SubjectKind != kind || r.Subject != subject || d < string(from) || d > string(to) {
			continue
		}
		if o, ok := newest[d]; !ok || o.Rev < r.Rev {
			newest[d] = r
		}
	}
	var out []contracts.ReadingRecord
	for _, r := range newest {
		out = append(out, r)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Day.Before(out[b].Day.Time) })
	return out, nil
}

// text is a stand-in of ivbar.FromClose for the tests: plain decimals above 0 and not above 10 pass as given.
func text(close string) (string, error) {
	switch close {
	case "0", "0.0", "11", "10.5", "x":
		return "", errBad
	}
	return close, nil
}

type badErr struct{}

func (badErr) Error() string { return "bad close" }

var errBad = badErr{}

func entry(symbol string) jobs.WatchEntry {
	return jobs.WatchEntry{Issuer: "cik:" + symbol, Market: "US", Contract: broker.Contract{SecType: "STK", Symbol: symbol, Currency: "USD", ConID: 100}}
}

func build(t *testing.T, h broker.HistoryClient, st *memReadings, syms ...string) contracts.Job {
	t.Helper()
	var es []jobs.WatchEntry
	for _, s := range syms {
		es = append(es, entry(s))
	}
	j, err := New(h, st, st, es, text, fakeClock{})
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func keysOf(syms ...string) []contracts.ExpectedKey {
	var ks []contracts.ExpectedKey
	for _, s := range syms {
		ks = append(ks, contracts.ExpectedKey{DatasetID: "readings.reading", SubjectKey: "market.iv_ibkr|underlying|" + s})
	}
	return ks
}

func bar(day, close string) broker.Bar { return broker.Bar{Day: day, Close: close} }

func day(s string) types.SerializableDate {
	d, _ := time.Parse(dayLayout, s)
	return types.SerializableDate{Time: d}
}

func TestManifestLoads(t *testing.T) {
	t.Parallel()
	m, err := DefaultManifest()
	if err != nil {
		t.Fatal(err)
	}
	s := m.Schedule
	if m.ID != "iv_history" || m.Reading != "market.iv_ibkr" || m.BackfillDays != 400 || m.DailyDays != 5 || m.LookbackDays != 365 ||
		s.Calendar != "XNYS" || s.Anchor != "session_close" || s.Offset != 10*time.Minute {
		t.Fatalf("manifest %+v", m)
	}
}

func TestManifestRefusesBadDays(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"backfill_days", "daily_days", "lookback_days"} {
		lines := strings.Split(string(manifestBytes), "\n")
		for i, l := range lines {
			if strings.HasPrefix(l, key+" =") {
				lines[i] = key + " = 0"
			}
		}
		if _, err := LoadManifest([]byte(strings.Join(lines, "\n"))); err == nil || !strings.Contains(err.Error(), key) {
			t.Fatalf("%s: got %v", key, err)
		}
	}
}

func TestPlanOneKeyPerNameSorted(t *testing.T) {
	t.Parallel()
	j := build(t, &fakeHistory{}, &memReadings{}, "MSFT", "AAPL")
	keys, h1, err := j.Plan(context.Background(), runDay)
	if err != nil {
		t.Fatal(err)
	}
	want := keysOf("AAPL", "MSFT")
	if len(keys) != 2 || keys[0] != want[0] || keys[1] != want[1] {
		t.Fatalf("keys %+v", keys)
	}
	_, h2, _ := j.Plan(context.Background(), runDay+"x")
	if h1 == h2 || len(h1) != 64 {
		t.Fatalf("hashes %q %q", h1, h2)
	}
}

func TestNewRefusesNilAndDuplicates(t *testing.T) {
	t.Parallel()
	st := &memReadings{}
	if _, err := New(nil, st, st, nil, text, fakeClock{}); err == nil {
		t.Fatal("nil history accepted")
	}
	if _, err := New(&fakeHistory{}, st, st, []jobs.WatchEntry{entry("A"), entry("A")}, text, fakeClock{}); err == nil {
		t.Fatal("duplicate accepted")
	}
}

func TestPostconditionFollowsDayReading(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: func(broker.BarQuery) ([]broker.Bar, error) {
		return []broker.Bar{bar("2026-11-06", "0.30"), bar("2026-11-09", "0.28")}, nil
	}}
	st := &memReadings{}
	j := build(t, h, st, "AAA")
	if res := j.Run(context.Background(), runDay, keysOf("AAA")); res.Err != nil || res.Returned != 1 || res.Usable != 1 {
		t.Fatalf("with the bar of the day: %+v", res)
	}
	h2 := &fakeHistory{answer: func(broker.BarQuery) ([]broker.Bar, error) { return []broker.Bar{bar("2026-11-06", "0.30")}, nil }}
	st2 := &memReadings{}
	j2 := build(t, h2, st2, "AAA")
	if res := j2.Run(context.Background(), runDay, keysOf("AAA")); res.Err != nil || res.Returned != 1 || res.Usable != 0 {
		t.Fatalf("without the bar of the day: %+v", res)
	}
}

func TestPacedAnswerIsRetryable(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: func(broker.BarQuery) ([]broker.Bar, error) { return nil, retryable{} }}
	st := &memReadings{}
	j := build(t, h, st, "AAA")
	res := j.Run(context.Background(), runDay, keysOf("AAA"))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || len(st.rows) != 0 || res.Returned != 0 {
		t.Fatalf("result %+v rows %d", res, len(st.rows))
	}
}

type retryable struct{}

func (retryable) Error() string             { return "paced" }
func (retryable) Kind() contracts.ErrorKind { return contracts.Retryable }
