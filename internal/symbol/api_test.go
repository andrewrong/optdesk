package symbol_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/symbol"
	"github.com/andrewrong/optdesk/internal/symbol/contracttest"
)

// fakeTranslator is a minimal Translator: it splits on separator runs and joins the groups.
type fakeTranslator struct{}

func groups(s string) []string {
	return strings.FieldsFunc(strings.ToUpper(s), func(r rune) bool {
		return r == ' ' || r == '.' || r == '-'
	})
}

func (fakeTranslator) ToVendor(s string, v symbol.Vendor) (string, error) {
	if v != symbol.AlphaVantage {
		return "", errors.New("unknown vendor")
	}
	g := groups(s)
	if len(g) == 0 {
		return "", errors.New("empty symbol")
	}
	return strings.Join(g, "-"), nil
}

func (fakeTranslator) OSIRoot(s string) string { return strings.Join(groups(s), "") }

func newFake() symbol.Translator { return fakeTranslator{} }

func TestTranslatorFake(t *testing.T) {
	t.Parallel()
	contracttest.Run(t, newFake)
}

// sameCompany is a consumer: it uses a Translator only through the interface.
func sameCompany(tr symbol.Translator, entry, listing string) bool {
	a, err := tr.ToVendor(entry, symbol.AlphaVantage)
	if err != nil {
		return false
	}
	b, err := tr.ToVendor(listing, symbol.AlphaVantage)
	return err == nil && a == b
}

func TestConsumerThroughInterface(t *testing.T) {
	t.Parallel()
	var newT symbol.NewTranslatorFunc = newFake
	tr := newT()
	cases := []struct {
		entry, listing string
		want           bool
	}{
		{"BRK B", "BRK.B", true},
		{"BRK B", "brk-b", true},
		{"BF B", "BRK B", false},
		{"AAPL", "AAPL", true},
		{"", "AAPL", false},
	}
	for _, c := range cases {
		if got := sameCompany(tr, c.entry, c.listing); got != c.want {
			t.Errorf("sameCompany(%q, %q) = %v, want %v", c.entry, c.listing, got, c.want)
		}
	}
}
