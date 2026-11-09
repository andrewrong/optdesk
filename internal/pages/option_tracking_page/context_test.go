package option_tracking_page

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/atombender/go-jsonschema/pkg/types"
)

const buildDay = contracts.Day("2026-11-10")

type fakeLedger struct {
	rows []store.FlexTradeRow
	err  error
}

func (f fakeLedger) FlexTradeRows(context.Context, string) ([]store.FlexTradeRow, error) {
	return f.rows, f.err
}
func (fakeLedger) LatestOkFetch(context.Context, string, string) (store.FlexFetchRecord, bool, error) {
	return store.FlexFetchRecord{}, false, nil
}
func (fakeLedger) StoredAliases(context.Context) ([]string, error) { return nil, nil }

type fakeReplay struct {
	mu  sync.Mutex
	in  compute.LedgerInput
	out compute.LedgerResult
}

func (f *fakeReplay) Replay(in compute.LedgerInput, _ compute.LedgerParams) compute.LedgerResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.in = in
	return f.out
}

type fakeCatalog struct{ ids []int64 }

func (f fakeCatalog) OptionEodLatest(context.Context, contracts.Day) ([]store.OptionEodLatest, error) {
	out := make([]store.OptionEodLatest, 0, len(f.ids))
	for _, id := range f.ids {
		out = append(out, store.OptionEodLatest{UnderlyingID: id})
	}
	return out, nil
}

type fakeNames struct {
	names map[int64]string
	sizes *[]int
	mu    *sync.Mutex
}

func (f fakeNames) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	f.mu.Lock()
	*f.sizes = append(*f.sizes, len(ids))
	f.mu.Unlock()
	var out []store.InstrumentName
	for _, id := range ids {
		if n, ok := f.names[id]; ok {
			out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: n})
		}
	}
	return out, nil
}

type fakeEod struct {
	rows map[int64][]contracts.OptionEod
}

func (f fakeEod) OptionEod(_ context.Context, id int64, _, _ contracts.Day) ([]contracts.OptionEod, error) {
	return f.rows[id], nil
}

type fakeTerms struct {
	terms map[int64]store.OptionTerms
	calls *[]int
	mu    *sync.Mutex
}

