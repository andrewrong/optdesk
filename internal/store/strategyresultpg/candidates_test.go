package strategyresultpg

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
)

type fakeResults struct {
	rows  []strategyresult.Row
	calls int
}

func (f *fakeResults) Results(context.Context, contracts.Day, string) ([]strategyresult.Row, error) {
	f.calls++
	return f.rows, nil
}

type fakeOld struct {
	rows  []store.Candidate
	calls int
	kinds []string
}

func (f *fakeOld) Candidates(_ context.Context, _ contracts.Day, kind string) ([]store.Candidate, error) {
	f.calls++
	f.kinds = append(f.kinds, kind)
	return f.rows, nil
}

func sp(s string) *string { return &s }

func fullCandidate() store.Candidate {
	next, ex := contracts.Day("2027-01-28"), contracts.Day("2026-12-01")
	ver := int64(7)
	return store.Candidate{
		SessionDay: "2026-11-13", RunID: "r1", Kind: "short_put", InstrumentID: 42, Alias: "",
		RuleVersion: "put_v1", Issuer: "AAPL", UnderlyingID: 7, Expiry: "2026-12-18", DTE: 35, Right: "put",
		Strike: "180", Multiplier: 100, BidEod: "2.15", Ask: sp("2.2"), Spot: sp("190.5"), BuyPrice: sp("1.9"),
		SalePrice: sp("2.4"), BreakEven: "177.85", AnnualizedYield: sp("0.12"), Delta: sp("-0.2"), Iv: sp("0.31"),
		Status: "ok", Gates: `[{"gate":"spread","status":"pass","reason":""},{"gate":"fit","status":"unknown","reason":"no_account"}]`,
		Marks: `["near_earnings"]`, Fits: `{"A":"fit","B":"no_fit"}`, NextEarnings: &next, EarningsState: "listed",
		NextExDate: &ex, ExAmount: sp("0.24"), Extrinsic: sp("1.1"), Rank: ip(1), RankInUnderlying: ip(1),
		ComputedAt: t0, SettingsVersion: &ver,
	}
}

// pack is the test's own packer: a candidate as a result row, absent figures omitted from Facts.
func pack(c store.Candidate) strategyresult.Row {
	facts := map[string]string{"issuer": c.Issuer, "dte": itoa(c.DTE), "bid": c.BidEod, "break_even": c.BreakEven,
		"marks": c.Marks, "fits": c.Fits, "earnings_state": c.EarningsState}
	for k, v := range map[string]*string{"ask": c.Ask, "spot": c.Spot, "buy_price": c.BuyPrice, "sale_price": c.SalePrice,
		"annualized_yield": c.AnnualizedYield, "delta": c.Delta, "iv": c.Iv, "ex_amount": c.ExAmount, "extrinsic": c.Extrinsic} {
		if v != nil {
			facts[k] = *v
		}
	}
	if c.NextEarnings != nil {
		facts["next_earnings"] = string(*c.NextEarnings)
	}
	if c.NextExDate != nil {
		facts["next_ex_date"] = string(*c.NextExDate)
	}
	leg := []legJSON{{InstrumentID: itoa(c.InstrumentID), Side: "short", Right: "P", Strike: c.Strike, Expiry: string(c.Expiry),
		Multiplier: itoa(c.Multiplier), Quantity: 1}}
	legs, _ := json.Marshal(leg)
	fb, _ := json.Marshal(facts)
	return strategyresult.Row{SessionDay: c.SessionDay, RunID: c.RunID, Strategy: "short_put", RuleVersion: c.RuleVersion,
		ParamHash: "h", Key: itoa(c.InstrumentID), UnderlyingID: c.UnderlyingID, Status: c.Status, Rank: c.Rank,
		RankInUnderlying: c.RankInUnderlying, Legs: string(legs), Checks: gatesToChecks(c.Gates), Facts: string(fb),
		ComputedAt: c.ComputedAt, SettingsVersion: c.SettingsVersion}
}

func gatesToChecks(g string) string { return g } // the same JSON: gate, status, reason

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestCandidateReaderRebuildsEveryColumn(t *testing.T) {
	t.Parallel()
	for _, strip := range []bool{false, true} {
		in := fullCandidate()
		if strip {
			in.Ask, in.Spot, in.BuyPrice, in.SalePrice, in.AnnualizedYield, in.Delta, in.Iv = nil, nil, nil, nil, nil, nil, nil
			in.NextEarnings, in.NextExDate, in.ExAmount, in.Extrinsic = nil, nil, nil, nil
			in.Rank, in.RankInUnderlying, in.SettingsVersion = nil, nil, nil
		}
		rd := NewCandidateReader(&fakeResults{rows: []strategyresult.Row{pack(in)}}, &fakeOld{})
		got, err := rd.Candidates(context.Background(), "2026-11-13", "short_put")
		if err != nil || len(got) != 1 {
			t.Fatalf("strip=%v: %+v, %v", strip, got, err)
		}
		if !reflect.DeepEqual(got[0], in) {
			t.Errorf("strip=%v: got %+v, want %+v", strip, got[0], in)
		}
	}
}

