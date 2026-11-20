package shared

import (
	"regexp"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestBarSourceOf(t *testing.T) {
	t.Parallel()
	ib := contracts.PriceBar{Source: "ibkr"}
	yf := contracts.PriceBar{Source: "yfinance"}
	cases := []struct {
		name     string
		rows     []contracts.PriceBar
		kind     string
		wantNote bool
	}{
		{"no rows", nil, "ibkr", false},
		{"all ibkr", []contracts.PriceBar{ib, ib}, "ibkr", false},
		{"all yfinance", []contracts.PriceBar{yf, yf}, "yfinance", true},
		{"both", []contracts.PriceBar{ib, yf}, "mixed", true},
	}
	han := regexp.MustCompile(`\p{Han}`)
	latin := regexp.MustCompile(`[A-Za-z]+`)
	for _, c := range cases {
		kind, note := BarSourceOf(c.rows)
		if kind != c.kind {
			t.Errorf("%s: kind = %q, want %q", c.name, kind, c.kind)
		}
		if !c.wantNote {
			if note != "" {
				t.Errorf("%s: note = %q, want empty", c.name, note)
			}
			continue
		}
		if !han.MatchString(note) {
			t.Errorf("%s: note %q has no Chinese text", c.name, note)
		}
		for _, w := range latin.FindAllString(note, -1) {
			if w != "IB" && w != "Yahoo" {
				t.Errorf("%s: note %q has Latin word %q", c.name, note, w)
			}
		}
		if !strings.Contains(note, "Yahoo") {
			t.Errorf("%s: note %q does not name Yahoo", c.name, note)
		}
	}
}
