package optioncandidates

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optioncand"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	priorDay   = contracts.Day("2026-11-06")
	termExpiry = contracts.Day("2026-12-11") // 32 days after runDay
)

// srcEnv holds every store of the source tests: the open rows, the eod rows by underlying and the terms.
type srcEnv struct {
	mu      sync.Mutex
	open    []contracts.OptionOpen
	openErr error
	eod     []contracts.OptionEod
	terms   map[int64]store.OptionTerms
	catalog map[int64]contracts.Day
	symbols map[int64]string
	written []store.Candidate
	inputs  []compute.CandidateInput
	openGet int
}

func newSrcEnv() *srcEnv {
	return &srcEnv{
		terms:   map[int64]store.OptionTerms{},
		catalog: map[int64]contracts.Day{},
		symbols: map[int64]string{},
	}
}

func (e *srcEnv) OptionOpen(context.Context, contracts.Day) ([]contracts.OptionOpen, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.openGet++
	return e.open, e.openErr
}

func (e *srcEnv) OptionEod(_ context.Context, id int64, from, to contracts.Day) ([]contracts.OptionEod, error) {
	var out []contracts.OptionEod
	for _, r := range e.eod {
		if int64(r.UnderlyingID) == id && contracts.Day(r.SessionDay.Format("2006-01-02")) >= from &&
			contracts.Day(r.SessionDay.Format("2006-01-02")) <= to {
			out = append(out, r)
		}
	}
	return out, nil
}

func (e *srcEnv) OptionEodLatest(context.Context, contracts.Day) ([]store.OptionEodLatest, error) {
	var out []store.OptionEodLatest
	for id, d := range e.catalog {
		out = append(out, store.OptionEodLatest{UnderlyingID: id, SessionDay: d})
	}
	return out, nil
}

func (e *srcEnv) OptionTerms(_ context.Context, ids []int64) ([]store.OptionTerms, error) {
	var out []store.OptionTerms
	for _, id := range ids {
		if t, ok := e.terms[id]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

func (e *srcEnv) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, id := range ids {
		out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: e.symbols[id]})
	}
	return out, nil
}

func (e *srcEnv) InsertCandidates(_ context.Context, rows []store.Candidate) (int, error) {
	e.mu.Lock()
	e.written = append(e.written, rows...)
	e.mu.Unlock()
	return len(rows), nil
}

func (e *srcEnv) SessionDays(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SessionDay, error) {
	var out []contracts.SessionDay
	for _, d := range []string{"2026-11-04", "2026-11-05", "2026-11-06", "2026-11-09"} {
		out = append(out, contracts.SessionDay{CalendarID: "XNYS", SessionDay: sd(d), IsSession: true})
	}
	return out, nil
}

// Evaluate records the input and runs the real rule.
func (e *srcEnv) Evaluate(in compute.CandidateInput, p compute.CandidateParams) []compute.Candidate {
	e.mu.Lock()
	e.inputs = append(e.inputs, in)
	e.mu.Unlock()
	return optioncand.New().Evaluate(in, p)
}

func sd(s string) types.SerializableDate { return day(s) }

// underlying adds a watched underlying with the chain day of its eod fallback.
func (e *srcEnv) underlying(id int64, symbol string) {
	e.symbols[id] = symbol
	e.catalog[id] = priorDay
}

// put declares the terms of an instrument.
func (e *srcEnv) put(ul, id int64, strike string) {
	e.terms[id] = store.OptionTerms{InstrumentID: id, Right: "P", Strike: strike, Multiplier: "100", ExpirationDate: termExpiry}
	_ = ul
}

func (e *srcEnv) call(id int64, strike string) {
	e.terms[id] = store.OptionTerms{InstrumentID: id, Right: "C", Strike: strike, Multiplier: "100", ExpirationDate: termExpiry}
}

func openRowOf(ul, id, status int, bid, ask float64) contracts.OptionOpen {
	r := contracts.OptionOpen{
		UnderlyingID: ul, InstrumentID: id, Currency: "USD", Status: status, BidSize: f64(5),
		UnderlyingPrice: f64(100), MarketDataType: 1, CaptureID: "o1",
	}
	if bid > 0 {
		r.Bid = f64(bid)
	}
	if ask > 0 {
		r.Ask = f64(ask)
	}
	return r
}

