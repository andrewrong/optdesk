package buyprice_test

import (
	"errors"

	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

// fakeCalc answers Lines with a fixed BuyHigh text (or error); every other method is unused.
type fakeCalc struct {
	valuelinesapi.Calculator
	buyHigh string
	err     error
	got     *valuelinesapi.Input
}

func (f *fakeCalc) Lines(in valuelinesapi.Input) (valuelinesapi.Lines, error) {
	if f.got != nil {
		*f.got = in
	}
	if f.err != nil {
		return valuelinesapi.Lines{}, f.err
	}
	return valuelinesapi.Lines{BuyHigh: f.buyHigh}, nil
}

var errBad = errors.New("bad")

// zoneOf is a usable USD profile with invented numbers.
func zoneOf(symbol string) buypriceapi.Zone {
	return buypriceapi.Zone{Symbol: symbol, Currency: "USD", Pool: "active", Market: valuelinesapi.MarketUS,
		V: "100", P: "3", ReviewDay: "2026-11-17", Reviewed: true}
}

func tablesOn() buypriceapi.Tables {
	return buypriceapi.Tables{Present: true, R: valuelinesapi.RValues{US: "1", JP: "1", HKCN: "1"},
		Margin: []valuelinesapi.MarginRow{{PFrom: "1", M: "0.1"}}}
}

func inputOf(symbol, hand string, zones ...buypriceapi.Zone) buypriceapi.Input {
	return buypriceapi.Input{Symbol: symbol, Currency: "USD", Hand: hand, Zones: zones, Tables: tablesOn()}
}
