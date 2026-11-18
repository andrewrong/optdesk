package option_candidates_page

import (
	"context"
	"sync"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
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

// fakeLines answers a BuyHigh the page cannot produce, or err, and records the inputs.
type fakeLines struct {
	valuelinesapi.Calculator
	mu  sync.Mutex
	in  []valuelinesapi.Input
	buy string
	err error
}

func (f *fakeLines) Lines(in valuelinesapi.Input) (valuelinesapi.Lines, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.in = append(f.in, in)
	if f.err != nil {
		return valuelinesapi.Lines{}, f.err
	}
	return valuelinesapi.Lines{BuyHigh: f.buy}, nil
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
