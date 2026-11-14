package bookrisk

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
	"github.com/andrewrong/optdesk/internal/strategy"
)

const runDay = contracts.Day("2026-11-09") // a Monday

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

func f64(v float64) *float64 { return &v }

func date(s string) types.SerializableDate {
	t, _ := time.Parse(dayLayout, s)
	return types.SerializableDate{Time: t}
}

// world holds every store of one test; a test sets the fields it needs before it builds the job.
type world struct {
	mu        sync.Mutex
	aliases   []string
	ledger    map[string][]store.FlexTradeRow // an alias present with no rows has none
	ledgerErr map[string]error
	positions map[string][]compute.LedgerPosition // what the replay answers, by alias
	cycles    map[string][]compute.LedgerCycle
	capture   map[string][]store.Position // the capture of the Friday before, by alias
	values    map[string][]store.AccountValue
	eod       map[int64][]contracts.OptionEod // by underlying instrument id
	terms     []store.OptionTerms
	marks     map[string][]mark.PositionMark
	cands     []strategyresult.Row
	totals    map[string]map[string]string // alias -> currency -> total of the fake stresser
	excluded  map[string][]strategy.RiskExcluded
	fitStatus string
	books     map[string]strategy.RiskBook // what the stresser saw
	inserted  [][]strategyresult.Row
}

func newWorld() *world {
	return &world{aliases: []string{"mine"}, ledger: map[string][]store.FlexTradeRow{}, ledgerErr: map[string]error{},
		positions: map[string][]compute.LedgerPosition{}, cycles: map[string][]compute.LedgerCycle{},
		capture: map[string][]store.Position{}, values: map[string][]store.AccountValue{},
		eod: map[int64][]contracts.OptionEod{}, marks: map[string][]mark.PositionMark{},
		totals: map[string]map[string]string{}, excluded: map[string][]strategy.RiskExcluded{}, fitStatus: "pass",
		books: map[string]strategy.RiskBook{}}
}

// addPut gives an alias a ledger row and one open short put on under.
func (w *world) addPut(alias, under, currency string) {
	w.ledger[alias] = []store.FlexTradeRow{{TransactionID: "t1", Revision: 1, Attributes: `{}`}}
	w.positions[alias] = append(w.positions[alias], compute.LedgerPosition{PositionID: "p-" + under, Alias: alias,
		Underlying: under, Status: "open", Right: "P", Expiry: "2026-12-18", Strike: "50", Quantity: "-1", Multiplier: "100",
		Currency: currency})
	w.capture[alias] = append(w.capture[alias], store.Position{Symbol: under, SecType: "OPTION", Currency: currency,
		Right: "put", Strike: "50", Expiry: "2026-12-18", Quantity: "-1"})
}

func (w *world) addValues(alias, currency string) {
	at := time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC)
	for tag, v := range map[string]string{"NetLiquidation": "100000", "AvailableFunds": "50000", "ExcessLiquidity": "40000"} {
		w.values[alias] = append(w.values[alias], store.AccountValue{CaptureID: "c1", Alias: alias, CapturedAt: at, Tag: tag, Value: v, Currency: currency})
	}
}

type ledgerFake struct{ w *world }

func (l ledgerFake) FlexTradeRows(_ context.Context, alias string) ([]store.FlexTradeRow, error) {
	if err := l.w.ledgerErr[alias]; err != nil {
		return nil, err
	}
	return l.w.ledger[alias], nil
}
func (ledgerFake) LatestOkFetch(context.Context, string, string) (store.FlexFetchRecord, bool, error) {
	return store.FlexFetchRecord{}, false, nil
}
func (ledgerFake) StoredAliases(context.Context) ([]string, error) { return nil, nil }

type accountFake struct{ w *world }

func (a accountFake) Positions(_ context.Context, alias, day string) ([]store.Position, error) {
	if day != "2026-11-06" { // the capture is of the Friday before the run day
		return nil, nil
	}
	return a.w.capture[alias], nil
}
func (a accountFake) Values(_ context.Context, alias, day string) ([]store.AccountValue, error) {
	if day != "2026-11-06" {
		return nil, nil
	}
	return a.w.values[alias], nil
}
func (accountFake) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return nil, nil
}

type sessionsFake struct{}

func (sessionsFake) SessionDays(_ context.Context, _ string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	f, _ := time.Parse(dayLayout, string(from))
	e, _ := time.Parse(dayLayout, string(to))
	var out []contracts.SessionDay
	for d := f; !d.After(e); d = d.AddDate(0, 0, 1) {
		out = append(out, contracts.SessionDay{CalendarID: "XNYS", SessionDay: types.SerializableDate{Time: d},
			IsSession: d.Weekday() != time.Saturday && d.Weekday() != time.Sunday})
	}
	return out, nil
}

