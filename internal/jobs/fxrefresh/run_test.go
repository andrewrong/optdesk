package fxrefresh

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math/big"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

const runDay = contracts.Day("2026-11-09")

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 11, 9, 21, 40, 0, 0, time.UTC) }

type fakeBroker struct {
	broker.Broker
	mu       sync.Mutex
	resolved []broker.Contract
	listed   bool
}

func (f *fakeBroker) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolved = append(f.resolved, c)
	c.ConID = 9001
	return broker.Resolved{Listed: f.listed, Contract: c}, nil
}

type fakeHistory struct {
	mu      sync.Mutex
	queries []broker.BarQuery
	bars    []broker.Bar
	err     error
}

func (f *fakeHistory) Bars(_ context.Context, q broker.BarQuery) ([]broker.Bar, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, q)
	return f.bars, f.err
}
func (f *fakeHistory) Close() error { return nil }

type memFx struct {
	mu        sync.Mutex
	rows      []contracts.FxRate
	calls     int
	upserts   int
	writtenBy []string
	counts    []bar.FxWriteCount
}

// UpsertFxRates follows Decision 4 on a map by day: numeric equality decides, a differing row replaces.
func (m *memFx) UpsertFxRates(_ context.Context, rows []contracts.FxRate, writtenBy string) (bar.FxWriteCount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upserts++
	m.writtenBy = append(m.writtenBy, writtenBy)
	var c bar.FxWriteCount
	for _, r := range rows {
		idx := -1
		for i, s := range m.rows {
			if s.Pair == r.Pair && s.RateDay.Format(dayLayout) == r.RateDay.Format(dayLayout) {
				idx = i
			}
		}
		if idx < 0 {
			m.rows = append(m.rows, r)
			c.Written++
			continue
		}
		old, _ := new(big.Rat).SetString(m.rows[idx].Rate)
		nv, _ := new(big.Rat).SetString(r.Rate)
		if old != nil && nv != nil && old.Cmp(nv) == 0 && m.rows[idx].Source == r.Source {
			c.Unchanged++
			continue
		}
		m.rows[idx].Rate, m.rows[idx].Source = r.Rate, r.Source
		c.Written++
	}
	m.counts = append(m.counts, c)
	return c, nil
}

// plainWriter is only an FxRateWriter.
type plainWriter struct{ calls int }

func (p *plainWriter) InsertFxRates(context.Context, []contracts.FxRate) (int, error) {
	p.calls++
	return 0, nil
}

type failUpsert struct{ memFx }

func (f *failUpsert) UpsertFxRates(context.Context, []contracts.FxRate, string) (bar.FxWriteCount, error) {
	return bar.FxWriteCount{}, errors.New("db down")
}

func (m *memFx) InsertFxRates(_ context.Context, rows []contracts.FxRate) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.rows = append(m.rows, rows...)
	return len(rows), nil
}