func (f fakeTerms) OptionTerms(_ context.Context, ids []int64) ([]store.OptionTerms, error) {
	f.mu.Lock()
	*f.calls = append(*f.calls, len(ids))
	f.mu.Unlock()
	var out []store.OptionTerms
	for _, id := range ids {
		if t, ok := f.terms[id]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

type fakeSessions struct{}

func (fakeSessions) SessionDays(_ context.Context, _ string, _, _ contracts.Day) ([]contracts.SessionDay, error) {
	mk := func(s string, is bool) contracts.SessionDay {
		d, _ := time.Parse("2006-01-02", s)
		return contracts.SessionDay{SessionDay: types.SerializableDate{Time: d}, IsSession: is}
	}
	return []contracts.SessionDay{mk("2026-11-06", true), mk("2026-11-07", false), mk("2026-11-09", true)}, nil
}

type fakeCandidates struct {
	byKey map[string][]store.Candidate // "day|kind"
	asked *[]string
	mu    *sync.Mutex
	err   error
}

func (f fakeCandidates) Candidates(_ context.Context, day contracts.Day, kind string) ([]store.Candidate, error) {
	f.mu.Lock()
	*f.asked = append(*f.asked, string(day)+"|"+kind)
	f.mu.Unlock()
	return f.byKey[string(day)+"|"+kind], f.err
}

type fakeAccount struct {
	byDay map[string][]store.Position
	asked *[]string
	mu    *sync.Mutex
}

func (f fakeAccount) Positions(_ context.Context, _, day string) ([]store.Position, error) {
	f.mu.Lock()
	*f.asked = append(*f.asked, day)
	f.mu.Unlock()
	return f.byDay[day], nil
}
func (fakeAccount) Values(context.Context, string, string) ([]store.AccountValue, error) {
	return nil, nil
}
func (fakeAccount) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return nil, nil
}

// rig holds a Config of fakes and what they recorded.
type rig struct {
	replay     *fakeReplay
	nameSizes  []int
	termCalls  []int
	candAsked  []string
	acctAsked  []string
	terms      map[int64]store.OptionTerms
	eod        map[int64][]contracts.OptionEod
	candidates map[string][]store.Candidate
	positions  map[string][]store.Position
	names      map[int64]string
	catalogIDs []int64
}

func newRig() *rig {
	r := &rig{replay: &fakeReplay{}, terms: map[int64]store.OptionTerms{}, eod: map[int64][]contracts.OptionEod{},
		candidates: map[string][]store.Candidate{}, positions: map[string][]store.Position{}, names: map[int64]string{}}
	r.replay.out = compute.LedgerResult{Positions: []compute.LedgerPosition{
		{PositionID: "p1", Underlying: "AAPL", Status: "open", Right: "P", Expiry: "2026-12-18", Strike: "150"}}}
	r.names[1], r.catalogIDs = "AAPL", []int64{1}
	return r
}

func (r *rig) config() Config {
	mu := &sync.Mutex{}
	return Config{
		Ledger:     fakeLedger{rows: []store.FlexTradeRow{{TransactionID: "t1", Revision: 2, Attributes: `{"symbol":"AAPL"}`}}},
		Replay:     r.replay,
		Catalog:    fakeCatalog{ids: r.catalogIDs},
		Names:      fakeNames{names: r.names, sizes: &r.nameSizes, mu: mu},
		Eod:        fakeEod{rows: r.eod},
		Terms:      fakeTerms{terms: r.terms, calls: &r.termCalls, mu: mu},
		Sessions:   fakeSessions{},
		Candidates: fakeCandidates{byKey: r.candidates, asked: &r.candAsked, mu: mu},
		Account:    fakeAccount{byDay: r.positions, asked: &r.acctAsked, mu: mu},
		CalendarID: "XNYS",
	}
}

func fp(v float64) *float64 { return &v }

func eodRow(id int, day string, bid, ask float64) contracts.OptionEod {
	d, _ := time.Parse("2006-01-02", day)
	return contracts.OptionEod{InstrumentID: id, SessionDay: types.SerializableDate{Time: d}, Bid: fp(bid), Ask: fp(ask)}
}

func put(id int64, strike string) store.OptionTerms {
	return store.OptionTerms{InstrumentID: id, Right: "P", Strike: strike, ExpirationDate: "2026-12-18"}
}

func TestGatherReplaysFlexRows(t *testing.T) {
	t.Parallel()
	r := newRig()
	in, gs, err := gather(context.Background(), r.config(), "main", buildDay)
	if err != nil || len(gs) != 0 {
		t.Fatalf("err = %v gaps = %v", err, gs)
	}
	want := []compute.LedgerTrade{{TransactionID: "t1", Revision: 2, Attributes: map[string]string{"symbol": "AAPL"}}}
	if !reflect.DeepEqual(r.replay.in.Trades, want) {
		t.Fatalf("trades = %+v", r.replay.in.Trades)
	}
	if r.replay.in.Alias != "main" || r.replay.in.AsOf != buildDay {
		t.Fatalf("replay input = %+v", r.replay.in)
	}
	if len(in.Positions) != 1 || in.Positions[0].PositionID != "p1" || in.Alias != "main" || in.AsOf != buildDay {
		t.Fatalf("input = %+v", in)
	}
	if !reflect.DeepEqual(in.SessionDays, []contracts.Day{"2026-11-06", "2026-11-09"}) {
		t.Fatalf("session days = %v", in.SessionDays)
	}
}

func TestNoMarksPassedToReplay(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.terms[10] = put(10, "150")
	r.eod[1] = []contracts.OptionEod{eodRow(10, "2026-11-09", 1, 2)}
	in, _, err := gather(context.Background(), r.config(), "main", buildDay)
	if err != nil || len(in.Quotes) != 1 {
		t.Fatalf("err = %v quotes = %v", err, in.Quotes)
	}
	if len(r.replay.in.Marks) != 0 {
		t.Fatalf("marks = %+v", r.replay.in.Marks)
	}
}

func TestUnderlyingFoundByName(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.names = map[int64]string{1: "MSFT", 2: "AAPL"}
	r.catalogIDs = []int64{1, 2}
	r.terms[10], r.terms[20] = put(10, "150"), put(20, "150")
	r.eod[1] = []contracts.OptionEod{eodRow(10, "2026-11-09", 9, 9.5)}
	r.eod[2] = []contracts.OptionEod{eodRow(20, "2026-11-09", 1, 2)}
	in, _, err := gather(context.Background(), r.config(), "main", buildDay)
	if err != nil || len(in.Quotes) != 1 {
		t.Fatalf("err = %v quotes = %v", err, in.Quotes)
	}
	if q := in.Quotes[0]; q.Underlying != "AAPL" || q.Bid != "1" || q.Ask != "2" {
		t.Fatalf("quote = %+v", q)
	}
}

func TestQuoteMatchedByTerms(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.terms[10], r.terms[11] = put(10, "150.00"), put(11, "155")
	r.eod[1] = []contracts.OptionEod{eodRow(10, "2026-11-06", 1, 2), eodRow(10, "2026-11-09", 1.1, 2.2), eodRow(11, "2026-11-09", 7, 8)}
	in, _, err := gather(context.Background(), r.config(), "main", buildDay)
	if err != nil || len(in.Quotes) != 1 {
		t.Fatalf("err = %v quotes = %+v", err, in.Quotes)
	}
	want := "1.1"
	if q := in.Quotes[0]; q.Strike != "150.00" || q.Right != "P" || q.Expiry != "2026-12-18" || q.Day != "2026-11-09" || q.Bid != want || q.Ask != "2.2" {
		t.Fatalf("quote = %+v", q)
	}
}

func TestChunksOf500(t *testing.T) {
	t.Parallel()
	r := newRig()
	rows := make([]contracts.OptionEod, 0, 501)
	for i := 1; i <= 501; i++ {
		rows = append(rows, eodRow(i, "2026-11-09", 1, 2))
	}
	r.eod[1] = rows
	if _, _, err := gather(context.Background(), r.config(), "main", buildDay); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.termCalls, []int{500, 1}) {
		t.Fatalf("terms calls = %v", r.termCalls)
	}
	ids := make([]int64, 0, 501)
	for i := int64(1); i <= 501; i++ {
		ids = append(ids, i)
		r.names[i] = "X"
	}
	r.catalogIDs = ids
	r.nameSizes = nil
	if _, _, err := gather(context.Background(), r.config(), "main", buildDay); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.nameSizes, []int{500, 1}) {
		t.Fatalf("name calls = %v", r.nameSizes)
	}
}

