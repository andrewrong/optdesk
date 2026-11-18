package valueimport

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelines"
)

func TestRefusesRowOffByMoreThanRounding(t *testing.T) {
	t.Parallel()
	b := mutate(t, func(r map[string]any) { company0(r, 2)["B_hi"] = num("1300") }) // site 1280, diff 20 > 0.5% and 0.5
	row := rowByName(t, plan(t, b, listState(), nil), "SynthCedar")
	if row.Action != valueimportapi.ActionRefused || row.Reason != "the site computes another buy-zone top" ||
		row.SiteBuyTop != "1280" || row.FileBuyTop != "1300" {
		t.Fatalf("row %+v", row)
	}
}

func TestAcceptsRowWithinRounding(t *testing.T) {
	t.Parallel()
	b := mutate(t, func(r map[string]any) { company0(r, 2)["B_hi"] = num("1285") }) // diff 5 <= 0.5% of 1285
	if row := rowByName(t, plan(t, b, listState(), nil), "SynthCedar"); row.Action != valueimportapi.ActionInsert {
		t.Fatalf("row %+v", row)
	}
	b = mutate(t, func(r map[string]any) { company0(r, 4)["B_hi"] = num("13.9") }) // diff 0.4 <= absolute 0.5
	if row := rowByName(t, plan(t, b, listState(), nil), "SynthFoxtrot"); row.Action != valueimportapi.ActionInsert {
		t.Fatalf("row %+v", row)
	}
}

func TestRefusesRowWithOtherRThanMarket(t *testing.T) {
	t.Parallel()
	b := mutate(t, func(r map[string]any) { company0(r, 0)["R"] = num("0.91") })
	if row := rowByName(t, plan(t, b, listState(), nil), "SynthAlpha"); row.Action != valueimportapi.ActionRefused {
		t.Fatalf("row %+v", row)
	}
}

func TestRefusesHKCNDifference(t *testing.T) {
	t.Parallel()
	b := mutate(t, func(r map[string]any) { meta(r)["R_table_current"].(map[string]any)["CN"] = num("0.60") })
	p := plan(t, b, listState(), map[string]valueimportapi.Pick{"9001.HK": {Key: "HK:9001", RMarket: "HKCN"}})
	if p.R.Action != valueimportapi.ActionRefused || p.R.Problem != "HK and CN differ: the site has one HKCN column" {
		t.Fatalf("r %+v", p.R)
	}
	if d := rowByName(t, p, "SynthDelta"); d.Action != valueimportapi.ActionRefused {
		t.Fatalf("row %+v", d)
	}
}

func TestRefusesRowWhenCheckFails(t *testing.T) {
	t.Parallel()
	p, err := New().Plan(valueimportapi.Input{File: load(t, "synthetic.json"), Stored: listState(), Calc: failCheck{valuelines.New()}})
	if err != nil {
		t.Fatal(err)
	}
	row := rowByName(t, p, "SynthAlpha")
	if row.Action != valueimportapi.ActionRefused || row.Reason != "synthetic problem" {
		t.Fatalf("row %+v", row)
	}
}

func TestNeedsTickWhenCurrencyDiffers(t *testing.T) {
	t.Parallel()
	b := mutate(t, func(r map[string]any) { company0(r, 0)["currency"] = "EUR" })
	p := plan(t, b, listState(), nil)
	if !rowByName(t, p, "SynthAlpha").NeedsTick || rowByName(t, p, "SynthCedar").NeedsTick {
		t.Fatalf("ticks %+v", p.Rows)
	}
}
