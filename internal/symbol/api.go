// Package symbol translates the stock symbol of the IBKR contract of watch.toml (the one spelling kept)
// into the spelling of a vendor. It reads nothing and keeps nothing. The implementation is
// internal/symbol/table.
package symbol

// Vendor names a spelling.
type Vendor string

// AlphaVantage is the spelling of the Alpha Vantage earnings calendar.
const AlphaVantage Vendor = "alphavantage"

// Translator is the translation layer. Both methods trim, upper-case and read a run of spaces, dots and
// dashes between two alphanumeric groups as one share-class separator, so the canonical form and the vendor
// forms of one company give one answer.
type Translator interface {
	// ToVendor returns the spelling of symbol that v uses ("BRK B" gives "BRK-B" for AlphaVantage). It fails
	// on an empty symbol and on an unknown vendor.
	ToVendor(symbol string, v Vendor) (string, error)
	// OSIRoot returns the option root and trading class of symbol: the separators removed ("BRK B" gives
	// "BRKB"). Its length is not checked and it is not padded.
	OSIRoot(symbol string) string
}

// NewTranslatorFunc is the constructor of the implementation (internal/symbol/table.New).
type NewTranslatorFunc func() Translator