func eodRowOf(ul, id int, d string, bid, ask, oi float64) contracts.OptionEod {
	return contracts.OptionEod{
		UnderlyingID: ul, InstrumentID: id, SessionDay: sd(d), Bid: f64(bid), Ask: f64(ask), BidSize: f64(5),
		OpenInterest: f64(oi), UnderlyingPrice: f64(99), Status: 1, MarketDataType: 1, CaptureID: "e1",
	}
}

func (e *srcEnv) stores(withOpen bool) CandidateStores {
	st := CandidateStores{
		OptionEod: e, Catalog: e, Terms: e, Names: e, Candidates: e, SessionDays: e,
	}
	if withOpen {
		st.Open = e
	}
	return st
}

type logBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *logBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *logBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func entryFor(symbol, issuer, buy string) jobs.WatchEntry {
	return jobs.WatchEntry{Issuer: issuer, Market: "US", BuyPrice: buy, Contract: broker.Contract{Symbol: symbol, SecType: "STK"}}
}

type skipper interface{ Skipped() []string }

// run builds the put job, runs it and returns the job with its log text.
func (e *srcEnv) run(t *testing.T, withOpen bool, entries ...jobs.WatchEntry) (contracts.Job, *logBuf) {
	t.Helper()
	lb := &logBuf{}
	st := e.stores(withOpen)
	st.log = slog.New(slog.NewTextHandler(lb, nil))
	j, err := New(st, e, entries, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), runDay, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	return j, lb
}

func (e *srcEnv) rowOf(t *testing.T, id int) store.Candidate {
	t.Helper()
	for _, r := range e.written {
		if r.InstrumentID == id {
			return r
		}
	}
	t.Fatalf("no row for instrument %d in %d rows", id, len(e.written))
	return store.Candidate{}
}

func (e *srcEnv) hasRow(id int) bool {
	for _, r := range e.written {
		if r.InstrumentID == id {
			return true
		}
	}
	return false
}

func (e *srcEnv) inputOf(symbol string) compute.CandidateInput {
	for _, in := range e.inputs {
		if in.Underlying == symbol {
			return in
		}
	}
	return compute.CandidateInput{}
}

func TestSourceNeedHighEqualsNeedRow(t *testing.T) {
	t.Parallel()
	m, _, _, err := parseManifest(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	var high string
	for _, r := range m.Need {
		if r.Strategy == "short_put" {
			high = r.High
		}
	}
	if high == "" || m.Source.NeedHigh != high || m.Source.OpenMinRows != 1 || m.Source.OILookbackSessions != 3 {
		t.Fatalf("source %+v, short_put high %q", m.Source, high)
	}
}

func TestSourceManifestErrorNamesKeyNotValue(t *testing.T) {
	t.Parallel()
	err := checkSource(sourceParams{OpenMinRows: 1, NeedHigh: "secret-value", OILookbackSessions: 3})
	if err == nil || !strings.Contains(err.Error(), "need_high") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("err %v", err)
	}
	if err := checkSource(sourceParams{NeedHigh: "1.05", OILookbackSessions: 3}); err == nil || !strings.Contains(err.Error(), "open_min_rows") {
		t.Fatalf("err %v", err)
	}
	if err := checkSource(sourceParams{OpenMinRows: 1, NeedHigh: "1.05"}); err == nil || !strings.Contains(err.Error(), "oi_lookback_sessions") {
		t.Fatalf("err %v", err)
	}
}