func (m *memFx) FxRates(_ context.Context, p string, from, to contracts.Day) ([]contracts.FxRate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []contracts.FxRate
	for _, r := range m.rows {
		if d := r.RateDay.Format(dayLayout); r.Pair == p && d >= string(from) && d <= string(to) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memFx) LatestFxRate(context.Context, string, contracts.Day) (contracts.FxRate, bool, error) {
	return contracts.FxRate{}, false, nil
}

func stored(day, rate string) contracts.FxRate {
	d, _ := time.Parse(dayLayout, day)
	return contracts.FxRate{Pair: pair, Rate: rate, RateDay: types.SerializableDate{Time: d}, Source: sourceMidpoint}
}

func mkbar(day, close string) broker.Bar {
	return broker.Bar{Day: day, Open: close, High: close, Low: close, Close: close}
}

func build(t *testing.T, b *fakeBroker, h *fakeHistory, st *memFx) contracts.Job {
	t.Helper()
	j, err := New(b, h, st, st, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	return j
}

var usdjpy = []contracts.ExpectedKey{{DatasetID: datasetID, SubjectKey: pair}}

func TestOneKeyUSDJPY(t *testing.T) {
	t.Parallel()
	b := &fakeBroker{listed: true}
	h := &fakeHistory{bars: []broker.Bar{mkbar("2026-11-09", "153.2150")}}
	st := &memFx{}
	j := build(t, b, h, st)
	keys, hash, err := j.Plan(context.Background(), runDay)
	if err != nil || hash == "" || len(keys) != 1 || keys[0].DatasetID != "obs.fx_rate" || keys[0].SubjectKey != "USDJPY" {
		t.Fatalf("plan %+v %q %v", keys, hash, err)
	}
	if j.ID() != "fx_refresh" {
		t.Fatalf("id %q", j.ID())
	}
	res := j.Run(context.Background(), runDay, keys)
	if res.Err != nil || res.Returned != 1 || res.Usable != 1 {
		t.Fatalf("result %+v", res)
	}
	if len(b.resolved) != 1 {
		t.Fatalf("resolved %d times", len(b.resolved))
	}
	c := b.resolved[0]
	if c.Exchange != "IDEALPRO" || c.Currency != "JPY" || c.Symbol != "USD" || c.SecType != "CASH" {
		t.Fatalf("contract %+v", c)
	}
	q := h.queries[0]
	if len(h.queries) != 1 || q.Kind != broker.BarMidpoint || q.DurationDays != 5 || q.Contract.ConID != 9001 {
		t.Fatalf("queries %+v", h.queries)
	}
}

func TestReplacesDifferingDay(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{bars: []broker.Bar{mkbar("2026-11-05", "151.1000"), mkbar("2026-11-06", "152.2000"), mkbar("2026-11-09", "153.3000")}}
	st := &memFx{rows: []contracts.FxRate{stored("2026-11-06", "999.0000")}}
	j := build(t, &fakeBroker{listed: true}, h, st)
	res := j.Run(context.Background(), runDay, usdjpy)
	if res.Err != nil || res.Returned != 3 || res.Usable != 1 {
		t.Fatalf("result %+v", res)
	}
	if st.upserts != 1 || st.writtenBy[0] != "fx_refresh:2026-11-09" {
		t.Fatalf("upserts %d by %v", st.upserts, st.writtenBy)
	}
	if st.counts[0] != (bar.FxWriteCount{Written: 3}) {
		t.Fatalf("counts %+v", st.counts)
	}
	if len(st.rows) != 3 || st.rows[0].Rate != "152.2000" {
		t.Fatalf("rows %+v", st.rows)
	}
	for _, r := range st.rows {
		if r.Source != sourceMidpoint || r.Pair != "USDJPY" {
			t.Fatalf("row %+v", r)
		}
	}
}

func TestEqualDayNoRewrite(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{bars: []broker.Bar{mkbar("2026-11-09", "153.4250")}}
	st := &memFx{rows: []contracts.FxRate{stored("2026-11-09", "153.425")}}
	j := build(t, &fakeBroker{listed: true}, h, st)
	res := j.Run(context.Background(), runDay, usdjpy)
	if res.Err != nil || res.Usable != 1 {
		t.Fatalf("result %+v", res)
	}
	if st.counts[0] != (bar.FxWriteCount{Unchanged: 1}) || st.rows[0].Rate != "153.425" {
		t.Fatalf("counts %+v rows %+v", st.counts, st.rows)
	}
}

func TestPastDayAsksEndDay(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{bars: []broker.Bar{mkbar("2026-11-06", "152.2000")}}
	j := build(t, &fakeBroker{listed: true}, h, &memFx{})
	if res := j.Run(context.Background(), "2026-11-06", usdjpy); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || h.queries[0].End != "2026-11-06" {
		t.Fatalf("queries %+v", h.queries)
	}
}

func TestTodayAsksNoEnd(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{bars: []broker.Bar{mkbar("2026-11-09", "153.2000")}}
	j := build(t, &fakeBroker{listed: true}, h, &memFx{})
	if res := j.Run(context.Background(), runDay, usdjpy); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || h.queries[0].End != "" {
		t.Fatalf("queries %+v", h.queries)
	}
}

func TestBarsAfterDayAreDropped(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{bars: []broker.Bar{mkbar("2026-11-05", "151.1000"), mkbar("2026-11-06", "152.2000"), mkbar("2026-11-07", "153.3000")}}
	st := &memFx{}
	j := build(t, &fakeBroker{listed: true}, h, st)
	res := j.Run(context.Background(), "2026-11-06", usdjpy)
	if res.Err != nil || res.Returned != 2 {
		t.Fatalf("result %+v", res)
	}
	if st.upserts != 1 || len(st.rows) != 2 || st.rows[1].RateDay.Format(dayLayout) != "2026-11-06" {
		t.Fatalf("rows %+v upserts %d", st.rows, st.upserts)
	}
}

func TestEmptyAnswerNeverReplaces(t *testing.T) {
	t.Parallel()
	st := &memFx{rows: []contracts.FxRate{stored("2026-11-09", "153.3000")}}
	j := build(t, &fakeBroker{listed: true}, &fakeHistory{}, st)
	res := j.Run(context.Background(), runDay, usdjpy)
	if res.Err != nil || res.Returned != 0 || st.upserts != 0 || len(st.rows) != 1 || st.rows[0].Rate != "153.3000" {
		t.Fatalf("result %+v rows %+v upserts %d", res, st.rows, st.upserts)
	}
}

func TestRunWithoutUpserterIsRetryable(t *testing.T) {
	t.Parallel()
	b := &fakeBroker{listed: true}
	h := &fakeHistory{bars: []broker.Bar{mkbar("2026-11-09", "153.2150")}}
	w := &plainWriter{}
	j, err := New(b, h, w, &memFx{}, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), runDay, usdjpy)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || !strings.Contains(res.Err.Error(), "bar store") {
		t.Fatalf("result %+v", res)
	}
	if len(b.resolved) != 0 || len(h.queries) != 0 || w.calls != 0 {
		t.Fatalf("broker %d history %d insert %d", len(b.resolved), len(h.queries), w.calls)
	}
}

func TestBadBarIsPermanent(t *testing.T) {
	t.Parallel()
	for _, bars := range [][]broker.Bar{{mkbar("2026-11-09", "0")}, {mkbar("2026-11-09", "abc")}, {mkbar("bad", "1.5")}} {
		st := &memFx{}
		j := build(t, &fakeBroker{listed: true}, &fakeHistory{bars: bars}, st)
		res := j.Run(context.Background(), runDay, usdjpy)
		if res.Err == nil || res.Err.Kind() != contracts.Permanent || st.upserts != 0 {
			t.Fatalf("%+v: result %+v", bars, res)
		}
	}
}

func TestStoreErrorIsRetryable(t *testing.T) {
	t.Parallel()
	st := &failUpsert{}
	j, err := New(&fakeBroker{listed: true}, &fakeHistory{bars: []broker.Bar{mkbar("2026-11-09", "153.2")}}, st, &st.memFx, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), runDay, usdjpy)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("result %+v", res)
	}
}