type replayFake struct{ w *world }

func (r replayFake) Replay(in compute.LedgerInput, _ compute.LedgerParams) compute.LedgerResult {
	return compute.LedgerResult{Positions: r.w.positions[in.Alias], Cycles: r.w.cycles[in.Alias]}
}

// structFake makes one structure of one leg per position, marked at the mid of its quote when it has one, and
// compares the held rows with the positions by underlying.
type structFake struct{}

func (structFake) Evaluate(in compute.StructureInput, _ compute.StructureParams) compute.StructureResult {
	var res compute.StructureResult
	open := map[string]bool{}
	for _, p := range in.Positions {
		if p.Status != "open" {
			continue
		}
		open[p.Underlying] = true
		l := compute.StructureLeg{PositionID: p.PositionID, Right: p.Right, Strike: p.Strike, Expiry: p.Expiry,
			Quantity: p.Quantity, Multiplier: p.Multiplier}
		for _, q := range in.Quotes {
			if q.Underlying == p.Underlying && q.Right == p.Right && q.Expiry == p.Expiry {
				b, _ := strconv.ParseFloat(q.Bid, 64)
				a, _ := strconv.ParseFloat(q.Ask, 64)
				l.Mid = strconv.FormatFloat((a+b)/2, 'f', -1, 64)
			}
		}
		res.Structures = append(res.Structures, compute.Structure{ID: p.PositionID, Alias: in.Alias, Underlying: p.Underlying,
			Currency: p.Currency, Kind: "short_put", Legs: []compute.StructureLeg{l}})
	}
	for _, h := range in.Held {
		if !open[h.Underlying] {
			res.Diff.CaptureOnly = append(res.Diff.CaptureOnly, h.Underlying+" "+h.Expiry+" "+h.Strike+" "+h.Right)
		}
	}
	return res
}

type eodFake struct{ w *world }

func (e eodFake) OptionEod(_ context.Context, id int64, _, _ contracts.Day) ([]contracts.OptionEod, error) {
	return e.w.eod[id], nil
}

type termsFake struct{ w *world }

func (t termsFake) OptionTerms(_ context.Context, ids []int64) ([]store.OptionTerms, error) {
	var out []store.OptionTerms
	for _, x := range t.w.terms {
		if slices.Contains(ids, x.InstrumentID) {
			out = append(out, x)
		}
	}
	return out, nil
}

type marksFake struct{ w *world }

func (m marksFake) Marks(_ context.Context, alias, _ string) ([]mark.PositionMark, error) {
	return m.w.marks[alias], nil
}

type resultsFake struct{ w *world }

func (r resultsFake) InsertResults(_ context.Context, rows []strategyresult.Row) (int, error) {
	r.w.mu.Lock()
	defer r.w.mu.Unlock()
	r.w.inserted = append(r.w.inserted, append([]strategyresult.Row(nil), rows...))
	return len(rows), nil
}
func (r resultsFake) Results(context.Context, contracts.Day, string) ([]strategyresult.Row, error) {
	return r.w.cands, nil
}

// stressFake answers the totals the test set per alias and currency, the exclusions it set, and no_capture for a
// book without a capture, as the real stresser does.
type stressFake struct{ w *world }

func (s stressFake) Stress(b strategy.RiskBook) strategy.RiskOutcome {
	s.w.mu.Lock()
	s.w.books[b.Alias] = b
	s.w.mu.Unlock()
	out := strategy.RiskOutcome{Alias: b.Alias, Excluded: append([]strategy.RiskExcluded(nil), s.w.excluded[b.Alias]...)}
	if !b.HasCapture {
		out.Excluded = append(out.Excluded, strategy.RiskExcluded{Currency: b.Currency, Reason: "no_capture"})
	}
	for cur, v := range s.w.totals[b.Alias] {
		out.Totals = append(out.Totals, strategy.RiskTotal{Currency: cur, WorstLoss: v})
		out.Classes = append(out.Classes, strategy.RiskClassResult{Class: "equity", Currency: cur,
			Worst: strategy.RiskPoint{Move: "-0.15", PnL: "-" + v, AssignmentCash: "5000"}, OldestSpotDay: "2026-11-06"})
	}
	return out
}

type fitFake struct{ w *world }

