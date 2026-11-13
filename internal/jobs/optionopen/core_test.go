package optionopen

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

var (
	sessDay    = time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	testExpiry = time.Date(2026, 12, 11, 0, 0, 0, 0, time.UTC) // 30 days after sessDay
	open0945   = time.Date(2026, 11, 11, 14, 45, 0, 0, time.UTC)
)

func f(v float64) *float64 { return &v }

type typedErr struct{ kind contracts.ErrorKind }

func (e typedErr) Error() string             { return "broker: snapshot: " + string(e.kind) }
func (e typedErr) Kind() contracts.ErrorKind { return e.kind }

// fakeBroker answers the underlying snapshots from spots (the last one repeats) and the option snapshots from opt.
type fakeBroker struct {
	clk        *clockwork.FakeClock
	spots      []float64
	under      func(n int) ([]broker.Quote, error)
	opt        func(call int, cs []broker.Contract) ([]broker.Quote, error)
	step       time.Duration // the clock moves by this at every call
	underCalls int
	optCalls   [][]broker.Contract
	kinds      []broker.Kind
}

func (b *fakeBroker) ListChains(context.Context, broker.Contract) ([]broker.Chain, error) {
	return nil, errors.New("not used")
}

func (b *fakeBroker) ResolveContract(context.Context, broker.Contract) (broker.Resolved, error) {
	return broker.Resolved{}, errors.New("not used")
}

func (b *fakeBroker) Close() error { return nil }

func (b *fakeBroker) Snapshot(_ context.Context, kind broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	b.clk.Advance(b.step)
	if kind == broker.KindSnapshot {
		n := b.underCalls
		b.underCalls++
		if b.under != nil {
			return b.under(n)
		}
		s := b.spots[min(n, len(b.spots)-1)]
		return []broker.Quote{{Contract: cs[0], Last: f(s), ReceivedAt: b.clk.Now()}}, nil
	}
	b.kinds = append(b.kinds, kind)
	b.optCalls = append(b.optCalls, cs)
	if b.opt != nil {
		return b.opt(len(b.optCalls)-1, cs)
	}
	return okQuotes(b.clk, cs), nil
}

func okQuote(clk *clockwork.FakeClock, c broker.Contract) broker.Quote {
	return broker.Quote{Contract: c, Status: broker.StatusOK, MarketDataType: 1, Generation: 4, ReceivedAt: clk.Now(),
		Bid: f(1), Ask: f(2), ImpliedVol: f(0.3), Delta: f(-0.2)}
}

func okQuotes(clk *clockwork.FakeClock, cs []broker.Contract) []broker.Quote {
	out := make([]broker.Quote, len(cs))
	for i, c := range cs {
		out[i] = okQuote(clk, c)
	}
	return out
}

func newFake(spots ...float64) *fakeBroker {
	return &fakeBroker{clk: clockwork.NewFakeClockAt(open0945), spots: spots}
}

func target(under, id int, class Class, strike float64, right Right) Target {
	return Target{
		Item: contracts.OptionOpenItem{
			SessionDay: types.SerializableDate{Time: sessDay}, Slot: "0945", PlanID: "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
			InstrumentID: id, UnderlyingID: under, Class: gen.OptionOpenItemClass(class), Rank: id,
			PlannedSpot: 100, Sigma: 0.3, SigmaSource: gen.OptionOpenItemSigmaSource(SigmaPriorClose),
		},
		Symbol: "AAA", Currency: "USD", Expiry: testExpiry, Strike: strike, Right: right,
		Under:  broker.Contract{SecType: "STK", Symbol: "AAA", ConID: int64(under)},
		Option: broker.Contract{SecType: "OPT", Symbol: "AAA", ConID: int64(id), Strike: "x"},
	}
}

func newCore(b *fakeBroker) *Core {
	return &Core{B: b, Clk: b.clk, Params: DefaultPlanParams(), Slot: "0945", Chunk: 30,
		Retry: map[Class]bool{ClassAroundSpot: true, ClassPuts: true, ClassHeld: true}}
}

func rowOf(t *testing.T, r Result, id int) contracts.OptionOpen {
	t.Helper()
	var got []contracts.OptionOpen
	for _, row := range r.Rows {
		if row.InstrumentID == id {
			got = append(got, row)
		}
	}
	if len(got) != 1 {
		t.Fatalf("instrument %d has %d rows, want exactly one", id, len(got))
	}
	return got[0]
}