func TestEmptyAnswerWritesNothing(t *testing.T) {
	t.Parallel()
	st := &memFx{}
	j := build(t, &fakeBroker{listed: true}, &fakeHistory{}, st)
	res := j.Run(context.Background(), runDay, usdjpy)
	if res.Err != nil || res.Returned != 0 || res.Usable != 0 || st.calls != 0 || len(st.rows) != 0 {
		t.Fatalf("result %+v rows %+v calls %d", res, st.rows, st.calls)
	}
}

func TestNotListedIsPermanent(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{}
	j := build(t, &fakeBroker{listed: false}, h, &memFx{})
	res := j.Run(context.Background(), runDay, usdjpy)
	if res.Err == nil || res.Err.Kind() != contracts.Permanent || len(h.queries) != 0 {
		t.Fatalf("result %+v", res)
	}
}

type pacedErr struct{}

func (pacedErr) Error() string             { return "paced" }
func (pacedErr) Kind() contracts.ErrorKind { return contracts.Retryable }

func TestPacedIsRetryable(t *testing.T) {
	t.Parallel()
	for _, err := range []error{pacedErr{}, errors.New("socket closed")} {
		h := &fakeHistory{err: err}
		j := build(t, &fakeBroker{listed: true}, h, &memFx{})
		res := j.Run(context.Background(), runDay, usdjpy)
		if res.Err == nil || res.Err.Kind() != contracts.Retryable {
			t.Fatalf("%v: result %+v", err, res)
		}
	}
}

func isNum(e ast.Expr) bool {
	for {
		switch v := e.(type) {
		case *ast.BasicLit:
			return v.Kind == token.INT || v.Kind == token.FLOAT
		case *ast.UnaryExpr:
			e = v.X
		case *ast.ParenExpr:
			e = v.X
		default:
			return false
		}
	}
}

func rateName(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.Ident:
		return containsRate(v.Name)
	case *ast.SelectorExpr:
		return containsRate(v.Sel.Name)
	}
	return false
}

func containsRate(s string) bool { return strings.Contains(strings.ToLower(s), "rate") }

// numericRateAssignment reports whether n assigns a numeric literal to something named like a rate.
func numericRateAssignment(n ast.Node) bool {
	switch v := n.(type) {
	case *ast.AssignStmt:
		for i, l := range v.Lhs {
			if i < len(v.Rhs) && rateName(l) && isNum(v.Rhs[i]) {
				return true
			}
		}
	case *ast.ValueSpec:
		for i, l := range v.Names {
			if i < len(v.Values) && containsRate(l.Name) && isNum(v.Values[i]) {
				return true
			}
		}
	case *ast.KeyValueExpr:
		return rateName(v.Key) && isNum(v.Value)
	}
	return false
}

// TestNoDefaultRate fails when a numeric literal is assigned to anything named like a rate.
func TestNoDefaultRate(t *testing.T) {
	t.Parallel()
	names, err := filepath.Glob("*.go")
	if err != nil || len(names) == 0 {
		t.Fatalf("no source: %v", err)
	}
	fset := token.NewFileSet()
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if numericRateAssignment(n) {
				t.Errorf("%s: numeric literal assigned to a rate", name)
			}
			return true
		})
	}
}
