package option_candidates_page

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

func legendText(v *tiersView, tier string) string {
	for _, l := range v.Legend {
		if l.Tier == tier {
			return l.Text
		}
	}
	return ""
}

func TestLegendPrintsCurrentValues(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42"), tierPut(2, "ok", "90.00", "0.94")}
	res := resultOf(fitsVerdict(keyOf(rows, 0)), greyVerdict(keyOf(rows, 1), "in_the_money"))
	res.Params = optiontierapi.Params{DeltaMax: "0.25", ThinDelta: "0.12", MaxCashAgeDays: 3, DaysInYear: 365}
	c := tierConfig(rows, &fakeTierer{res: res})
	c.Owner = fakeOwner{params: []ownerset.Param{{Key: "put_delta_max", Value: "0.25"}}}
	c.RuleParams = fakeRules{}
	c.Account, c.Aliases = &fakeAccount{values: cashAt("A", "2026-11-06", "5000.00")}, []string{"A"}
	view := buildTiersOf(t, c)
	if len(view.Legend) != 4 {
		t.Fatalf("legend = %+v", view.Legend)
	}
	fits := legendText(view, "fits")
	for _, want := range []string{"|delta| 不高于 0.25（试用值，可在设置里改）", "|delta| 低于 0.12 "} {
		if !strings.Contains(fits, want) {
			t.Errorf("fits text %q lacks %q", fits, want)
		}
	}
	if !strings.HasPrefix(view.Legend[0].Title, "✓ Fits (1)") || !strings.HasSuffix(view.Legend[2].Title, "(1)") {
		t.Errorf("titles = %+v", view.Legend)
	}
	cash := legendText(view, "cash")
	if !strings.Contains(cash, "截至 2026-11-06") || !strings.Contains(cash, "最多早 3 天") {
		t.Errorf("cash text = %q", cash)
	}
}

func TestLegendDefaultBound(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	view := buildTiersOf(t, tierConfig(rows, &fakeTierer{res: resultOf(fitsVerdict(keyOf(rows, 0)))}))
	if fits := legendText(view, "fits"); !strings.Contains(fits, "0.3（默认值）") || strings.Contains(fits, "试用值") {
		t.Errorf("fits text = %q", fits)
	}
}

func TestLegendCashClause(t *testing.T) {
	t.Parallel()
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42")}
	// the judge says the cash figure is built only when an account was passed
	judge := func(built bool) *fakeTierer {
		return &fakeTierer{fn: func(in optiontierapi.Input) optiontierapi.Result {
			r := resultOf(fitsVerdict(keyOf(rows, 0)))
			r.CashBuilt = built && len(in.Accounts) > 0
			return r
		}}
	}
	built := tierConfig(rows, judge(true))
	built.Account, built.Aliases = &fakeAccount{values: cashAt("A", "2026-11-09", "1.00")}, []string{"A"}
	v := buildTiersOf(t, built)
	if fits := legendText(v, "fits"); !strings.Contains(fits, "现金够付全额行权价") || strings.Contains(fits, "只做标记") {
		t.Errorf("built: fits text %q", fits)
	}
	if cash := legendText(v, "cash"); !strings.Contains(cash, "截至 2026-11-09") {
		t.Errorf("built: cash text %q", cash)
	}

	f := judge(true)
	none := tierConfig(rows, f)
	none.Aliases = []string{"A"} // no Account reader
	v = buildTiersOf(t, none)
	if len(f.in[0].Accounts) != 0 {
		t.Fatalf("accounts passed without a reader: %+v", f.in[0].Accounts)
	}
	if fits := legendText(v, "fits"); !strings.Contains(fits, "现金只做标记") || strings.Contains(fits, "现金够付") {
		t.Errorf("mark: fits text %q", fits)
	}
	if cash := legendText(v, "cash"); !strings.Contains(cash, "cash_unknown") {
		t.Errorf("mark: cash text %q", cash)
	}
}