func TestNilOpenIsTheJobOfToday(t *testing.T) {
	t.Parallel()
	mk := func() *srcEnv {
		e := newSrcEnv()
		e.underlying(7, "ABC")
		e.put(7, 11, "90")
		e.eod = []contracts.OptionEod{eodRowOf(7, 11, "2026-11-06", 1.99, 2.05, 40)}
		return e
	}
	entries := []jobs.WatchEntry{entryFor("ABC", "cik:1", "91.5"), entryFor("ZZZ", "cik:2", "10")}
	a, b := mk(), mk()
	ja, _ := a.run(t, false, entries...)
	b.open = nil
	jb, _ := b.run(t, true, entries...)
	if len(a.written) != 1 || len(b.written) != 1 {
		t.Fatalf("rows %d and %d", len(a.written), len(b.written))
	}
	ra, rb := a.written[0], b.written[0]
	if ra.RunID != rb.RunID || ra.Marks != rb.Marks || ra.Gates != rb.Gates || ra.BidEod != rb.BidEod || ra.Status != rb.Status {
		t.Fatalf("rows differ:\n%+v\n%+v", ra, rb)
	}
	if strings.Contains(ra.Marks, "eod_fallback") || strings.Join(ja.(skipper).Skipped(), "|") != strings.Join(jb.(skipper).Skipped(), "|") {
		t.Fatalf("marks %s, skipped %v and %v", ra.Marks, ja.(skipper).Skipped(), jb.(skipper).Skipped())
	}
}

func TestSourceOpenChosenByDataset(t *testing.T) {
	t.Parallel()
	e := newSrcEnv()
	e.underlying(7, "ABC")
	e.put(7, 11, "90")
	e.open = []contracts.OptionOpen{openRowOf(7, 11, 1, 2.0, 2.1)}
	late := eodRowOf(7, 11, "2026-11-09", 1.0, 1.1, 40)
	late.Generation, late.CaptureID = 9, "zzzz"
	e.eod = []contracts.OptionEod{late}
	e.catalog[7] = "2026-11-09"
	e.run(t, true, entryFor("ABC", "cik:1", "91.5"))
	r := e.rowOf(t, 11)
	if r.BidEod != "2" || r.Ask == nil || *r.Ask != "2.1" {
		t.Fatalf("row %+v", r)
	}
	if in := e.inputOf("ABC"); in.ChainDay != runDay || in.Spot != "100" {
		t.Fatalf("chain day %s spot %s", in.ChainDay, in.Spot)
	}
	if e.openGet != 1 {
		t.Fatalf("open read %d times", e.openGet)
	}
}

func TestSourceKeepsOnlyUSDOpenRows(t *testing.T) {
	t.Parallel()
	e := newSrcEnv()
	e.underlying(7, "ABC")
	e.put(7, 11, "90")
	r := openRowOf(7, 11, 1, 2.0, 2.1)
	r.Currency = "JPY"
	e.open = []contracts.OptionOpen{r}
	e.eod = []contracts.OptionEod{eodRowOf(7, 11, "2026-11-06", 1.5, 1.6, 40)}
	e.run(t, true, entryFor("ABC", "cik:1", "91.5"))
	if in := e.inputOf("ABC"); in.ChainDay != priorDay {
		t.Fatalf("chain day %s", in.ChainDay)
	}
}

func TestSourceDropsDeferredAndNotListed(t *testing.T) {
	t.Parallel()
	e := newSrcEnv()
	e.underlying(7, "ABC")
	for id := int64(11); id <= 16; id++ {
		e.put(7, id, "90")
	}
	e.open = []contracts.OptionOpen{
		openRowOf(7, 11, 1, 2.0, 2.1), openRowOf(7, 12, 7, 0, 0), openRowOf(7, 13, 4, 0, 0),
		openRowOf(7, 14, 3, 0, 0), openRowOf(7, 15, 5, 0, 0), openRowOf(7, 16, 9, 0, 0),
	}
	e.run(t, true, entryFor("ABC", "cik:1", "91.5"))
	if e.hasRow(12) || e.hasRow(13) {
		t.Fatalf("a deferred or not listed row was joined: %d rows", len(e.written))
	}
	ok := map[string]bool{}
	for _, q := range e.inputOf("ABC").Quotes {
		ok[q.InstrumentID] = q.QuoteOK
	}
	if len(ok) != 4 || !ok["11"] || ok["14"] || ok["15"] || ok["16"] {
		t.Fatalf("quotes %v", ok)
	}
}

