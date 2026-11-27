package optionopen

import (
	"context"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func invalid(fields ...string) []broker.MissingField {
	var out []broker.MissingField
	for _, f := range fields {
		out = append(out, broker.MissingField{Field: f, Reason: broker.MissingInvalid})
	}
	return out
}

// noQuoteQuote is what the broker sends for a contract nobody quotes: live, no bid, no ask, model values and sizes.
func noQuoteQuote(b *fakeBroker, c broker.Contract) broker.Quote {
	q := okQuote(b.clk, c)
	q.Bid, q.Ask = nil, nil
	q.Missing = invalid("bid", "ask")
	return q
}

func TestRowStatusNoQuote(t *testing.T) {
	t.Parallel()
	clk := newFake(100).clk
	c := broker.Contract{ConID: 1}
	both := okQuote(clk, c)
	both.Bid, both.Ask, both.Missing = nil, nil, invalid("bid", "ask")
	one := okQuote(clk, c)
	one.Bid, one.Missing = nil, invalid("bid")
	bothPartial := both
	bothPartial.Status = broker.StatusPartial
	frozen := both
	frozen.MarketDataType = 2
	nothing := broker.Quote{Contract: c, Status: broker.StatusNoData, ReceivedAt: clk.Now()}

	for _, tc := range []struct {
		name   string
		q      broker.Quote
		status int
		code   string
	}{
		{"both sides invalid", both, stNoData, "NO_QUOTE"},
		{"both sides invalid, broker partial", bothPartial, stNoData, "NO_QUOTE"},
		{"one side invalid", one, stPartial, ""},
		{"frozen type", frozen, stNoData, ""},
		{"nothing arrived", nothing, stNoData, ""},
	} {
		st, code := rowStatus(tc.q)
		got := ""
		if code != nil {
			got = *code
		}
		if st != tc.status || got != tc.code {
			t.Errorf("%s: status %d code %q, want %d %q", tc.name, st, got, tc.status, tc.code)
		}
	}

	core := newCore(newFake(100))
	r := core.row(target(1, 1000, ClassPuts, 90, "P"), answer{status: stNoData, code: new("NO_QUOTE"), q: &both}, clk.Now())
	if r.IvDecimal == nil || r.Delta == nil {
		t.Errorf("a NO_QUOTE row lost its IV or delta: %+v", r)
	}
}

func TestNoQuoteRowIsFinalOnResume(t *testing.T) {
	t.Parallel()
	s := seed(1, 4)
	put := func(id, status int, code *string) {
		r := storedRow(sessDay, "0945", id, status)
		r.ErrorCode = code
		_ = s.InsertOptionOpen(context.Background(), []contracts.OptionOpen{r})
	}
	put(1000, stNoData, new("NO_QUOTE"))
	put(1001, stNoData, new("NO_QUOTE"))
	put(1002, stNoData, nil)
	b := newFake(100)
	runAll(t, newPull(t, s, b))
	if asked(b) != 2 {
		t.Fatalf("asked %d contracts, want 2 (the no data row with no code and the unstored one)", asked(b))
	}
	for _, c := range b.optCalls[0] {
		if c.ConID == 10000 || c.ConID == 10010 {
			t.Errorf("asked a NO_QUOTE item %d", c.ConID)
		}
	}
}

// noQuoteFirstOK answers ok for the first n contracts and the no-quote sentinel for the others.
func noQuoteFirstOK(n int, rest func(b *fakeBroker, c broker.Contract) broker.Quote) func(*fakeBroker) func(int, []broker.Contract) ([]broker.Quote, error) {
	return func(b *fakeBroker) func(int, []broker.Contract) ([]broker.Quote, error) {
		return func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
			qs := okQuotes(b.clk, cs)
			for i := range qs {
				if cs[i].ConID/10%1000 >= int64(n) {
					qs[i] = rest(b, cs[i])
				}
			}
			return qs, nil
		}
	}
}

func TestNoQuoteLeavesOkShareDenominator(t *testing.T) {
	t.Parallel()
	nq := noQuoteQuote
	bad := func(b *fakeBroker, c broker.Contract) broker.Quote {
		return broker.Quote{Contract: c, Status: broker.StatusError, ErrorCode: 321, ReceivedAt: b.clk.Now()}
	}
	gone := func(b *fakeBroker, c broker.Contract) broker.Quote {
		return broker.Quote{Contract: c, Status: broker.StatusDisconnected, ReceivedAt: b.clk.Now()}
	}
	for _, jp := range []bool{false, true} {
		expiry := "2026-12-11"
		if jp {
			expiry = "2027-01-10"
		}
		if res := runGapped(t, gappedPlan(30, 30, expiry), jp, noQuoteFirstOK(12, nq)); res.Err != nil || res.Usable != 12 {
			t.Errorf("jp %v 12 ok 18 no quote: err %v usable %d, want no error", jp, res.Err, res.Usable)
		}
		if res := runGapped(t, gappedPlan(30, 30, expiry), jp, noQuoteFirstOK(0, nq)); res.Err != nil {
			t.Errorf("jp %v all no quote: err %v, want no error", jp, res.Err)
		}
		out := runGapped(t, gappedPlan(20, 0, "2027-06-11"), jp, noQuoteFirstOK(0, nq))
		if out.Err == nil || out.Err.Kind() != contracts.NoValue || !strings.Contains(out.Err.Error(), "all outside band") {
			t.Errorf("jp %v all outside: err %v, want NoValue", jp, out.Err)
		}
		// Case B stays a failure: rows the broker could not answer still fail the share and end Retryable.
		res := runGapped(t, gappedPlan(30, 30, expiry), jp, noQuoteFirstOK(12, gone))
		if res.Err == nil || res.Err.Kind() != contracts.Retryable {
			t.Errorf("jp %v 12 ok 18 disconnected: err %v, want Retryable", jp, res.Err)
		}
		res = runGapped(t, gappedPlan(30, 30, expiry), jp, noQuoteFirstOK(12, bad))
		if res.Err == nil || res.Err.Kind() != contracts.Retryable {
			t.Errorf("jp %v 12 ok 18 error: err %v, want Retryable", jp, res.Err)
		}
	}
}

func TestTwelveOkEighteenErrorIsRetryable(t *testing.T) {
	t.Parallel()
	bad := func(b *fakeBroker, c broker.Contract) broker.Quote {
		return broker.Quote{Contract: c, Status: broker.StatusError, ErrorCode: 321, ReceivedAt: b.clk.Now()}
	}
	res := runGapped(t, gappedPlan(30, 30, "2026-12-11"), false, noQuoteFirstOK(12, bad))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("12 ok 18 error: %v, want Retryable (acceptance item 5)", res.Err)
	}
}