func code(r contracts.OptionOpen) string {
	if r.ErrorCode == nil {
		return ""
	}
	return *r.ErrorCode
}

func TestEveryPlannedItemHasOneRowAndTheTypeOfTheAnswer(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	b.opt = func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
		qs := okQuotes(b.clk, cs)
		for i := range qs {
			qs[i].MarketDataType = 3 // delayed
		}
		return qs, nil
	}
	ts := []Target{target(1, 11, ClassAroundSpot, 95, Put), target(1, 12, ClassPuts, 90, Put), target(1, 13, ClassPuts, 80, Put)}
	r := newCore(b).Pull(context.Background(), ts)
	if len(r.Rows) != 3 {
		t.Fatalf("rows %d, want 3", len(r.Rows))
	}
	for _, id := range []int{11, 12} {
		row := rowOf(t, r, id)
		if row.Status != stNoData || row.MarketDataType != 3 || row.OutsideBand {
			t.Errorf("%d: status %d type %d outside %v", id, row.Status, row.MarketDataType, row.OutsideBand)
		}
		if row.CaptureID == "" || row.Slot != "0945" || row.PlanID == "" || row.SentAt == nil {
			t.Errorf("%d: incomplete row %+v", id, row)
		}
	}
	for _, c := range b.kinds {
		if c != broker.KindOptionOpen {
			t.Errorf("wave 1 sends KindOptionOpen, got %v", c)
		}
	}
}

// ladder is a 1 dollar grid of puts from 100 down to 80 (enough puts in any band for the six-put guard to rest) and
// one far strike at 60. The id of a contract is 1000 plus its strike.
func ladder() []Target {
	var ts []Target
	for k := 100; k >= 80; k-- {
		ts = append(ts, target(1, 1000+k, ClassPuts, float64(k), Put))
	}
	return append(ts, target(1, 1060, ClassPuts, 60, Put))
}

func TestBandIsChosenAtTheOpenSpot(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		spot            float64
		inside, outside int // the strike 80 and the strike 60 are the ones that move
	}{
		{100, 1090, 1080}, // band floor 88.7: 90 is inside, 80 outside
		{90, 1080, 1060},  // band floor 79.8: 80 is inside, 60 outside
	} {
		b := newFake(tc.spot)
		r := newCore(b).Pull(context.Background(), ladder())
		in, out := rowOf(t, r, tc.inside), rowOf(t, r, tc.outside)
		if in.Status != stOK || in.OutsideBand {
			t.Errorf("spot %v: contract %d status %d outside %v, want asked", tc.spot, tc.inside, in.Status, in.OutsideBand)
		}
		if out.Status != stDeferred || !out.OutsideBand || out.SentAt != nil {
			t.Errorf("spot %v: contract %d status %d outside %v sent %v, want deferred outside the band", tc.spot, tc.outside, out.Status, out.OutsideBand, out.SentAt)
		}
		for _, cs := range b.optCalls {
			for _, c := range cs {
				if int(c.ConID) == tc.outside {
					t.Errorf("spot %v: a contract outside the band was asked", tc.spot)
				}
			}
		}
		if len(r.Rows) != 22 {
			t.Errorf("spot %v: rows %d, want one per item (22)", tc.spot, len(r.Rows))
		}
	}
}

func TestPlannedDeferredAndHeldOutsideTheBand(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	over := target(1, 12, ClassPuts, 90, Put)
	over.Item.Deferred = true
	held := target(1, 13, ClassHeld, 50, Put)
	ts := []Target{target(1, 11, ClassAroundSpot, 95, Put), over, held}
	r := newCore(b).Pull(context.Background(), ts)
	if row := rowOf(t, r, 12); row.Status != stDeferred || row.OutsideBand || row.SentAt != nil {
		t.Errorf("planned deferred: status %d outside %v", row.Status, row.OutsideBand)
	}
	if row := rowOf(t, r, 13); row.Status != stOK {
		t.Errorf("a held contract is always asked, status %d", row.Status)
	}
}

