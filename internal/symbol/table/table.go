// Package table implements symbol.Translator with the share-class rule of docs/design/symbol.md
// (Decisions 4 to 6). It does no I/O and keeps no table.
package table

import (
	"errors"
	"fmt"
	"strings"

	"github.com/andrewrong/optdesk/internal/symbol"
)

type translator struct{}

// New returns the Translator (the value of symbol.NewTranslatorFunc).
func New() symbol.Translator { return translator{} }

// groups trims, upper-cases and splits symbol at every run of spaces, dots and dashes.
func groups(s string) []string {
	return strings.FieldsFunc(strings.ToUpper(strings.TrimSpace(s)), func(r rune) bool {
		return r == ' ' || r == '.' || r == '-'
	})
}

func (translator) ToVendor(s string, v symbol.Vendor) (string, error) {
	g := groups(s)
	if len(g) == 0 {
		return "", errors.New("symbol: empty symbol")
	}
	switch v {
	case symbol.AlphaVantage:
		return strings.Join(g, "-"), nil
	default:
		return "", fmt.Errorf("symbol: unknown vendor %q", string(v))
	}
}

func (translator) OSIRoot(s string) string {
	return strings.Join(groups(s), "")
}
