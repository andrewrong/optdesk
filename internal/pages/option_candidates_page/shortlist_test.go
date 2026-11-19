package option_candidates_page

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionshortlistapi"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func okList(t *testing.T) *shortListView {
	t.Helper()
	rows := shortRows()
	return listOf(t, shortConfig(rows, &fakeShortlister{res: shortResult(rows)}, true))
}

func TestShortListFromShortlister(t *testing.T) {
	t.Parallel()
	l := okList(t)
	if len(l.Groups) != 2 || l.Groups[0].Ticker != "BBB" || l.Groups[1].Ticker != "AAA" {
		t.Fatalf("groups = %+v", l.Groups)
	}
	if got := l.Groups[1].Rows; len(got) != 2 || got[0].Strike.Main != "$90.00" || got[1].Strike.Main != "$85.00" {
		t.Errorf("rows of AAA are not in the order of the result: %+v", got)
	}
	if l.Groups[1].Spot != "$96.50" || l.Groups[1].Summary != "2 rows, best 20.0%" {
		t.Errorf("group = %+v", l.Groups[1])
	}
	checkFigures(t, l.Groups[1].Rows[0])
	checkBadges(t, l)
}

func checkFigures(t *testing.T, r shortRow) {
	t.Helper()
	if r.OTM.Main != "6.7%" || r.Delta.Main != "0.19" || r.Annualized.Main != "20.0%" || r.Expiry.Main != "2026-12-18" ||
		r.Expiry.Sub == nil || *r.Expiry.Sub != "39d" {
		t.Errorf("row = %+v", r)
	}
	if r.Premium.Main != "$0.94" || r.Premium.Sub == nil || *r.Premium.Sub != "$42.00" {
		t.Errorf("premium = %+v", r.Premium)
	}
	if r.Buy.Main != "$90.00" || r.Buy.Sub == nil || *r.Buy.Sub != "BE $84.58 vs $90.00, margin 6.0%" {
		t.Errorf("buy = %+v", r.Buy)
	}
}

func checkBadges(t *testing.T, l *shortListView) {
	t.Helper()
	r, w := l.Groups[1].Rows[0], l.Groups[0].Rows[0]
	if len(w.Badges) != 1 || w.Badges[0].Code != "wide_spread" || w.Badges[0].Label != "Wide spread" || w.Badges[0].Gloss == "" {
		t.Errorf("badges = %+v", w.Badges)
	}
	if len(r.Badges) != 0 {
		t.Errorf("badges on a normal row: %+v", r.Badges)
	}
}

func TestShortListCountsLine(t *testing.T) {
	t.Parallel()
	rows := shortRows()
	res := shortResult(rows)
	res.Scanned, res.Shown = 2343, 3
	l := listOf(t, shortConfig(rows, &fakeShortlister{res: res}, false))
	if l.Counts != "Scanned 2,343 puts / Shown 3 rows, 2 underlyings" || l.RunDay != "Run day 2026-11-09" {
		t.Errorf("counts = %q run_day = %q", l.Counts, l.RunDay)
	}
	if l.FoldedTitle != "All 3 puts (tier view and stored tables)" {
		t.Errorf("folded = %q", l.FoldedTitle)
	}
	if len(l.Columns) != 8 || l.Columns[0].ID != "strike" || l.Columns[7].ID != "liquidity" {
		t.Errorf("columns = %+v", l.Columns)
	}
	if len(l.Filters) != 6 {
		t.Errorf("filters = %+v", l.Filters)
	}
}

func TestDroppedItemsOrderAndZeroOmitted(t *testing.T) {
	t.Parallel()
	l := okList(t)
	var codes []string
	for _, d := range l.DroppedItems {
		codes = append(codes, d.Code+"="+d.Count)
	}
	if strings.Join(codes, ",") != "no_bid=2,otm_low=3,over_cap=1" {
		t.Errorf("items = %v", codes)
	}
	if l.Dropped == nil || *l.Dropped != "Not shown 6" {
		t.Errorf("dropped = %v", l.Dropped)
	}
	if l.DroppedItems[1].Label != "OTM below 5%" || l.DroppedItems[2].Label != "Over 3 per underlying" {
		t.Errorf("labels = %+v", l.DroppedItems)
	}
	rows := shortRows()
	res := shortResult(rows)
	res.Dropped = nil
	l = listOf(t, shortConfig(rows, &fakeShortlister{res: res}, false))
	if l.Dropped != nil || l.DroppedItems == nil || len(l.DroppedItems) != 0 {
		t.Errorf("nothing dropped: %v %v", l.Dropped, l.DroppedItems)
	}
}

