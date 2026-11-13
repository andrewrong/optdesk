// Package stockreg declares the optional store methods that write the stock side
// and the company of an instrument and that read a stock by symbol or an
// identifier by scheme. Declarations only; the value store.OpenInstrumentFunc
// returns implements them.
package stockreg

import "context"

// Issuer is a company. Empty strings mean "not set"; at least one of CIK and EdinetCode is set.
type Issuer struct {
	Name, Country, CIK, EdinetCode string
}

// Registration is what the owner's list and the broker say about one stock, etf or index.
type Registration struct {
	ConID                                       int64
	Kind                                        string // stock or etf
	Symbol, Market, PrimaryExchange, SectorFund string // Market is US, JP or EU; a Tokyo symbol has no suffix
	Name, Currency, MIC, CalendarID             string
	Issuer                                      *Issuer // nil: no company is known
}

// StockRegistrar writes the stock side and the company of an instrument, creating the instrument by its ibkr_conid
// when none exists. Newest input wins; equal input changes nothing. It returns the instrument id. It is an optional
// method of the value store.OpenInstrumentFunc returns; the job asserts it.
type StockRegistrar interface {
	RegisterStock(ctx context.Context, r Registration) (int64, error)
}

// StockDirectory reads by symbol and by scheme. Optional method of the same value.
type StockDirectory interface {
	// LookupStock returns the id of the stock with the symbol in the market; ok is false when there is none.
	LookupStock(ctx context.Context, symbol, market string) (id int64, ok bool, err error)
	// Identifier returns the value of the scheme valid now for the instrument; ok is false when there is none.
	Identifier(ctx context.Context, instrumentID int64, scheme string) (value string, ok bool, err error)
}
