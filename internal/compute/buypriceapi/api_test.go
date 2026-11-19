package buypriceapi_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
)

// fakeResolver answers one source by the shape of the input, with made-up numbers.
type fakeResolver struct{}

func (fakeResolver) Resolve(in buypriceapi.Input) buypriceapi.Result {
	if len(in.Zones) > 0 {
		return buypriceapi.Result{Price: "91.2000", Source: buypriceapi.SourceValuation, ReviewDay: "2026-11-17"}
	}
	if in.Hand != "" {
		return buypriceapi.Result{Price: in.Hand, Source: buypriceapi.SourceHand, ZoneReason: buypriceapi.ReasonNoProfile}
	}
	return buypriceapi.Result{Source: buypriceapi.SourceNone, ZoneReason: buypriceapi.ReasonNoProfile}
}

// describe is a caller that reads every field of Result.
func describe(r buypriceapi.Resolver, in buypriceapi.Input) (string, string, string, string) {
	res := r.Resolve(in)
	return res.Price, res.Source, res.ReviewDay, res.ZoneReason
}

func TestResolverContractFake(t *testing.T) {
	t.Parallel()
	var r buypriceapi.Resolver = fakeResolver{}
	zone := buypriceapi.Zone{Symbol: "AAA", Currency: "USD", Pool: "active", Market: "US", V: "100", P: "3", ReviewDay: "2026-11-17", Reviewed: true}
	cases := []struct {
		name       string
		in         buypriceapi.Input
		price, src string
	}{
		{"valuation", buypriceapi.Input{Symbol: "AAA", Currency: "USD", Hand: "90.00", Zones: []buypriceapi.Zone{zone}}, "91.2000", buypriceapi.SourceValuation},
		{"hand", buypriceapi.Input{Symbol: "BBB", Currency: "USD", Hand: "41.00"}, "41.00", buypriceapi.SourceHand},
		{"none", buypriceapi.Input{Symbol: "CCC", Currency: "USD"}, "", buypriceapi.SourceNone},
	}
	for _, c := range cases {
		price, src, _, _ := describe(r, c.in)
		if price != c.price || src != c.src {
			t.Errorf("%s: got %q %q, want %q %q", c.name, price, src, c.price, c.src)
		}
	}
}

func TestMarkWordsDistinct(t *testing.T) {
	t.Parallel()
	const noBuyPrice = "no_buy_price"
	a, b := buypriceapi.MarkValuation, buypriceapi.MarkHand
	if a == b || a == noBuyPrice || b == noBuyPrice {
		t.Fatalf("mark words %q %q %q are not distinct", a, b, noBuyPrice)
	}
}