func TestFloatBecomesDecimalText(t *testing.T) {
	t.Parallel()
	nan, inf := math.NaN(), math.Inf(1)
	cases := []struct {
		in   *float64
		want string
	}{{fp(0.1), "0.1"}, {fp(12), "12"}, {fp(1.5e-7), "0.00000015"}, {&nan, ""}, {&inf, ""}, {nil, ""}}
	for _, c := range cases {
		if got := decimalText(c.in); got != c.want {
			t.Errorf("decimalText = %q, want %q", got, c.want)
		}
	}
}

func cand(kind, day, issuer, alias string) store.Candidate {
	next := contracts.Day("2026-11-20")
	return store.Candidate{Kind: kind, SessionDay: contracts.Day(day), Issuer: issuer, Alias: alias, Expiry: "2026-12-18",
		Strike: "150", Right: "put", NextEarnings: &next, EarningsState: "listed", Status: "block",
		Gates: `[{"status":"pass","reason":""},{"status":"block","reason":"early_assignment_risk"}]`}
}

func TestEventsFromAllKinds(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.candidates["2026-11-10|short_put"] = []store.Candidate{cand("short_put", "2026-11-10", "AAPL", "")}
	r.candidates["2026-11-10|covered_call"] = []store.Candidate{cand("covered_call", "2026-11-10", "AAPL", "main")}
	r.candidates["2026-11-10|held_call_guard"] = []store.Candidate{cand("held_call_guard", "2026-11-10", "AAPL", "main"),
		cand("held_call_guard", "2026-11-10", "MSFT", "other")}
	in, _, err := gather(context.Background(), r.config(), "main", buildDay)
	if err != nil || len(in.Events) != 3 {
		t.Fatalf("err = %v events = %+v", err, in.Events)
	}
	kinds := map[string]trackEventView{}
	for _, e := range in.Events {
		kinds[e.Kind] = trackEventView{e.Underlying, string(e.NextEarnings), e.GuardStatus, e.GuardReason, e.Right}
	}
	want := map[string]trackEventView{
		"short_put":       {"AAPL", "2026-11-20", "", "", "P"},
		"covered_call":    {"AAPL", "2026-11-20", "", "", "P"},
		"held_call_guard": {"AAPL", "2026-11-20", "block", "early_assignment_risk", "P"},
	}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("events = %+v", kinds)
	}
}