func TestCandidateReaderDelegatesOtherKindsAndPreShadowDays(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	oldRows := []store.Candidate{fullCandidate()}
	for _, kind := range []string{"covered_call", "held_call_guard"} {
		res, old := &fakeResults{rows: []strategyresult.Row{pack(fullCandidate())}}, &fakeOld{rows: oldRows}
		got, err := NewCandidateReader(res, old).Candidates(ctx, "2026-11-13", kind)
		if err != nil || !reflect.DeepEqual(got, oldRows) || res.calls != 0 || !reflect.DeepEqual(old.kinds, []string{kind}) {
			t.Errorf("%s: got %+v, %v, result calls %d, old kinds %v", kind, got, err, res.calls, old.kinds)
		}
	}
	res, old := &fakeResults{rows: []strategyresult.Row{}}, &fakeOld{rows: oldRows}
	got, err := NewCandidateReader(res, old).Candidates(ctx, "2026-11-06", "short_put")
	if err != nil || !reflect.DeepEqual(got, oldRows) || old.calls != 1 {
		t.Errorf("pre-shadow day: got %+v, %v, old calls %d", got, err, old.calls)
	}
	res, old = &fakeResults{rows: []strategyresult.Row{pack(fullCandidate())}}, &fakeOld{rows: oldRows}
	if _, err := NewCandidateReader(res, old).Candidates(ctx, "2026-11-13", "short_put"); err != nil || old.calls != 0 {
		t.Errorf("day with a result run: err %v, old calls %d, want 0", err, old.calls)
	}
}

func TestCandidateReaderOrderIsRankThenId(t *testing.T) {
	t.Parallel()
	st, _ := newFake()
	ctx := context.Background()
	var rows []strategyresult.Row
	for _, k := range []struct {
		id    int
		alias string
		rank  *int
	}{{9, "", nil}, {10, "B", ip(1)}, {10, "A", ip(1)}, {3, "", nil}, {7, "", ip(1)}} {
		cand := fullCandidate()
		cand.InstrumentID, cand.Rank = k.id, k.rank
		r := pack(cand)
		if k.alias != "" {
			r.Key += ":" + k.alias
		}
		rows = append(rows, r)
	}
	if _, err := st.InsertResults(ctx, rows); err != nil {
		t.Fatal(err)
	}
	got, err := NewCandidateReader(st, &fakeOld{}).Candidates(ctx, "2026-11-13", "short_put")
	if err != nil {
		t.Fatal(err)
	}
	var ids []int
	for _, c := range got {
		ids = append(ids, c.InstrumentID)
	}
	if want := []int{7, 10, 10, 3, 9}; !reflect.DeepEqual(ids, want) {
		t.Errorf("order %v, want %v (ranked first, unranked last, ties by id)", ids, want)
	}
	res, _ := st.Results(ctx, "2026-11-13", "short_put")
	if res[1].Key != "10:A" || res[2].Key != "10:B" {
		t.Errorf("alias tie order %s, %s, want 10:A, 10:B", res[1].Key, res[2].Key)
	}
}

func TestCompareNamesTheDifferingColumn(t *testing.T) {
	t.Parallel()
	a := fullCandidate()
	b := fullCandidate()
	b.RunID, b.ComputedAt = "other", t0.Add(time.Hour) // never a difference
	b.Strike = "185"
	b.Ask = nil
	b.Fits = `{"A":"no_fit"}`
	lines, err := Compare(&fakeOld{rows: []store.Candidate{a}}, &fakeResults{rows: []strategyresult.Row{pack(b)}}, "2026-11-13")
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("lines = %q, want 3", lines)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"key 42: column Strike differs: old 180, new 185", "column Ask differs: old 2.2, new nil",
		"key 42: column Fits differs"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
	if strings.Contains(joined, "no_fit") || strings.Contains(joined, "RunID") || strings.Contains(joined, "ComputedAt") {
		t.Errorf("a line carries a fits value or an ignored column: %q", joined)
	}
	// A row on one side only is a line of its own.
	extra, _ := Compare(&fakeOld{rows: []store.Candidate{a, b}}, &fakeResults{rows: []strategyresult.Row{pack(a)}}, "2026-11-13")
	if len(extra) == 0 || !strings.Contains(extra[len(extra)-1], "only in the old rows") {
		t.Errorf("extra = %q", extra)
	}
}

func TestCompareEmptyWhenEqual(t *testing.T) {
	t.Parallel()
	a := fullCandidate()
	b := fullCandidate()
	b.RunID, b.ComputedAt = "other", t0.Add(time.Hour)
	lines, err := Compare(&fakeOld{rows: []store.Candidate{a}}, &fakeResults{rows: []strategyresult.Row{pack(b)}}, "2026-11-13")
	if err != nil || lines == nil || len(lines) != 0 {
		t.Fatalf("lines = %#v, %v, want empty", lines, err)
	}
}
