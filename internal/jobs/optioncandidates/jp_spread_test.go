package optioncandidates

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestJPSpreadFigures(t *testing.T) {
	t.Parallel()
	const day = contracts.Day("2026-11-02")
	// put makes a quote with the given bid and ask, 30 days out unless expiry is set.
	put := func(bid, ask string) compute.OptionQuote {
		return compute.OptionQuote{Right: "P", Expiry: "2026-12-02", Bid: bid, Ask: ask, QuoteOK: true}
	}
	with := func(q compute.OptionQuote, f func(*compute.OptionQuote)) compute.OptionQuote { f(&q); return q }
	cases := []struct {
		name         string
		quotes       []compute.OptionQuote
		wantMin, med string
	}{
		// mid 1.00 for each: spreads 0.04, 0.08, 0.20
		{"odd count", []compute.OptionQuote{put("0.98", "1.02"), put("0.96", "1.04"), put("0.90", "1.10")}, "0.0400", "0.0800"},
		{"even count is the mean of the middle two", []compute.OptionQuote{put("0.98", "1.02"), put("0.96", "1.04"), put("0.90", "1.10"),
			put("0.80", "1.20")}, "0.0400", "0.1400"},
		{"single put", []compute.OptionQuote{put("0.958", "1.042")}, "0.0840", "0.0840"},
		{"none qualify", nil, "", ""},
		{"outside window, calls, no bid, zero bid, no ask, unusable are out", []compute.OptionQuote{
			with(put("0.98", "1.02"), func(q *compute.OptionQuote) { q.Expiry = "2027-06-01" }),
			with(put("0.98", "1.02"), func(q *compute.OptionQuote) { q.Expiry = "2026-11-03" }),
			with(put("0.98", "1.02"), func(q *compute.OptionQuote) { q.Right = "C" }),
			put("", "1.02"), put("0", "1.02"), put("0.98", ""),
			with(put("0.98", "1.02"), func(q *compute.OptionQuote) { q.QuoteOK = false }),
			put("0.90", "1.10"),
		}, "0.2000", "0.2000"},
	}
	for _, c := range cases {
		gotMin, gotMed := spreadFigures(c.quotes, day, 7, 60)
		if gotMin != c.wantMin || gotMed != c.med {
			t.Errorf("%s: spreadFigures = (%q, %q), want (%q, %q)", c.name, gotMin, gotMed, c.wantMin, c.med)
		}
	}
}
