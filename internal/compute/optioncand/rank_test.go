package optioncand

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

// rankInput holds four ok puts; yield = bid/strike x 365/dte.
func rankInput() compute.CandidateInput {
	in := baseInput()
	in.BuyPrice = "1000"
	mk := func(id, strike, bid string, dte int) compute.OptionQuote {
		q := putQuote(id, strike, bid, addDays(runDay, dte))
		q.Ask = bid
		return q
	}
	in.Quotes = []compute.OptionQuote{
		mk("lowy", "100", "1.00", 30),
		mk("hiy", "100", "3.00", 30),
		mk("mid", "100", "2.00", 30),
		mk("blocked", "100", "0", 30),
	}
	return in
}

func ids(rows []compute.Candidate) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.InstrumentID)
	}
	return out
}

func TestRankByYield(t *testing.T) {
	t.Parallel()
	rows := eval(rankInput())
	if got, want := ids(rows), []string{"hiy", "mid", "lowy", "blocked"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order %v, want %v", got, want)
	}
	for i, r := range rows[:3] {
		if r.Rank != i+1 || r.RankInUnderlying != i+1 || r.Status != "ok" {
			t.Errorf("%s rank %d/%d status %s", r.InstrumentID, r.Rank, r.RankInUnderlying, r.Status)
		}
	}
}

func TestRankTieOrder(t *testing.T) {
	t.Parallel()
	in := baseInput()
	in.BuyPrice = "1000"
	mk := func(id, strike, bid string, dte int) compute.OptionQuote {
		q := putQuote(id, strike, bid, addDays(runDay, dte))
		q.Ask = bid
		return q
	}
	in.Accounts[0].Funds = "100000"
	// every row yields exactly 365/3000; ties go to the lower DTE, then the lower strike,
	// then the instrument id
	in.Quotes = []compute.OptionQuote{
		mk("c-200", "200", "2.00", 30),
		mk("b-100-b", "100", "1.00", 30),
		mk("b-100-a", "100", "1.00", 30),
		mk("early", "300", "2.50", 25),
	}
	rows := eval(in)
	got := ids(rows)
	want := []string{"early", "b-100-a", "b-100-b", "c-200"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order %v, want %v", got, want)
	}
}

func TestBlockedHasNoRank(t *testing.T) {
	t.Parallel()
	rows := eval(rankInput())
	last := rows[len(rows)-1]
	if last.InstrumentID != "blocked" || last.Status != "blocked" || last.Rank != 0 || last.RankInUnderlying != 0 {
		t.Fatalf("%+v", last)
	}
	in := rankInput()
	in.Earnings = nil // every row unknown or blocked: nobody is ranked
	for _, r := range eval(in) {
		if r.Rank != 0 || r.Status == "ok" {
			t.Errorf("%s status %s rank %d", r.InstrumentID, r.Status, r.Rank)
		}
	}
}

func TestDeterministic(t *testing.T) {
	t.Parallel()
	in := rankInput()
	in.Quotes = append(in.Quotes,
		putQuote("x1", "95", "1.50", addDays(runDay, 35)),
		putQuote("x2", "90", "0.50", addDays(runDay, 28)),
		putQuote("x3", "99", "1.00", addDays(runDay, 40)),
	)
	want := eval(in)
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 20; i++ {
		shuffled := in
		shuffled.Quotes = append([]compute.OptionQuote(nil), in.Quotes...)
		rng.Shuffle(len(shuffled.Quotes), func(a, b int) {
			shuffled.Quotes[a], shuffled.Quotes[b] = shuffled.Quotes[b], shuffled.Quotes[a]
		})
		if got := eval(shuffled); !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d differs:\n%v\n%v", i, ids(got), ids(want))
		}
	}
}

func TestNoBuyPriceRowsRankedByYield(t *testing.T) {
	t.Parallel()
	in := rankInput()
	in.BuyPrice = ""
	in.Quotes = in.Quotes[:3]
	rows := eval(in)
	if got, want := ids(rows), []string{"hiy", "mid", "lowy"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order %v, want %v", got, want)
	}
	for i, r := range rows {
		if r.Rank != i+1 || r.RankInUnderlying != i+1 || r.Status != "ok" || !hasMark(r, "no_buy_price") {
			t.Errorf("%s rank %d/%d status %s marks %v", r.InstrumentID, r.Rank, r.RankInUnderlying, r.Status, r.Marks)
		}
	}
}