func TestUnderlyingSnapshotFailsOrHasNoData(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		under func(int) ([]broker.Quote, error)
		code  string
	}{
		{"no data", func(int) ([]broker.Quote, error) { return []broker.Quote{{Status: broker.StatusNoData}}, nil }, "NO_DATA"},
		{"timeout", func(int) ([]broker.Quote, error) { return []broker.Quote{{Status: broker.StatusTimeout}}, nil }, "TIMEOUT"},
		{"ibkr code", func(int) ([]broker.Quote, error) {
			return []broker.Quote{{Status: broker.StatusError, ErrorCode: 162}}, nil
		}, "162"},
		{"call error", func(int) ([]broker.Quote, error) { return nil, typedErr{contracts.Permanent} }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newFake(100)
			b.under = tc.under
			ts := []Target{target(1, 11, ClassAroundSpot, 95, Put), target(1, 12, ClassPuts, 90, Put)}
			r := newCore(b).Pull(context.Background(), ts)
			if len(b.optCalls) != 0 {
				t.Fatalf("a quote was taken against a stale spot: %d calls", len(b.optCalls))
			}
			if len(r.Rows) != 2 {
				t.Fatalf("rows %d, want one per item", len(r.Rows))
			}
			for _, row := range r.Rows {
				if row.OutsideBand {
					t.Error("outside_band must be false for a failed underlying snapshot")
				}
				if tc.code != "" && (row.Status != stDeferred || code(row) != tc.code) {
					t.Errorf("status %d code %q, want deferred %q", row.Status, code(row), tc.code)
				}
				if tc.code == "" && row.Status != stError {
					t.Errorf("a permanent call error gives status %d, want error", row.Status)
				}
			}
		})
	}
}

func TestBrokerStatusToRowStatus(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		q      broker.Quote
		status int
		code   string
	}{
		{"clean 200 not listed", broker.Quote{Status: broker.StatusError, ErrorCode: 200, ErrorText: "No security definition has been found"}, stNotListed, "200"},
		{"200 of another text", broker.Quote{Status: broker.StatusError, ErrorCode: 200, ErrorText: "ambiguous contract"}, stError, "200"},
		{"354", broker.Quote{Status: broker.StatusError, ErrorCode: 354}, stNoSubscription, "354"},
		{"no subscription status", broker.Quote{Status: broker.StatusNoSubscription}, stNoSubscription, "NO_SUBSCRIPTION"},
		{"other code", broker.Quote{Status: broker.StatusError, ErrorCode: 101}, stError, "101"},
		{"connection status", broker.Quote{Status: broker.StatusDisconnected}, stDisconnected, "DISCONNECTED"},
		{"connection code", broker.Quote{Status: broker.StatusError, ErrorCode: 1100}, stDisconnected, "1100"},
		{"no data", broker.Quote{Status: broker.StatusNoData}, stNoData, ""},
		{"empty ok", broker.Quote{Status: broker.StatusOK}, stNoData, ""},
		{"no model iv", broker.Quote{Status: broker.StatusOK, Bid: f(1), Ask: f(2)}, stPartial, ""},
		{"bid ask iv", broker.Quote{Status: broker.StatusOK, Bid: f(1), Ask: f(2), ImpliedVol: f(0.3)}, stOK, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newFake(100)
			b.opt = func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
				q := tc.q
				q.Contract, q.ReceivedAt = cs[0], b.clk.Now()
				return []broker.Quote{q}, nil
			}
			r := newCore(b).Pull(context.Background(), []Target{target(1, 11, ClassAroundSpot, 95, Put)})
			row := rowOf(t, r, 11)
			if row.Status != tc.status || code(row) != tc.code {
				t.Errorf("status %d code %q, want %d %q", row.Status, code(row), tc.status, tc.code)
			}
		})
	}
}

func TestWholeSnapshotUnavailableEndsEveryItemDisconnected(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	b.opt = func(int, []broker.Contract) ([]broker.Quote, error) { return nil, typedErr{contracts.Retryable} }
	ts := []Target{target(1, 11, ClassAroundSpot, 95, Put), target(1, 12, ClassPuts, 90, Put), target(2, 21, ClassAroundSpot, 95, Put)}
	ts[2].Symbol, ts[2].Under.ConID = "BBB", 2
	r := newCore(b).Pull(context.Background(), ts)
	if len(r.Rows) != 3 {
		t.Fatalf("rows %d, want 3", len(r.Rows))
	}
	for _, row := range r.Rows {
		if row.Status != stDisconnected || code(row) != "UNAVAILABLE" {
			t.Errorf("%d: status %d code %q, want disconnected UNAVAILABLE", row.InstrumentID, row.Status, code(row))
		}
	}
	if b.underCalls != 1 || len(b.optCalls) != 1 {
		t.Errorf("after UNAVAILABLE nothing more is asked: %d underlying, %d option calls", b.underCalls, len(b.optCalls))
	}
}