func (f fitFake) Fit(b strategy.RiskBook, _ strategy.RiskOutcome, c strategy.RiskCandidate) strategy.RiskFit {
	facts := map[string]string{"capital": "5000", "alias": b.Alias, "legs": strconv.Itoa(len(c.Legs))}
	return strategy.RiskFit{Check: strategy.Check{Name: "fit_capital", Status: f.w.fitStatus}, Facts: facts}
}

func (w *world) stores() Stores {
	return Stores{Aliases: w.aliases, Account: accountFake{w}, Ledger: ledgerFake{w}, SessionDays: sessionsFake{},
		Ledgers: func() compute.OptionLedger { return replayFake{w} }, Structures: func() compute.OptionStructures { return structFake{} },
		OptionEod: eodFake{w}, Terms: termsFake{w}, Marks: marksFake{w}, Results: resultsFake{w},
		Stresser: stressFake{w}, Fitter: fitFake{w}}
}

func (w *world) newJob(t *testing.T) *job {
	t.Helper()
	j, err := New(w.stores(), fixedClock{time.Date(2026, 11, 9, 21, 45, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	return j.(*job)
}

// run runs the job once and returns the rows of its insert by key.
func (w *world) run(t *testing.T) map[string]strategyresult.Row {
	t.Helper()
	res := w.newJob(t).Run(context.Background(), runDay, nil)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	return byKey(w.lastInsert())
}

func (w *world) lastInsert() []strategyresult.Row {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.inserted) == 0 {
		return nil
	}
	return w.inserted[len(w.inserted)-1]
}

func byKey(rows []strategyresult.Row) map[string]strategyresult.Row {
	m := map[string]strategyresult.Row{}
	for _, r := range rows {
		m[r.Strategy+"/"+r.Key] = r
	}
	return m
}

func keysOf(m map[string]strategyresult.Row) []string {
	var k []string
	for x := range m {
		k = append(k, x)
	}
	slices.Sort(k)
	return k
}

func TestBookRiskNewWritesOneRowPerAliasAndHousehold(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.aliases = []string{"mine", "wife"}
	w.addPut("mine", "AAA", "USD")
	w.addPut("wife", "BBB", "USD")
	w.totals["mine"] = map[string]string{"USD": "100.00"}
	w.totals["wife"] = map[string]string{"USD": "250.50"}
	rows := w.run(t)
	got := strings.Join(keysOf(rows), ",")
	want := "book_risk/household:USD,book_risk/mine:USD,book_risk/wife:USD"
	if got != want {
		t.Fatalf("rows = %s, want %s", got, want)
	}
	for _, r := range rows {
		if r.Legs != "[]" || r.Checks != "[]" || !strings.HasPrefix(r.Facts, "{") {
			t.Fatalf("row %s: legs %q checks %q facts %q", r.Key, r.Legs, r.Checks, r.Facts)
		}
	}
}

func TestBookRiskNewHouseholdIsSumOfCompleteAliasTotalsPerCurrency(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.aliases = []string{"mine", "wife"}
	w.addPut("mine", "AAA", "USD")
	w.addPut("mine", "JJJ", "JPY")
	w.addPut("wife", "BBB", "USD")
	w.totals["mine"] = map[string]string{"USD": "100.00", "JPY": "5000"}
	w.totals["wife"] = map[string]string{"USD": "250.50"}
	rows := w.run(t)
	if f := factsOf(t, rows["book_risk/household:USD"]); f["worst_loss_USD"] != "350.50" {
		t.Fatalf("household USD = %v", f)
	}
	if f := factsOf(t, rows["book_risk/household:JPY"]); f["worst_loss_JPY"] != "5000.00" {
		t.Fatalf("household JPY = %v", f)
	}
}

func TestBookRiskNewIncompleteAliasGivesHouseholdNoTotal(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.aliases = []string{"mine", "wife"}
	w.addPut("mine", "AAA", "USD")
	w.addPut("wife", "BBB", "USD")
	w.totals["mine"] = map[string]string{"USD": "100.00"}
	w.totals["wife"] = map[string]string{"USD": "250.50"}
	w.excluded["wife"] = []strategy.RiskExcluded{{Underlying: "BBB", Currency: "USD", Reason: "no_vol"}}
	rows := w.run(t)
	h := rows["book_risk/household:USD"]
	if h.Status != "unknown" {
		t.Fatalf("household status = %s", h.Status)
	}
	f := factsOf(t, h)
	if _, has := f["worst_loss_USD"]; has {
		t.Fatalf("an incomplete household carries a total: %v", f)
	}
	if f["incomplete_aliases"] != `["wife"]` {
		t.Fatalf("incomplete_aliases = %q", f["incomplete_aliases"])
	}
	if rows["book_risk/mine:USD"].Status != "ok" || rows["book_risk/wife:USD"].Status != "unknown" {
		t.Fatalf("alias rows: %s %s", rows["book_risk/mine:USD"].Status, rows["book_risk/wife:USD"].Status)
	}
}

func TestBookRiskNewAliasWithoutCaptureIsNoCapture(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.aliases = []string{"mine", "daughter"}
	w.addPut("mine", "AAA", "USD")
	w.addPut("daughter", "BBB", "USD")
	w.capture["daughter"] = nil
	w.totals["mine"] = map[string]string{"USD": "100.00"}
	w.totals["daughter"] = map[string]string{"USD": "10.00"}
	rows := w.run(t)
	if !w.books["mine"].HasCapture || w.books["daughter"].HasCapture {
		t.Fatalf("HasCapture: mine %v daughter %v", w.books["mine"].HasCapture, w.books["daughter"].HasCapture)
	}
	r := rows["book_risk/daughter:USD"]
	f := factsOf(t, r)
	if r.Status != "unknown" || !strings.Contains(f["excluded"], "no_capture") {
		t.Fatalf("daughter row: %s %v", r.Status, f)
	}
	if _, has := f["worst_loss_USD"]; has {
		t.Fatal("a no_capture row carries a total")
	}
}

func TestBookRiskNewBookRowHasUnderlyingIdZeroAndStatusByCompleteness(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.aliases = []string{"mine", "wife"}
	w.addPut("mine", "AAA", "USD")
	w.addPut("wife", "BBB", "USD")
	w.totals["mine"] = map[string]string{"USD": "100.00"}
	w.excluded["wife"] = []strategy.RiskExcluded{{Underlying: "BBB", Currency: "USD", Reason: "stale_mark"}}
	rows := w.run(t)
	for key, r := range rows {
		if r.UnderlyingID != 0 || r.Strategy != "book_risk" || r.Status == "blocked" {
			t.Fatalf("%s: underlying %d strategy %s status %s", key, r.UnderlyingID, r.Strategy, r.Status)
		}
	}
	if rows["book_risk/mine:USD"].Status != "ok" || rows["book_risk/wife:USD"].Status != "unknown" {
		t.Fatal("status does not follow completeness")
	}
}

func candidate(key string, status string, underlyingID int) strategyresult.Row {
	return strategyresult.Row{SessionDay: runDay, RunID: "cand", Strategy: "short_put", Key: key, UnderlyingID: underlyingID,
		Status: status, Legs: `[{"instrument_id":"` + key + `","side":"short","right":"P","strike":"50","expiry":"2026-12-18","multiplier":"100","quantity":1}]`,
		Checks: "[]", Facts: `{"issuer":"AAA","iv":"0.3","spot":"55"}`}
}

func TestBookRiskNewFitRowsOnlyForOkCandidates(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.addPut("mine", "AAA", "USD")
	w.addValues("mine", "USD")
	w.totals["mine"] = map[string]string{"USD": "100.00"}
	w.cands = []strategyresult.Row{candidate("11", "ok", 7), candidate("12", "blocked", 7), candidate("13", "unknown", 7)}
	w.fitStatus = "block"
	rows := w.run(t)
	fits := 0
	for _, r := range rows {
		if r.Strategy == "fit" {
			fits++
		}
	}
	r, ok := rows["fit/11:mine"]
	if fits != 1 || !ok {
		t.Fatalf("fit rows = %d, keys %v", fits, keysOf(rows))
	}
	if r.UnderlyingID != 7 || r.Status != "blocked" || !strings.Contains(r.Checks, `"fit_capital"`) || r.Rank != nil {
		t.Fatalf("fit row = %+v", r)
	}
	if f := factsOf(t, r); f["capital"] != "5000" || f["legs"] != "1" {
		t.Fatalf("facts = %v", f)
	}
	w.fitStatus = "pass"
	if got := w.run(t)["fit/11:mine"].Status; got != "ok" {
		t.Fatalf("pass gives %s", got)
	}
	w.fitStatus = "unknown"
	if got := w.run(t)["fit/11:mine"].Status; got != "unknown" {
		t.Fatalf("unknown gives %s", got)
	}
}

func TestBookRiskNewNoCandidateRowsGivesOnlyBookRows(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.addPut("mine", "AAA", "USD")
	w.addValues("mine", "USD")
	w.totals["mine"] = map[string]string{"USD": "100.00"}
	for k, r := range w.run(t) {
		if r.Strategy != "book_risk" {
			t.Fatalf("unexpected row %s", k)
		}
	}
}

func TestBookRiskNewAliasWithoutValuesGetsNoFitRow(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.aliases = []string{"mine", "daughter"}
	w.addPut("mine", "AAA", "USD")
	w.addPut("daughter", "BBB", "USD")
	w.addValues("mine", "USD")
	w.totals["mine"] = map[string]string{"USD": "100.00"}
	w.cands = []strategyresult.Row{candidate("11", "ok", 7)}
	rows := w.run(t)
	if _, has := rows["fit/11:daughter"]; has {
		t.Fatal("an alias with no account.value got a fit row")
	}
	if _, has := rows["fit/11:mine"]; !has {
		t.Fatal("the alias with funds got no fit row")
	}
	if w.books["daughter"].Funds != nil || w.books["mine"].Funds == nil || w.books["mine"].Funds.Currency != "USD" {
		t.Fatalf("funds: %+v / %+v", w.books["mine"].Funds, w.books["daughter"].Funds)
	}
}

func TestBookRiskNewOneAliasReadFailureKeepsTheOthers(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.aliases = []string{"mine", "wife"}
	w.addPut("mine", "AAA", "USD")
	w.addPut("wife", "BBB", "USD")
	w.totals["mine"] = map[string]string{"USD": "100.00"}
	w.ledgerErr["wife"] = errors.New("boom")
	rows := w.run(t)
	if rows["book_risk/mine:USD"].Status != "ok" {
		t.Fatalf("mine = %s", rows["book_risk/mine:USD"].Status)
	}
	f := rows["book_risk/wife:USD"]
	if f.Status != "unknown" || factsOf(t, f)["reason"] != "read_failed" {
		t.Fatalf("wife row = %+v", f)
	}
	if rows["book_risk/household:USD"].Status != "unknown" {
		t.Fatal("household is complete although one alias was not read")
	}
}

func TestBookRiskNewRerunWritesANewRunId(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.addPut("mine", "AAA", "USD")
	w.addValues("mine", "USD")
	w.totals["mine"] = map[string]string{"USD": "100.00"}
	w.cands = []strategyresult.Row{candidate("11", "ok", 7)}
	j := w.newJob(t)
	for range 2 {
		if res := j.Run(context.Background(), runDay, nil); res.Err != nil {
			t.Fatal(res.Err)
		}
	}
	ids := map[string]bool{}
	for _, batch := range w.inserted {
		one := map[string]bool{}
		for _, r := range batch {
			one[r.RunID] = true
		}
		if len(one) != 1 {
			t.Fatalf("a run wrote %d run ids", len(one))
		}
		for id := range one {
			ids[id] = true
		}
	}
	if len(w.inserted) != 2 || len(ids) != 2 {
		t.Fatalf("runs %d, run ids %d", len(w.inserted), len(ids))
	}
}

func factsOf(t *testing.T, r strategyresult.Row) map[string]string {
	t.Helper()
	m := map[string]string{}
	if err := jsonUnmarshal(r.Facts, &m); err != nil {
		t.Fatalf("facts of %s: %v", r.Key, err)
	}
	return m
}

func TestBookRiskNewManifestIsTheScheduleOfThePlan(t *testing.T) {
	t.Parallel()
	j := newWorld().newJob(t)
	cal, anchor, zone, off := j.Schedule()
	if j.ID() != "book_risk" || cal != "XNYS" || anchor != "session_close" || zone != "America/New_York" ||
		off != 45*time.Minute || j.RunBound() != 10*time.Minute || j.RestartSensitive() {
		t.Fatalf("schedule = %s %s %s %s %s", j.ID(), cal, anchor, zone, off)
	}
	if j.classOf("SPX") != "broad_high" || j.classOf("RUT") != "broad_other" || j.classOf("AAA") != "equity" {
		t.Fatal("class table")
	}
}

func TestBookRiskNewAliasWithoutFundsCarriesNoFundsReason(t *testing.T) {
	t.Parallel()
	w := newWorld()
	w.aliases = []string{"mine"}
	w.addPut("mine", "AAA", "USD")
	w.totals["mine"] = map[string]string{"USD": "100.00"}
	f := factsOf(t, w.run(t)["book_risk/mine:USD"])
	if f["reason"] != "no_funds" && !strings.Contains(f["excluded"], "no_funds") {
		t.Fatalf("no no_funds reason: %v", f)
	}
	w.addValues("mine", "USD")
	if f := factsOf(t, w.run(t)["book_risk/mine:USD"]); f["reason"] != "" {
		t.Fatalf("an alias with funds carries reason %q", f["reason"])
	}
}