func TestSourcePartialWithBidAskIsQuote(t *testing.T) {
	t.Parallel()
	e := newSrcEnv()
	e.underlying(7, "ABC")
	e.put(7, 11, "90")
	e.put(7, 12, "89")
	full := openRowOf(7, 11, 2, 2.0, 2.1)
	full.Delta, full.IvDecimal = f64(-0.2), f64(0.3)
	e.open = []contracts.OptionOpen{full, openRowOf(7, 12, 2, 2.0, 0)}
	e.run(t, true, entryFor("ABC", "cik:1", "91.5"))
	got := map[string]compute.OptionQuote{}
	for _, q := range e.inputOf("ABC").Quotes {
		got[q.InstrumentID] = q
	}
	if !got["11"].QuoteOK || got["11"].Delta != "" || got["11"].IV != "" || got["12"].QuoteOK {
		t.Fatalf("quotes %+v", got)
	}
}

func twoUnderlyings(e *srcEnv) []jobs.WatchEntry {
	e.underlying(7, "ABC")
	e.underlying(8, "DEF")
	e.put(7, 11, "90")
	e.put(8, 21, "90")
	return []jobs.WatchEntry{entryFor("ABC", "cik:1", "91.5"), entryFor("DEF", "cik:2", "91.5")}
}

func TestSourceFallsBackPerUnderlying(t *testing.T) {
	t.Parallel()
	e := newSrcEnv()
	entries := twoUnderlyings(e)
	e.open = []contracts.OptionOpen{openRowOf(7, 11, 1, 2.0, 2.1), openRowOf(8, 21, 3, 0, 0)}
	e.eod = []contracts.OptionEod{eodRowOf(7, 11, "2026-11-06", 1.5, 1.6, 40), eodRowOf(8, 21, "2026-11-06", 1.7, 1.8, 40)}
	e.run(t, true, entries...)
	if in := e.inputOf("ABC"); in.ChainDay != runDay {
		t.Fatalf("ABC chain day %s", in.ChainDay)
	}
	if in := e.inputOf("DEF"); in.ChainDay != priorDay {
		t.Fatalf("DEF chain day %s", in.ChainDay)
	}
	if a, d := e.rowOf(t, 11), e.rowOf(t, 21); a.BidEod != "2" || d.BidEod != "1.7" {
		t.Fatalf("bids %s and %s", a.BidEod, d.BidEod)
	}
}

func TestSourceBuyPriceBandDecidesUsable(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, strike string
		chainDay     contracts.Day
	}{
		{"at the band", "96.075", runDay}, // 1.05 times 91.5
		{"above the band", "96.1", priorDay},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e := newSrcEnv()
			e.underlying(7, "ABC")
			e.put(7, 11, c.strike)
			e.open = []contracts.OptionOpen{openRowOf(7, 11, 1, 2.0, 2.1)}
			e.eod = []contracts.OptionEod{eodRowOf(7, 11, "2026-11-06", 1.5, 1.6, 40)}
			e.run(t, true, entryFor("ABC", "cik:1", "91.5"))
			if in := e.inputOf("ABC"); in.ChainDay != c.chainDay {
				t.Fatalf("chain day %s, want %s", in.ChainDay, c.chainDay)
			}
		})
	}
	t.Run("no buy price", func(t *testing.T) {
		t.Parallel()
		e := newSrcEnv()
		e.underlying(7, "ABC")
		e.put(7, 11, "500")
		e.open = []contracts.OptionOpen{openRowOf(7, 11, 1, 2.0, 2.1)}
		e.run(t, true, entryFor("ABC", "cik:1", ""))
		if in := e.inputOf("ABC"); in.ChainDay != runDay {
			t.Fatalf("chain day %s", in.ChainDay)
		}
	})
}