func TestUnavailableAtTheUnderlyingSnapshotEndsEveryItemDisconnected(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	b.under = func(int) ([]broker.Quote, error) { return nil, typedErr{contracts.Retryable} }
	r := newCore(b).Pull(context.Background(), []Target{target(1, 11, ClassAroundSpot, 95, Put), target(1, 12, ClassPuts, 90, Put)})
	for _, row := range r.Rows {
		if row.Status != stDisconnected {
			t.Errorf("status %d, want disconnected", row.Status)
		}
	}
	if len(r.Rows) != 2 {
		t.Errorf("rows %d, want 2", len(r.Rows))
	}
}

func TestTimeoutRetriedOnceForPutsAndAroundSpotNotForCalls(t *testing.T) {
	t.Parallel()
	timeout := func(c broker.Contract) broker.Quote { return broker.Quote{Contract: c, Status: broker.StatusTimeout} }
	for _, tc := range []struct {
		name      string
		class     Class
		strike    float64
		right     Right
		secondOK  bool
		wantCalls int
		want      int
	}{
		{"put recovers", ClassPuts, 90, Put, true, 2, stOK},
		{"put twice times out", ClassPuts, 90, Put, false, 2, stDeadline},
		{"around_spot twice times out", ClassAroundSpot, 95, Put, false, 2, stDeadline},
		{"held recovers", ClassHeld, 90, Put, true, 2, stOK},
		{"call is not retried", ClassCalls, 110, Call, true, 1, stDeadline},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newFake(100)
			b.opt = func(call int, cs []broker.Contract) ([]broker.Quote, error) {
				if call == 1 && tc.secondOK {
					return okQuotes(b.clk, cs), nil
				}
				return []broker.Quote{timeout(cs[0])}, nil
			}
			ts := []Target{target(1, 11, tc.class, tc.strike, tc.right)}
			if tc.class == ClassCalls {
				ts = append(ts, target(1, 12, ClassHeld, 50, Put)) // a held position gives the name its calls
				b.opt = func(call int, cs []broker.Contract) ([]broker.Quote, error) {
					out := okQuotes(b.clk, cs)
					for i, c := range cs {
						if c.ConID == 11 {
							out[i] = timeout(c)
						}
					}
					return out, nil
				}
			}
			r := newCore(b).Pull(context.Background(), ts)
			row := rowOf(t, r, 11)
			if row.Status != tc.want || len(b.optCalls) != tc.wantCalls {
				t.Errorf("status %d after %d calls, want %d after %d", row.Status, len(b.optCalls), tc.want, tc.wantCalls)
			}
			if tc.wantCalls == 2 && len(b.optCalls[1]) != 1 {
				t.Errorf("the retry asks only the timed-out contract, asked %d", len(b.optCalls[1]))
			}
		})
	}
}

func TestPartialThatEndedAtTheTimeoutIsRetriedOnceAndNoDataNever(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	b.opt = func(call int, cs []broker.Contract) ([]broker.Quote, error) {
		q := okQuote(b.clk, cs[0])
		if call == 0 {
			q.Status, q.ImpliedVol = broker.StatusPartial, nil
		}
		return []broker.Quote{q}, nil
	}
	r := newCore(b).Pull(context.Background(), []Target{target(1, 11, ClassPuts, 90, Put)})
	if row := rowOf(t, r, 11); row.Status != stOK || len(b.optCalls) != 2 {
		t.Errorf("partial then ok: status %d after %d calls", row.Status, len(b.optCalls))
	}

	nb := newFake(100)
	nb.opt = func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
		return []broker.Quote{{Contract: cs[0], Status: broker.StatusNoData, ReceivedAt: nb.clk.Now()}}, nil
	}
	r = newCore(nb).Pull(context.Background(), []Target{target(1, 11, ClassPuts, 90, Put)})
	if row := rowOf(t, r, 11); row.Status != stNoData || len(nb.optCalls) != 1 {
		t.Errorf("no_data: status %d after %d calls, want no_data after 1", row.Status, len(nb.optCalls))
	}
}

