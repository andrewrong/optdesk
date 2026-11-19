package buyprice_test

import (
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/buyprice"
	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
)

func TestResolveValuationWinsOverHand(t *testing.T) {
	t.Parallel()
	// a text the resolver cannot produce: it must come back unchanged
	r := buyprice.New(&fakeCalc{buyHigh: "87.123456789"})
	got := r.Resolve(inputOf("AAA", "41.00", zoneOf("AAA")))
	want := buypriceapi.Result{Price: "87.123456789", Source: buypriceapi.SourceValuation, ReviewDay: "2026-11-17"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestResolveHandWhenNoZone(t *testing.T) {
	t.Parallel()
	r := buyprice.New(&fakeCalc{buyHigh: "9"})
	got := r.Resolve(inputOf("BBB", "41.00"))
	want := buypriceapi.Result{Price: "41.00", Source: buypriceapi.SourceHand, ZoneReason: buypriceapi.ReasonNoProfile}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestResolveNone(t *testing.T) {
	t.Parallel()
	r := buyprice.New(&fakeCalc{buyHigh: "9"})
	got := r.Resolve(inputOf("CCC", ""))
	want := buypriceapi.Result{Source: buypriceapi.SourceNone, ZoneReason: buypriceapi.ReasonNoProfile}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestResolveReasons(t *testing.T) {
	t.Parallel()
	mod := func(f func(*buypriceapi.Zone)) buypriceapi.Zone { z := zoneOf("AAA"); f(&z); return z }
	cases := []struct {
		name   string
		in     buypriceapi.Input
		calc   *fakeCalc
		reason string
	}{
		{"no profile", inputOf("AAA", "41.00", zoneOf("BBB")), &fakeCalc{buyHigh: "9"}, buypriceapi.ReasonNoProfile},
		{"ambiguous", inputOf("AAA", "41.00", zoneOf("AAA"), zoneOf("aaa")), &fakeCalc{buyHigh: "9"}, buypriceapi.ReasonAmbiguous},
		{"removed", inputOf("AAA", "41.00", mod(func(z *buypriceapi.Zone) { z.Pool = "removed" })), &fakeCalc{buyHigh: "9"}, buypriceapi.ReasonRemoved},
		{"no review", inputOf("AAA", "41.00", mod(func(z *buypriceapi.Zone) { z.Reviewed = false })), &fakeCalc{buyHigh: "9"}, buypriceapi.ReasonNoReview},
		{"no tables", func() buypriceapi.Input {
			in := inputOf("AAA", "41.00", zoneOf("AAA"))
			in.Tables.Present = false
			return in
		}(), &fakeCalc{buyHigh: "9"}, buypriceapi.ReasonNoTables},
		{"currency", inputOf("AAA", "41.00", mod(func(z *buypriceapi.Zone) { z.Currency = "JPY" })), &fakeCalc{buyHigh: "9"}, buypriceapi.ReasonCurrency},
		{"calculator error", inputOf("AAA", "41.00", zoneOf("AAA")), &fakeCalc{err: errBad}, buypriceapi.ReasonBadInput},
		{"zero buy high", inputOf("AAA", "41.00", zoneOf("AAA")), &fakeCalc{buyHigh: "0.00"}, buypriceapi.ReasonBadInput},
		{"negative buy high", inputOf("AAA", "41.00", zoneOf("AAA")), &fakeCalc{buyHigh: "-3"}, buypriceapi.ReasonBadInput},
		{"empty buy high", inputOf("AAA", "41.00", zoneOf("AAA")), &fakeCalc{}, buypriceapi.ReasonBadInput},
	}
	for _, c := range cases {
		got := buyprice.New(c.calc).Resolve(c.in)
		want := buypriceapi.Result{Price: "41.00", Source: buypriceapi.SourceHand, ZoneReason: c.reason}
		if got != want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, want)
		}
	}
}

func TestResolveBadHandIsUnset(t *testing.T) {
	t.Parallel()
	r := buyprice.New(&fakeCalc{buyHigh: "9"})
	for _, hand := range []string{"", "0", "0.00", "-5", "abc", "1e3", "."} {
		got := r.Resolve(inputOf("CCC", hand))
		if got.Source != buypriceapi.SourceNone || got.Price != "" {
			t.Errorf("hand %q: got %+v, want none", hand, got)
		}
	}
}

func TestResolveMatchIgnoresCase(t *testing.T) {
	t.Parallel()
	r := buyprice.New(&fakeCalc{buyHigh: "55.5"})
	got := r.Resolve(inputOf("aaa", "", zoneOf("AAA")))
	if got.Source != buypriceapi.SourceValuation || got.Price != "55.5" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveEmptyKeyNoZone(t *testing.T) {
	t.Parallel()
	r := buyprice.New(&fakeCalc{buyHigh: "55.5"})
	got := r.Resolve(inputOf("", "41.00", zoneOf("")))
	want := buypriceapi.Result{Price: "41.00", Source: buypriceapi.SourceHand, ZoneReason: buypriceapi.ReasonNoProfile}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestResolveCurrencyIsTheInputs(t *testing.T) {
	t.Parallel()
	r := buyprice.New(&fakeCalc{buyHigh: "4100"})
	z := zoneOf("BBB")
	z.Currency = "JPY"
	in := inputOf("BBB", "", z)
	in.Currency = "JPY"
	if got := r.Resolve(in); got.Source != buypriceapi.SourceValuation || got.Price != "4100" {
		t.Fatalf("JPY: got %+v", got)
	}
	in.Currency = "USD"
	if got := r.Resolve(in); got.ZoneReason != buypriceapi.ReasonCurrency || got.Source != buypriceapi.SourceNone {
		t.Fatalf("USD: got %+v", got)
	}
}

// TestResolveIsPure: resolve.go imports no store, jobs or pages package, database/sql, os or time.
func TestResolveIsPure(t *testing.T) {
	t.Parallel()
	f, err := parser.ParseFile(token.NewFileSet(), "resolve.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, im := range f.Imports {
		p, _ := strconv.Unquote(im.Path.Value)
		if p == "database/sql" || p == "os" || p == "time" ||
			strings.Contains(p, "/internal/store") || strings.Contains(p, "/internal/jobs") || strings.Contains(p, "/internal/pages") {
			t.Errorf("resolve.go imports %q", p)
		}
	}
}