func TestCaveatOnlyWithStaleChain(t *testing.T) {
	t.Parallel()
	if l := okList(t); l.Caveat == nil || !strings.Contains(*l.Caveat, "2") {
		t.Errorf("caveat = %v", l.Caveat)
	}
	rows := shortRows()
	res := shortResult(rows)
	res.StaleChain = 0
	if l := listOf(t, shortConfig(rows, &fakeShortlister{res: res}, false)); l.Caveat != nil {
		t.Errorf("caveat = %v", *l.Caveat)
	}
}

func TestNilShortlistOmitsProperty(t *testing.T) {
	t.Parallel()
	rows := shortRows()
	out := buildShort(t, shortConfig(rows, nil, true))
	if strings.Contains(string(out), "short_put_list") {
		t.Error("property written without a reader")
	}
	f := &fakeShortlister{res: shortResult(shortRows())}
	c := evConfig(nil, &fakeEval{})
	c.Shortlist = f
	out = buildShort(t, c)
	if strings.Contains(string(out), "short_put_list") || f.calls() != 0 {
		t.Errorf("property written or Select called without rows (%d)", f.calls())
	}
}

func TestBuyLinkOnlyWithoutBuyPrice(t *testing.T) {
	t.Parallel()
	l := okList(t)
	noBuy := l.Groups[0].Rows[0]
	if noBuy.BuyLink == nil || *noBuy.BuyLink != "/plan/price-lines" || noBuy.Buy.Gap == nil || noBuy.Buy.Gap.Text != "No buy price" {
		t.Errorf("row without buy price = %+v", noBuy)
	}
	if noBuy.BuySource != nil {
		t.Errorf("source on a row with no price: %+v", noBuy.BuySource)
	}
	if l.Groups[1].Rows[0].BuyLink != nil {
		t.Error("link on a row with a buy price")
	}
}

func TestBuySourceSlotIsHand(t *testing.T) {
	t.Parallel()
	r := okList(t).Groups[1].Rows[1] // the row with no source mark: an old row with a price is hand
	if r.BuySource == nil || r.BuySource.Code != "hand" || r.BuySource.Label == "" || r.BuySource.Gloss == "" || r.BuyLink != nil {
		t.Errorf("row = %+v", r)
	}
	if got := buySourceOf(shortRows()[2]); got != "" {
		t.Errorf("source of a row with no price = %q", got)
	}
}

func TestDeltaGapWhenEmpty(t *testing.T) {
	t.Parallel()
	c := shortScenario("shortlist_gaps")
	r := listOf(t, c).Groups[0].Rows[0]
	if r.Delta.Gap == nil || r.Delta.Main != "No delta" {
		t.Errorf("delta = %+v", r.Delta)
	}
}

func TestExpiryChipFromResult(t *testing.T) {
	t.Parallel()
	rows := shortRows()
	res := shortResult(rows)
	res.DTEMin, res.DTEMax = "23", "44"
	l := listOf(t, shortConfig(rows, &fakeShortlister{res: res}, false))
	if l.Filters[0].ID != "expiry" || l.Filters[0].Value != "23-44 d" || !strings.Contains(l.Filters[0].Hint, "job.toml") {
		t.Errorf("chip = %+v", l.Filters[0])
	}
	want := map[string]string{"strike": "5-15%", "delta": "0.15-0.30", "cap": "3", "bid": "present", "open_interest": "present"}
	for _, f := range l.Filters[1:] {
		if want[f.ID] != f.Value {
			t.Errorf("chip %s = %q", f.ID, f.Value)
		}
	}
}

func TestEveryDropCodeHasLabelAndHint(t *testing.T) {
	t.Parallel()
	b := testBounds
	for _, code := range []string{"bad_row", "no_bid", "no_open_interest", "no_spot", "otm_low", "otm_high", "no_delta", "delta_out", "over_cap"} {
		if dropLabel(code, b) == code || dropHints[code] == "" {
			t.Errorf("code %s has no label or hint", code)
		}
	}
	res := optionshortlistapi.Result{Dropped: []optionshortlistapi.Drop{{Code: "new_code", N: 1}}, Bounds: b}
	if _, items := droppedOf(res); len(items) != 1 || items[0].Label != "new_code" || items[0].Hint == "" {
		t.Errorf("unknown code = %+v", items)
	}
}

func checkShortGolden(t *testing.T, name string) {
	t.Helper()
	golden, err := os.ReadFile("testdata/" + name + ".golden.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(golden))
	if err != nil {
		t.Fatal(err)
	}
	if err := pageSchema(t).Validate(v); err != nil {
		t.Errorf("%s: %v", name, err)
	}
	if !bytes.Equal(compact(t, golden), compact(t, buildShort(t, shortScenario(name)))) {
		t.Errorf("%s: the page differs from its golden file", name)
	}
}

func TestShortListValidatesSchema(t *testing.T) {
	t.Parallel()
	checkShortGolden(t, "shortlist_ok")
}

func TestShortListGapsGolden(t *testing.T) {
	t.Parallel()
	checkShortGolden(t, "shortlist_gaps")
}
