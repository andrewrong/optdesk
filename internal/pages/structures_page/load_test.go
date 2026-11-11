package structures_page

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var (
	testZone = time.FixedZone("CST", 8*3600)
	testAt   = time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC)
)

const testDay = contracts.Day("2026-11-09")

type stubLedger struct {
	store.OptionLedgerReader
	rows []store.FlexTradeRow
}

func (s stubLedger) FlexTradeRows(context.Context, string) ([]store.FlexTradeRow, error) {
	return s.rows, nil
}

type stubSessions struct {
	store.SessionDayReader
	days []string
}

func (s stubSessions) SessionDays(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SessionDay, error) {
	var out []contracts.SessionDay
	for _, d := range s.days {
		t, _ := time.Parse(dayLayout, d)
		out = append(out, contracts.SessionDay{CalendarID: "XNYS", IsSession: true, SessionDay: types.SerializableDate{Time: t}})
	}
	return out, nil
}

type stubAccount struct {
	store.AccountReader
	byDay map[string][]store.Position
}

func (s stubAccount) Positions(_ context.Context, _, day string) ([]store.Position, error) {
	return s.byDay[day], nil
}

type stubEod struct {
	store.OptionEodReader
	byUnderlying map[int64][]contracts.OptionEod
}

func (s stubEod) OptionEod(_ context.Context, id int64, _, _ contracts.Day) ([]contracts.OptionEod, error) {
	return s.byUnderlying[id], nil
}

type stubTerms struct {
	store.OptionTermsReader
	terms []store.OptionTerms
}

func (s stubTerms) OptionTerms(context.Context, []int64) ([]store.OptionTerms, error) {
	return s.terms, nil
}

type stubReplay struct {
	got *compute.LedgerInput
	res compute.LedgerResult
}

func (s stubReplay) Replay(in compute.LedgerInput, _ compute.LedgerParams) compute.LedgerResult {
	*s.got = in
	return s.res
}

type stubRule struct {
	res    compute.StructureResult
	in     *compute.StructureInput
	params *compute.StructureParams
}

func (s stubRule) Evaluate(in compute.StructureInput, p compute.StructureParams) compute.StructureResult {
	*s.in, *s.params = in, p
	return s.res
}

// rig is a Config over fakes and the inputs the rules received.
type rig struct {
	cfg    Config
	replay compute.LedgerInput
	in     compute.StructureInput
	params compute.StructureParams
}

func newRig(res compute.StructureResult) *rig {
	r := &rig{}
	r.cfg = Config{
		RunID: "run1", BuildID: "build1", Zone: testZone, Aliases: []string{"main", "second"},
		Ledger:   stubLedger{},
		Sessions: stubSessions{days: []string{"2026-11-04", "2026-11-05", "2026-11-06", "2026-11-09"}},
		Account:  stubAccount{}, Eod: stubEod{}, Terms: stubTerms{},
		Ledgers:    func() compute.OptionLedger { return stubReplay{got: &r.replay} },
		Structures: func() compute.OptionStructures { return stubRule{res: res, in: &r.in, params: &r.params} },
	}
	return r
}

func (r *rig) build(t *testing.T, alias string) []byte {
	t.Helper()
	b, err := buildList(context.Background(), r.cfg, alias, testDay, testAt)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func readResult(t *testing.T, name string) compute.StructureResult {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var res compute.StructureResult
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	return res
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestNoLedgerRowsIsNothingHeld(t *testing.T) {
	t.Parallel()
	r := newRig(compute.StructureResult{})
	m := decode(t, r.build(t, "main"))
	for _, g := range m["groups"].([]any) {
		if rows := g.(map[string]any)["rows"].([]any); len(rows) != 0 {
			t.Fatalf("rows = %v", rows)
		}
	}
	if m["sentence"] != "Account main has no structures held" || m["run_status"] == "failed" {
		t.Fatalf("sentence = %v, status = %v", m["sentence"], m["run_status"])
	}
}

func TestLoadFeedsTheRules(t *testing.T) {
	t.Parallel()
	r := newRig(compute.StructureResult{})
	r.cfg.Ledger = stubLedger{rows: []store.FlexTradeRow{{TransactionID: "T1", Revision: 2, Attributes: `{"symbol":"AAPL","quantity":"-1"}`}}}
	r.cfg.Ledgers = func() compute.OptionLedger {
		open := compute.LedgerPosition{PositionID: "main:T1:1", Underlying: "AAPL", Status: "open"}
		return stubReplay{got: &r.replay, res: compute.LedgerResult{Positions: []compute.LedgerPosition{open}}}
	}
	r.cfg.Account = stubAccount{byDay: map[string][]store.Position{"2026-11-06": {
		{SecType: "STK", Symbol: "AAPL", ConID: 77},
		{SecType: "OPT", Symbol: "AAPL", Right: "put", Strike: "150", Expiry: "2026-12-18", Quantity: "-1"},
	}}}
	bid, ask, delta := 1.1, 1.3, -0.25
	r.cfg.Eod = stubEod{byUnderlying: map[int64][]contracts.OptionEod{77: {
		{InstrumentID: 5, SessionDay: types.SerializableDate{Time: time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)}, Bid: &bid, Ask: &ask},
		{InstrumentID: 5, SessionDay: types.SerializableDate{Time: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)}, Bid: &bid, Ask: &ask, Delta: &delta},
	}}}
	r.cfg.Terms = stubTerms{terms: []store.OptionTerms{{InstrumentID: 5, Right: "P", Strike: "150", ExpirationDate: "2026-12-18"}}}
	r.build(t, "main")
	if len(r.replay.Trades) != 1 || r.replay.Trades[0].Attributes["symbol"] != "AAPL" || r.replay.Trades[0].Revision != 2 {
		t.Fatalf("trades = %+v", r.replay.Trades)
	}
	if len(r.replay.SessionDays) != 4 || r.replay.AsOf != testDay {
		t.Fatalf("replay = %+v", r.replay)
	}
	if r.in.HeldDay != "2026-11-06" || len(r.in.Held) != 1 || r.in.Held[0].Right != "P" || r.in.Held[0].Underlying != "AAPL" {
		t.Fatalf("held = %v on %q", r.in.Held, r.in.HeldDay)
	}
	want := compute.StructureQuote{Underlying: "AAPL", Right: "P", Strike: "150", Expiry: "2026-12-18", Day: "2026-11-06",
		Bid: "1.1", Ask: "1.3", Delta: "-0.25"}
	if len(r.in.Quotes) != 1 || r.in.Quotes[0] != want {
		t.Fatalf("quotes = %+v, want %+v", r.in.Quotes, want)
	}
	if r.params.CaptureTarget != "0.50" || r.params.ExitDTE != 21 {
		t.Fatalf("params = %+v", r.params)
	}
}