func TestSkewFlagFromTheMoveOfTheSpotWithinAGroup(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		spots []float64 // the open spot, then the spot at the end of the group
		skew  bool
		price float64
	}{
		{"moved 2 percent", []float64{100, 102}, true, 102},
		{"moved 1 percent", []float64{100, 101}, true, 101},
		{"moved half a percent", []float64{100, 100.5}, false, 100.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newFake(tc.spots...)
			r := newCore(b).Pull(context.Background(), []Target{target(1, 11, ClassAroundSpot, 95, Put), target(1, 12, ClassPuts, 90, Put)})
			for _, row := range r.Rows {
				if row.Skew != tc.skew || row.UnderlyingPrice == nil || *row.UnderlyingPrice != tc.price {
					t.Errorf("skew %v price %v, want %v %v", row.Skew, row.UnderlyingPrice, tc.skew, tc.price)
				}
			}
		})
	}
}

func TestGroupsAreByExpiryAndChunked(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	var ts []Target
	for i := 0; i < 5; i++ {
		ts = append(ts, target(1, 10+i, ClassPuts, 90+float64(i), Put))
	}
	ts[0].Strike = 95 // the strike at or below the spot is around_spot, the others puts
	later := target(1, 20, ClassPuts, 91, Put)
	later.Expiry = testExpiry.AddDate(0, 0, 7)
	ts = append(ts, later)
	c := newCore(b)
	c.Chunk = 2
	r := c.Pull(context.Background(), ts)
	if len(r.Rows) != 6 {
		t.Fatalf("rows %d", len(r.Rows))
	}
	var sizes []int
	for _, cs := range b.optCalls {
		sizes = append(sizes, len(cs))
	}
	if len(sizes) != 4 || sizes[0] != 2 || sizes[1] != 2 || sizes[2] != 1 || sizes[3] != 1 {
		t.Errorf("chunks %v, want [2 2 1 1] (two groups: five contracts, then one)", sizes)
	}
	if b.optCalls[3][0].ConID != 20 {
		t.Error("the later expiry is asked after the nearer one")
	}
	// Three samples of the underlying: the open spot and the end of each of the two groups.
	if b.underCalls != 3 {
		t.Errorf("underlying snapshots %d, want 3", b.underCalls)
	}
}

func TestBudgetSpentDefersWhatIsLeft(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	b.step = 5 * time.Minute
	c := newCore(b)
	c.Budget = 8 * time.Minute
	c.Chunk = 1
	r := c.Pull(context.Background(), []Target{target(1, 11, ClassAroundSpot, 95, Put), target(1, 12, ClassPuts, 90, Put), target(1, 13, ClassPuts, 91, Put)})
	if len(r.Rows) != 3 {
		t.Fatalf("rows %d, want 3", len(r.Rows))
	}
	deferred := 0
	for _, row := range r.Rows {
		if row.Status == stDeferred {
			deferred++
			if code(row) != "BUDGET" || row.OutsideBand {
				t.Errorf("deferred by budget: code %q outside %v", code(row), row.OutsideBand)
			}
		}
	}
	if deferred == 0 || deferred == 3 {
		t.Errorf("deferred %d, want some but not all", deferred)
	}
}

func TestTargetWithAProblemEndsError(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	bad := target(1, 12, ClassPuts, 90, Put)
	bad.Problem = "NO_CONTRACT"
	r := newCore(b).Pull(context.Background(), []Target{target(1, 11, ClassAroundSpot, 95, Put), bad})
	if row := rowOf(t, r, 12); row.Status != stError || code(row) != "NO_CONTRACT" {
		t.Errorf("status %d code %q", row.Status, code(row))
	}
	if row := rowOf(t, r, 11); row.Status != stOK {
		t.Errorf("the other item is quoted, status %d", row.Status)
	}
}