type trackEventView struct{ under, earnings, guard, reason, right string }

func TestEventLookbackSevenDays(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.candidates["2026-11-03|short_put"] = []store.Candidate{cand("short_put", "2026-11-03", "AAPL", "")}
	r.candidates["2026-11-02|short_put"] = []store.Candidate{cand("short_put", "2026-11-02", "AAPL", "")}
	in, _, err := gather(context.Background(), r.config(), "main", buildDay)
	if err != nil || len(in.Events) != 1 || in.Events[0].Day != "2026-11-03" {
		t.Fatalf("err = %v events = %+v", err, in.Events)
	}
	if len(r.candAsked) != 8*3 {
		t.Fatalf("asked %d (day, kind) pairs: %v", len(r.candAsked), r.candAsked)
	}
}

func TestHeldFromNewestCapture(t *testing.T) {
	t.Parallel()
	r := newRig()
	opt := store.Position{SecType: "OPT", Symbol: "AAPL", Expiry: "2026-12-18", Strike: "150", Right: "put", Quantity: "-1"}
	stk := store.Position{SecType: "STK", Symbol: "AAPL", Quantity: "100"}
	r.positions["2026-11-08"] = []store.Position{stk, opt}
	r.positions["2026-11-05"] = []store.Position{stk}
	in, _, err := gather(context.Background(), r.config(), "main", buildDay)
	if err != nil || !in.HeldKnown {
		t.Fatalf("err = %v known = %v", err, in.HeldKnown)
	}
	wantHeld := []string{"AAPL|2026-12-18|150|P|-1"}
	var got []string
	for _, h := range in.Held {
		got = append(got, h.Underlying+"|"+string(h.Expiry)+"|"+h.Strike+"|"+h.Right+"|"+h.Quantity)
	}
	if !reflect.DeepEqual(got, wantHeld) {
		t.Fatalf("held = %v", got)
	}
	if !reflect.DeepEqual(r.acctAsked, []string{"2026-11-10", "2026-11-09", "2026-11-08"}) {
		t.Fatalf("asked = %v", r.acctAsked)
	}
	r2 := newRig()
	in, _, err = gather(context.Background(), r2.config(), "main", buildDay)
	if err != nil || in.HeldKnown || len(r2.acctAsked) != 7 {
		t.Fatalf("err = %v known = %v asked = %v", err, in.HeldKnown, r2.acctAsked)
	}
}

func TestNilReadersAreGaps(t *testing.T) {
	t.Parallel()
	_, gs, err := gather(context.Background(), Config{}, "main", buildDay)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := gaps{gapNoReader, gapNoSessions, gapNoEodReaders, gapNoCandidates, gapNoAccount}
	if !reflect.DeepEqual(gs, want) {
		t.Fatalf("gaps = %v", gs)
	}
	r := newRig()
	c := r.config()
	c.Ledger = fakeLedger{}
	_, gs, err = gather(context.Background(), c, "main", buildDay)
	if err != nil || !reflect.DeepEqual(gs, gaps{gapNoFlexRows}) {
		t.Fatalf("err = %v gaps = %v", err, gs)
	}
}

func TestReaderErrorIsReturned(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	r := newRig()
	c := r.config()
	c.Ledger = fakeLedger{err: boom}
	if _, _, err := gather(context.Background(), c, "main", buildDay); !errors.Is(err, boom) {
		t.Fatalf("ledger err = %v", err)
	}
	c = r.config()
	c.Candidates = fakeCandidates{err: boom, asked: &[]string{}, mu: &sync.Mutex{}}
	if _, _, err := gather(context.Background(), c, "main", buildDay); !errors.Is(err, boom) {
		t.Fatalf("candidates err = %v", err)
	}
}
