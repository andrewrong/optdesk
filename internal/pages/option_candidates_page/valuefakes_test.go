package option_candidates_page

import (
	"context"
	"strings"
	"sync"

	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// fakeValuation answers Current with st or err.
type fakeValuation struct {
	valuation.Reader
	mu    sync.Mutex
	st    valuation.State
	err   error
	calls int
}

func (f *fakeValuation) Current(context.Context) (valuation.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.st, f.err
}

// fakeBuy is a resolver that answers one price for a zone it can use and the reasons of the zone rule, in the
// order of Decision 2, otherwise; it records every input. reason, when set, is answered for every company.
type fakeBuy struct {
	mu     sync.Mutex
	in     []buypriceapi.Input
	price  string
	reason string
}

func (f *fakeBuy) Resolve(in buypriceapi.Input) buypriceapi.Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.in = append(f.in, in)
	none := func(r string) buypriceapi.Result {
		return buypriceapi.Result{Source: buypriceapi.SourceNone, ZoneReason: r}
	}
	var z *buypriceapi.Zone
	for i := range in.Zones {
		if in.Symbol != "" && strings.EqualFold(in.Zones[i].Symbol, in.Symbol) {
			z = &in.Zones[i]
			break
		}
	}
	switch {
	case z == nil:
		return none(buypriceapi.ReasonNoProfile)
	case z.Pool == "removed":
		return none(buypriceapi.ReasonRemoved)
	case !z.Reviewed:
		return none(buypriceapi.ReasonNoReview)
	case !in.Tables.Present:
		return none(buypriceapi.ReasonNoTables)
	case z.Currency != in.Currency:
		return none(buypriceapi.ReasonCurrency)
	case f.reason != "":
		return none(f.reason)
	}
	return buypriceapi.Result{Price: f.price, Source: buypriceapi.SourceValuation, ReviewDay: z.ReviewDay}
}

func valueEntry(symbol string, review *valuation.Review) valuation.Entry {
	return valuation.Entry{
		Profile: valuation.Profile{CompanyKey: "k-" + symbol, Symbol: symbol, Currency: "USD", RMarket: "US", Pool: "in_pool"},
		Current: review,
	}
}

func valueReview() *valuation.Review {
	return &valuation.Review{V: "100", P: "3", VersionDate: contracts.Day("2026-11-10")}
}

func valueState(entries ...valuation.Entry) valuation.State {
	return valuation.State{
		Entries:  entries,
		RVersion: &valuation.RVersion{RUS: "0.9", RJP: "0.8", RHKCN: "0.7"},
		Margin:   &valuation.MarginTableVersion{Rows: []valuation.MarginTableRow{{PFrom: "1", M: "0.1", W: "0.2", H: "0.3", S: "0.4"}}},
	}
}