func TestCrossedQuoteIsStoredAsTheBrokerGaveIt(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	b.opt = func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
		q := okQuote(b.clk, cs[0])
		q.Bid, q.Ask = f(2.5), f(1.5)
		return []broker.Quote{q}, nil
	}
	r := newCore(b).Pull(context.Background(), []Target{target(1, 11, ClassAroundSpot, 95, Put)})
	row := rowOf(t, r, 11)
	if row.Status != stOK || *row.Bid != 2.5 || *row.Ask != 1.5 || row.Skew || row.OutsideBand || row.ErrorCode != nil {
		t.Errorf("a crossed quote is stored unchanged with no flag: %+v", row)
	}
}

func TestRunSummaryAndHeartbeat(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	b.step = 20 * time.Second
	b.opt = func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
		qs := okQuotes(b.clk, cs)
		qs[len(qs)-1].ImpliedVol = nil // partial
		return qs, nil
	}
	var buf bytes.Buffer
	c := newCore(b)
	c.Beat = 30 * time.Second
	c.Log = slog.New(slog.NewTextHandler(&buf, nil))
	r := c.Pull(context.Background(), []Target{target(1, 11, ClassAroundSpot, 95, Put), target(1, 12, ClassPuts, 90, Put), target(1, 13, ClassPuts, 80, Put)})
	s := r.Summary
	if s.Items != 3 || s.OK != 2 || s.Partial != 1 {
		t.Errorf("summary %+v, want 3 items, 2 ok, 1 partial", s)
	}
	if s.Seconds != 20 {
		t.Errorf("seconds %v, want 20 (the group is requested after the open snapshot, answered 20 s later)", s.Seconds)
	}
	if len(s.Stocks) != 1 || s.Stocks[0].Done != 3 || s.Stocks[0].Total != 3 || s.Stocks[0].LastRowT.IsZero() {
		t.Errorf("progress %+v", s.Stocks)
	}
	if line := s.String(); !strings.Contains(line, "items=3 ok=2 partial=1") {
		t.Errorf("summary line %q", line)
	}
	if n := strings.Count(buf.String(), "option_open heartbeat"); n == 0 {
		t.Errorf("no heartbeat in a run of 60 s with a 30 s beat: %q", buf.String())
	}
}

func TestRefreshAsksThePlanOnlyAndKeepsFreshRows(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	b.opt = func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
		qs := okQuotes(b.clk, cs)
		for i, c := range cs {
			switch c.ConID {
			case 12:
				qs[i].Status, qs[i].ImpliedVol = broker.StatusPartial, nil
			case 13:
				qs[i] = broker.Quote{Contract: c, Status: broker.StatusTimeout}
			}
		}
		return qs, nil
	}
	far := target(1, 14, ClassPuts, 40, Put) // outside any band at spot 100: Refresh uses the stored plan, no band
	ts := []Target{target(1, 11, ClassAroundSpot, 95, Put), target(1, 12, ClassPuts, 90, Put), target(1, 13, ClassPuts, 91, Put), far}
	res := Refresh(context.Background(), b, b.clk, ts, 2)
	if res.Asked != 4 || len(res.Rows) != 3 {
		t.Fatalf("asked %d, rows %d; want 4 asked, the ok, the partial and the far strike kept (timeout dropped)", res.Asked, len(res.Rows))
	}
	for _, row := range res.Rows {
		if row.Slot != openquote.SlotRefresh {
			t.Errorf("slot %q, want %q", row.Slot, openquote.SlotRefresh)
		}
	}
	if b.underCalls != 0 || len(b.optCalls) != 2 || len(b.optCalls[0]) != 2 {
		t.Errorf("refresh asks only the contracts in chunks of 2: %d underlying, calls %v", b.underCalls, len(b.optCalls))
	}
}

func TestRefreshDropsEverythingWhenTheBrokerIsUnavailable(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	b.opt = func(int, []broker.Contract) ([]broker.Quote, error) { return nil, typedErr{contracts.Retryable} }
	res := Refresh(context.Background(), b, b.clk, []Target{target(1, 11, ClassPuts, 90, Put), target(1, 12, ClassPuts, 91, Put)}, 1)
	if len(res.Rows) != 0 || res.Asked != 2 || len(b.optCalls) != 1 {
		t.Errorf("rows %d asked %d calls %d; want none, 2, 1", len(res.Rows), res.Asked, len(b.optCalls))
	}
}

