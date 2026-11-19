package option_candidates_page

import (
	"bytes"
	"os"
	"reflect"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func markedRow(price *string, marks string) store.Candidate {
	c := okPut(1)
	c.BuyPrice, c.Marks = price, marks
	return c
}

func TestBuySourceFromMarks(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ marks, want string }{
		"valuation":              {`["buy_from_valuation"]`, buypriceapi.SourceValuation},
		"hand":                   {`["buy_from_hand"]`, buypriceapi.SourceHand},
		"valuation among others": {`["eod_fallback","buy_from_valuation"]`, buypriceapi.SourceValuation},
	} {
		if got := buySourceCode(markedRow(strp("90.00"), tc.marks)); got != tc.want {
			t.Errorf("%s: code = %q, want %q", name, got, tc.want)
		}
	}
}

func TestBuySourceOldRowIsHand(t *testing.T) {
	t.Parallel()
	for _, marks := range []string{`[]`, `["eod_fallback"]`, `not json`} {
		if got := buySourceCode(markedRow(strp("90.00"), marks)); got != buypriceapi.SourceHand {
			t.Errorf("marks %s: code = %q", marks, got)
		}
	}
}

func TestBuySourceNoPriceNone(t *testing.T) {
	t.Parallel()
	for _, p := range []*string{nil, strp("")} {
		if got := buySourceCode(markedRow(p, `["buy_from_valuation"]`)); got != buypriceapi.SourceNone {
			t.Errorf("price %v: code = %q", p, got)
		}
	}
	if got := buySourceOf(markedRow(nil, `[]`)); got != "" {
		t.Errorf("buySourceOf with no price = %q", got)
	}
}

// stripOf is the strip of the first company of tiers_ok after its rows are changed by mod.
func stripsOf(t *testing.T, mod func(rows []store.Candidate)) []companyView {
	t.Helper()
	c, _ := tierScenario("tiers_ok")
	rr := c.Candidates.(fakeReader)
	mod(rr.rows["2026-11-09/short_put"])
	return buildTiersOf(t, c).Companies
}

func TestStripBuySourceBadge(t *testing.T) {
	t.Parallel()
	val := stripsOf(t, func(rows []store.Candidate) {
		for i := range rows {
			if rows[i].UnderlyingID == 7 {
				rows[i].Marks = `["buy_from_valuation"]`
			}
		}
	})[0]
	if val.BuySource == nil || val.BuySource.Code != "valuation" || val.BuySource.Label != "Valuation buy zone" || val.BuyPrice == nil {
		t.Errorf("valuation strip = %+v", val)
	}
	hand := stripsOf(t, func(rows []store.Candidate) {
		for i := range rows {
			if rows[i].UnderlyingID == 7 {
				rows[i].Marks = `["buy_from_hand"]`
			}
		}
	})[0]
	if hand.BuySource == nil || hand.BuySource.Code != "hand" || hand.BuySource.Label != "Hand price" {
		t.Errorf("hand strip = %+v", hand.BuySource)
	}
	old := buildTiersOf(t, mustScenario(t)).Companies[0]
	if old.BuySource == nil || old.BuySource.Code != "hand" {
		t.Errorf("old row strip = %+v", old.BuySource)
	}
}

func mustScenario(t *testing.T) Config {
	t.Helper()
	c, _ := tierScenario("tiers_ok")
	return c
}

// A company whose first row has no price takes the source and price of its first row that has one.
func TestStripSourceFromFirstPricedRow(t *testing.T) {
	t.Parallel()
	cs := stripsOf(t, func(rows []store.Candidate) {
		rows[0].BuyPrice = nil
		rows[1].Marks = `["buy_from_valuation"]`
	})
	if cs[0].BuySource == nil || cs[0].BuySource.Code != "valuation" || cs[0].BuyPrice == nil || *cs[0].BuyPrice != "$90.00" {
		t.Errorf("strip = %+v", cs[0])
	}
}