func TestOpenReadErrorFallsBackAndWarns(t *testing.T) {
	t.Parallel()
	e := newSrcEnv()
	entries := twoUnderlyings(e)
	e.openErr = context.DeadlineExceeded
	e.eod = []contracts.OptionEod{eodRowOf(7, 11, "2026-11-06", 1.5, 1.6, 40), eodRowOf(8, 21, "2026-11-06", 1.7, 1.8, 40)}
	j, lb := e.run(t, true, entries...)
	log := lb.String()
	if strings.Count(log, "level=WARN") != 1 || !strings.Contains(log, "component=jobs") || !strings.Contains(log, "status=degraded") {
		t.Fatalf("log %s", log)
	}
	if len(e.written) != 2 || !strings.Contains(e.written[0].Marks, "eod_fallback") || len(j.(skipper).Skipped()) != 2 {
		t.Fatalf("rows %+v skipped %v", e.written, j.(skipper).Skipped())
	}
}

func TestSourceOpenRowWithoutTermsIsDropped(t *testing.T) {
	t.Parallel()
	e := newSrcEnv()
	e.underlying(7, "ABC")
	e.put(7, 11, "90") // instrument 12 has no terms
	e.open = []contracts.OptionOpen{openRowOf(7, 11, 1, 2.0, 2.1), openRowOf(7, 12, 1, 2.0, 2.1)}
	_, lb := e.run(t, true, entryFor("ABC", "cik:1", "91.5"))
	if e.hasRow(12) || !e.hasRow(11) || e.inputOf("ABC").ChainDay != runDay {
		t.Fatalf("rows %+v", e.written)
	}
	if !strings.Contains(lb.String(), "open_rows_without_terms=1") {
		t.Fatalf("log %s", lb.String())
	}
}

func TestSourceNoTermsAtAllFallsBack(t *testing.T) {
	t.Parallel()
	e := newSrcEnv()
	e.underlying(7, "ABC")
	e.put(7, 11, "90")
	e.open = []contracts.OptionOpen{openRowOf(7, 99, 1, 2.0, 2.1)}
	e.eod = []contracts.OptionEod{eodRowOf(7, 11, "2026-11-06", 1.5, 1.6, 40)}
	e.run(t, true, entryFor("ABC", "cik:1", "91.5"))
	if e.inputOf("ABC").ChainDay != priorDay || !e.hasRow(11) {
		t.Fatalf("rows %+v", e.written)
	}
}

func TestFallbackRowsCarryMarkAndSkippedLine(t *testing.T) {
	t.Parallel()
	e := newSrcEnv()
	entries := twoUnderlyings(e)
	entries = append(entries, entryFor("GHI", "cik:3", "91.5")) // no chain at all
	e.open = []contracts.OptionOpen{openRowOf(7, 11, 1, 2.0, 2.1)}
	e.eod = []contracts.OptionEod{eodRowOf(8, 21, "2026-11-06", 1.7, 1.8, 40)}
	j, lb := e.run(t, true, entries...)
	if m := e.rowOf(t, 21).Marks; !strings.Contains(m, "eod_fallback") {
		t.Fatalf("DEF marks %s", m)
	}
	if m := e.rowOf(t, 11).Marks; strings.Contains(m, "eod_fallback") {
		t.Fatalf("ABC marks %s", m)
	}
	var fell []string
	for _, s := range j.(skipper).Skipped() {
		if strings.Contains(s, "DEF") && strings.Contains(s, "2026-11-06") {
			fell = append(fell, s)
		}
	}
	if len(fell) != 1 {
		t.Fatalf("skipped %v", j.(skipper).Skipped())
	}
	log := lb.String()
	for _, want := range []string{"open=1", "eod_fallback=1", "none=1", "ABC=1"} {
		if !strings.Contains(log, want) {
			t.Fatalf("log lacks %s: %s", want, log)
		}
	}
}