func TestNonLiveMarketDataTypeIsNoDataWithTheRowKept(t *testing.T) {
	t.Parallel()
	for _, typ := range []int{2, 3, 4} {
		b := newFake(100)
		b.opt = func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
			qs := okQuotes(b.clk, cs)
			for i := range qs {
				qs[i].MarketDataType = typ
			}
			return qs, nil
		}
		r := newCore(b).Pull(context.Background(), []Target{target(1, 11, ClassAroundSpot, 95, Put)})
		row := rowOf(t, r, 11)
		if row.Status == stOK || row.Status == stPartial || row.Status != stNoData {
			t.Errorf("type %d: status %d, want no_data", typ, row.Status)
		}
		if row.MarketDataType != typ || row.Bid == nil {
			t.Errorf("type %d: kept type %d bid %v", typ, row.MarketDataType, row.Bid)
		}
	}
}

func TestLiveMarketDataTypeOneStaysOk(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	r := newCore(b).Pull(context.Background(), []Target{target(1, 11, ClassAroundSpot, 95, Put)})
	if row := rowOf(t, r, 11); row.Status != stOK || row.MarketDataType != 1 {
		t.Errorf("status %d type %d, want ok type 1", row.Status, row.MarketDataType)
	}
}

func TestRoundDelta(t *testing.T) {
	t.Parallel()
	nan, inf := math.NaN(), math.Inf(-1)
	for _, tc := range []struct {
		name string
		in   *float64
		want string // shortest text of the result; "nil" for nil
	}{
		{"nil", nil, "nil"},
		{"long delta", f(-0.31495717239425247), "-0.315"},
		{"four places stay", f(-0.2446), "-0.2446"},
		{"half", f(-0.5), "-0.5"},
		{"below half of the last place", f(0.00004), "0"},
		{"nan", &nan, "NaN"},
		{"infinity", &inf, "-Inf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := roundDelta(tc.in)
			text := "nil"
			if got != nil {
				text = strconv.FormatFloat(*got, 'f', -1, 64)
			}
			if text != tc.want {
				t.Errorf("roundDelta = %s, want %s", text, tc.want)
			}
		})
	}
}

func TestRowStoresDeltaFourPlaces(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	b.opt = func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
		q := okQuote(b.clk, cs[0])
		q.Delta, q.Gamma = f(-0.31495717239425247), f(0.0123456789)
		return []broker.Quote{q}, nil
	}
	r := newCore(b).Pull(context.Background(), []Target{target(1, 11, ClassPuts, 90, Put)})
	row := rowOf(t, r, 11)
	if row.Delta == nil || *row.Delta != -0.315 {
		t.Errorf("delta = %v, want -0.315", row.Delta)
	}
	if row.Gamma == nil || *row.Gamma != 0.0123456789 {
		t.Errorf("gamma = %v, want 0.0123456789", row.Gamma)
	}
}

func TestPullLogsEveryRowThatIsNotOkWithItsAbsentFields(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	b.opt = func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
		qs := okQuotes(b.clk, cs)
		qs[1].Ask, qs[1].ImpliedVol = nil, nil // partial
		qs[2].MarketDataType = 2               // frozen
		return qs, nil
	}
	var buf bytes.Buffer
	c := newCore(b)
	c.Log = slog.New(slog.NewTextHandler(&buf, nil))
	c.Pull(context.Background(), []Target{target(1, 11, ClassAroundSpot, 95, Put), target(1, 12, ClassPuts, 90, Put), target(1, 13, ClassPuts, 80, Put)})
	var lines []string
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "option_open row not ok") {
			lines = append(lines, l)
		}
	}
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "status=partial") || !strings.Contains(lines[0], `absent="[ask last iv]"`) || strings.Contains(lines[0], "bid") {
		t.Errorf("partial line %q", lines[0])
	}
	if !strings.Contains(lines[1], "market_data_type=2") || !strings.Contains(lines[1], "status=no_data") {
		t.Errorf("frozen line %q", lines[1])
	}
	if !strings.Contains(lines[0], "strike=90") || !strings.Contains(lines[0], "right=P") || !strings.Contains(lines[0], "symbol=AAA") {
		t.Errorf("partial line attributes %q", lines[0])
	}
}