func TestStripNotJudgedText(t *testing.T) {
	t.Parallel()
	c, _ := tierScenario("tiers_ok")
	c.Valuation, c.Buy = &fakeValuation{st: valueState(valueEntry("BBB", valueReview()))}, &fakeBuy{price: "50"}
	cs := buildTiersOf(t, c).Companies
	var bbb *companyView
	for i := range cs {
		if cs[i].Ticker == "BBB" {
			bbb = &cs[i]
		}
	}
	if bbb == nil {
		t.Fatal("no BBB strip")
	}
	if bbb.BuyPrice != nil || bbb.BuyPriceNote == nil || *bbb.BuyPriceNote != "not judged" {
		t.Errorf("strip = %+v", bbb)
	}
	if bbb.Valuation == nil || bbb.Valuation.GateText != "not judged" || bbb.Valuation.GateLabel != "Buy price used by the gate" {
		t.Errorf("valuation = %+v", bbb.Valuation)
	}
	if bbb.BuySource == nil || bbb.BuySource.Code != "none" || bbb.BuySource.Label != "Not judged" {
		t.Errorf("source = %+v", bbb.BuySource)
	}
}

func TestShortRowValuationBadge(t *testing.T) {
	t.Parallel()
	r := okList(t).Groups[1].Rows[0]
	b := r.BuySource
	if b == nil || b.Code != "valuation" || b.Label != "Valuation buy zone" || b.Gloss != zhBuySourceValuation || r.BuyLink != nil {
		t.Errorf("row = %+v", r)
	}
	noBuy := okList(t).Groups[0].Rows[0]
	if noBuy.BuySource != nil || noBuy.BuyLink == nil {
		t.Errorf("no-price row = %+v", noBuy)
	}
}

func TestBuySourceTextsFromZhConsts(t *testing.T) {
	t.Parallel()
	for code, gloss := range map[string]string{
		"valuation": zhBuySourceValuation, "hand": zhBuySourceHand, "none": zhBuySourceNone,
	} {
		b := buySourceBadge(code)
		if b == nil || b.Gloss != gloss || b.Code != code || b.Label == "" {
			t.Errorf("%s: badge = %+v", code, b)
			continue
		}
		if n := utf8.RuneCountInString(gloss); n < 1 || n > 200 {
			t.Errorf("%s gloss has %d characters", code, n)
		}
		for _, r := range gloss {
			if unicode.IsDigit(r) {
				t.Errorf("%s gloss holds a number", code)
			}
		}
	}
	if buySourceBadge("other") != nil {
		t.Error("badge for an unknown code")
	}
}

func TestBuySourceValidatesSchema(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"tiers_ok", "tiers_gaps", "valueline", "shortlist_ok", "shortlist_gaps"} {
		golden, err := os.ReadFile("testdata/" + n + ".golden.json")
		if err != nil {
			t.Fatal(err)
		}
		v, err := jsonschema.UnmarshalJSON(bytes.NewReader(golden))
		if err != nil {
			t.Fatal(err)
		}
		if err := pageSchema(t).Validate(v); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
}

func TestBuySourceDoesNotChangeTiers(t *testing.T) {
	t.Parallel()
	with, _ := tierScenario("tiers_ok")
	without, _ := tierScenario("tiers_ok")
	rr := with.Candidates.(fakeReader)
	for i := range rr.rows["2026-11-09/short_put"] {
		rr.rows["2026-11-09/short_put"][i].Marks = `["buy_from_valuation"]`
	}
	a, b := buildTiersOf(t, with), buildTiersOf(t, without)
	for _, v := range []*tiersView{a, b} {
		for i := range v.Companies {
			v.Companies[i].BuySource, v.Companies[i].SourceBadge, v.Companies[i].source = nil, nil, ""
		}
	}
	if !reflect.DeepEqual(a, b) {
		t.Error("the source marks changed the tiers, rows, counts, gates or the order of companies")
	}
}