func TestOpenInterestJoinedFromEod(t *testing.T) {
	t.Parallel()
	e := newSrcEnv()
	e.underlying(7, "ABC")
	e.put(7, 11, "90")
	e.put(7, 12, "89")
	e.open = []contracts.OptionOpen{openRowOf(7, 11, 1, 2.0, 2.1), openRowOf(7, 12, 1, 2.0, 2.1)}
	newer := eodRowOf(7, 11, "2026-11-06", 1, 1, 5) // below min_open_interest 10
	older := eodRowOf(7, 11, "2026-11-05", 1, 1, 400)
	e.eod = []contracts.OptionEod{older, newer, eodRowOf(7, 12, "2026-11-05", 1, 1, 40)}
	e.run(t, true, entryFor("ABC", "cik:1", "91.5"))
	low, fine := e.rowOf(t, 11), e.rowOf(t, 12)
	if low.Status != "blocked" || !strings.Contains(low.Gates, "low_open_interest") || !strings.Contains(low.Marks, "oi_prior_session") {
		t.Fatalf("low row %+v", low)
	}
	if strings.Contains(fine.Gates, "low_open_interest") || !strings.Contains(fine.Marks, "oi_prior_session") {
		t.Fatalf("fine row %+v", fine)
	}
}

func TestNoEodRowGivesNoOpenInterestMark(t *testing.T) {
	t.Parallel()
	e := newSrcEnv()
	e.underlying(7, "ABC")
	e.put(7, 11, "90")
	e.open = []contracts.OptionOpen{openRowOf(7, 11, 1, 2.0, 2.1)}
	e.eod = []contracts.OptionEod{eodRowOf(7, 11, "2026-11-03", 1, 1, 40)} // outside the last 3 sessions
	e.run(t, true, entryFor("ABC", "cik:1", "91.5"))
	r := e.rowOf(t, 11)
	if !strings.Contains(r.Marks, "no_open_interest") || strings.Contains(r.Marks, "oi_prior_session") || strings.Contains(r.Gates, "low_open_interest") {
		t.Fatalf("row %+v", r)
	}
}

func TestCallsJobUsesSameSourceAsPuts(t *testing.T) {
	t.Parallel()
	e := newSrcEnv()
	entries := twoUnderlyings(e)
	e.call(13, "105")
	e.call(23, "105")
	e.open = []contracts.OptionOpen{openRowOf(7, 11, 1, 2.0, 2.1), openRowOf(7, 13, 1, 3.0, 3.1)}
	e.eod = []contracts.OptionEod{
		eodRowOf(8, 21, "2026-11-06", 1.7, 1.8, 40), eodRowOf(8, 23, "2026-11-06", 2.7, 2.8, 40),
	}
	st := e.stores(true)
	j, err := NewWithCalls(st, &fakeDividends{}, e, entries, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), runDay, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	bids := func(symbol string) (map[string]string, contracts.Day) {
		in := e.inputOf(symbol)
		out := map[string]string{}
		for _, q := range in.Quotes {
			out[q.Right] = q.Bid
		}
		return out, in.ChainDay
	}
	if b, d := bids("ABC"); d != runDay || b["P"] != "2" || b["C"] != "3" {
		t.Fatalf("ABC %v on %s", b, d)
	}
	if b, d := bids("DEF"); d != priorDay || b["P"] != "1.7" || b["C"] != "2.7" {
		t.Fatalf("DEF %v on %s", b, d)
	}
	if !strings.Contains(e.rowOf(t, 21).Marks, "eod_fallback") {
		t.Fatalf("DEF row %+v", e.rowOf(t, 21))
	}
}

// TestSourceIgnoresJPYOpenRowsOfTheUSJob: a JPY open row is never read by the US job, so a name with only such
// a row takes the eod chain.
func TestSourceIgnoresJPYOpenRowsOfTheUSJob(t *testing.T) {
	t.Parallel()
	e := newSrcEnv()
	e.underlying(7, "ABC")
	e.put(7, 11, "90")
	r := openRowOf(7, 11, 1, 2.0, 2.1)
	r.Currency = "JPY"
	e.open = []contracts.OptionOpen{r}
	e.eod = []contracts.OptionEod{eodRowOf(7, 11, "2026-11-06", 1.0, 1.1, 40)}
	e.run(t, true, entryFor("ABC", "cik:1", "91.5"))
	if in := e.inputOf("ABC"); in.ChainDay != priorDay {
		t.Fatalf("chain day %s, the JPY open row was read", in.ChainDay)
	}
}
