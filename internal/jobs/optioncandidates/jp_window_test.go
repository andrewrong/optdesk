package optioncandidates

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optioncandapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

// windowEnv is the JP stores with one name (7203) whose open puts expire 20, 21, 58, 60 and 61 days after the
// fixture day 2026-11-19. Its evaluator keeps the quotes inside the DTE window of the params it is given.
type windowEnv struct{ *jpEnv }

func (w windowEnv) EvaluateIn(in optioncandapi.CurrencyInput, p compute.CandidateParams) []compute.Candidate {
	var out []compute.Candidate
	run, _ := time.Parse(time.DateOnly, string(in.RunDay))
	for _, q := range in.Quotes {
		exp, _ := time.Parse(time.DateOnly, string(q.Expiry))
		dte := int(exp.Sub(run).Hours() / 24)
		if !q.QuoteOK || dte < p.MinDTE || dte > p.MaxDTE {
			continue
		}
		out = append(out, compute.Candidate{Kind: compute.KindShortPut, Underlying: in.Underlying, InstrumentID: q.InstrumentID,
			Expiry: q.Expiry, DTE: dte, Right: "P", Strike: q.Strike, Multiplier: q.Multiplier, Bid: q.Bid,
			AnnualizedYield: "0.1", Status: "ok", Gates: []compute.GateResult{{Gate: "buy_price", Status: compute.GatePass}}})
	}
	return out
}

func newWindowEnv(t *testing.T, dtes map[int64]string) windowEnv {
	t.Helper()
	e := loadJP(t)
	e.open, e.eod, e.terms = nil, nil, map[int64]store.OptionTerms{}
	e.names = []store.InstrumentName{{InstrumentID: 1, Type: "stock", Name: "7203"}}
	for id, exp := range dtes {
		e.terms[id] = store.OptionTerms{InstrumentID: id, Right: "P", Strike: "2400", Multiplier: "100", ExpirationDate: contracts.Day(exp)}
		e.open = append(e.open, contracts.OptionOpen{UnderlyingID: 1, InstrumentID: int(id), Status: 1, Currency: "JPY",
			Bid: f64(30), Ask: f64(31), BidSize: f64(5), UnderlyingPrice: f64(2500), MarketDataType: 1,
			CaptureID: "o" + strconv.Itoa(int(id))})
	}
	return windowEnv{e}
}

func (w windowEnv) run(t *testing.T) (contracts.Job, contracts.JobResult) {
	t.Helper()
	st := w.stores()
	j, err := NewJP(st, w, []jobs.WatchEntry{jpEntry("7203", "jp:7203")}, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	return j, j.Run(context.Background(), w.day, nil)
}

func TestJPWindowSixty(t *testing.T) {
	t.Parallel()
	// 2026-11-19 plus 20, 21, 58, 60 and 61 days.
	w := newWindowEnv(t, map[int64]string{
		920: "2026-12-09", 921: "2026-12-10", 958: "2027-01-16", 960: "2027-01-18", 961: "2027-01-19",
	})
	j, res := w.run(t)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	for id, want := range map[int]bool{920: false, 921: true, 958: true, 960: true, 961: false} {
		if _, got := w.rowOf(id); got != want {
			t.Errorf("instrument %d has a row = %v, want %v", id, got, want)
		}
	}
	if skippedHas(j, "no usable open quote") {
		t.Fatalf("a name with puts of 21 to 60 days was skipped: %v", j.(interface{ Skipped() []string }).Skipped())
	}

	// A name whose only puts are 58 days out is usable; one with only 20 or 61 days is not.
	for _, c := range []struct {
		exp  string
		want bool
	}{{"2027-01-16", true}, {"2027-01-19", false}, {"2026-12-09", false}} {
		w := newWindowEnv(t, map[int64]string{930: c.exp})
		j, res := w.run(t)
		if res.Err != nil {
			t.Fatal(res.Err)
		}
		if got := !skippedHas(j, "no usable open quote"); got != c.want {
			t.Errorf("expiry %s counted usable = %v, want %v", c.exp, got, c.want)
		}
		if _, got := w.rowOf(930); got != c.want {
			t.Errorf("expiry %s has a row = %v, want %v", c.exp, got, c.want)
		}
	}
}
