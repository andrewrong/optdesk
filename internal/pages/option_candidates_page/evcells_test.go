package option_candidates_page

import (
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/store"
)

func asValue(t *testing.T, cell any) valueCell {
	t.Helper()
	vc, ok := cell.(valueCell)
	if !ok {
		t.Fatalf("cell = %+v, want a value cell", cell)
	}
	return vc
}

func asGap(t *testing.T, cell any) gap {
	t.Helper()
	g, ok := cell.(gap)
	if !ok {
		t.Fatalf("cell = %+v, want a gap", cell)
	}
	return g
}

func TestSigmaCellText(t *testing.T) {
	t.Parallel()
	vc := asValue(t, sigmaCell(rowEvidence{ev: optionevidenceapi.Evidence{OneSigma: "8.5775", OneSigmaPct: "8.8886", BreakEvenInSigmas: "0.8674"}}))
	if vc.Text != "$8.58 (8.9%)" || !slices.Equal(vc.Badges, []string{"0.9 sigma to break-even"}) {
		t.Errorf("cell = %+v", vc)
	}
}

func TestSignedMoneyText(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"0.21": "+$0.21", "0": "$0.00", "0.004": "$0.00", "-0.21": "−$0.21", "-0.004": "$0.00", "21234.5": "+$21,234.50", "-1234.5": "−$1,234.50",
	} {
		if got, ok := signedMoney(in); !ok || got != want {
			t.Errorf("signedMoney(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBelowBuyText(t *testing.T) {
	t.Parallel()
	if vc := asValue(t, belowBuyCell("0.94", "1.0444", "")); vc.Text != "$0.94 (1.0%)" {
		t.Errorf("text = %q", vc.Text)
	}
	if vc := asValue(t, belowBuyCell("-0.94", "-1.0444", "")); vc.Text != "−$0.94 (−1.0%)" {
		t.Errorf("negative text = %q", vc.Text)
	}
}

func TestSkewUnitText(t *testing.T) {
	t.Parallel()
	vc := asValue(t, skewCell(rowEvidence{ev: optionevidenceapi.Evidence{SkewPct: "1.1000", ATMIVPct: "29.4000"}, atmStrike: "95.00"}))
	if vc.Text != "+1.1 pts" || !slices.Equal(vc.Badges, []string{"ATM IV 29.4% at 95.00"}) {
		t.Errorf("cell = %+v", vc)
	}
	if vc := asValue(t, skewCell(rowEvidence{ev: optionevidenceapi.Evidence{SkewPct: "-0.0400"}})); vc.Text != "0.0 pts" {
		t.Errorf("zero = %q", vc.Text)
	}
}

func TestEmptyDeltaNotStored(t *testing.T) {
	t.Parallel()
	r := rowEvidence{ev: optionevidenceapi.Evidence{Labels: []string{"delta_outside_band"}}}
	vc := asValue(t, labelsCell(r, store.Candidate{}))
	if !slices.Contains(vc.Badges, "delta not stored") || slices.Contains(vc.Badges, "delta outside the trial band") {
		t.Errorf("badges = %v", vc.Badges)
	}
}

func TestEmptyIVNotStored(t *testing.T) {
	t.Parallel()
	r := rowEvidence{ev: optionevidenceapi.Evidence{MoveWhy: "no_iv", SkewWhy: "no_iv"}}
	for _, cell := range []any{sigmaCell(r), skewCell(r)} {
		if g := asGap(t, cell); g.Text != "not stored" || g.Reason != "missing_input" {
			t.Errorf("gap = %+v", g)
		}
	}
}

func TestWhyTexts(t *testing.T) {
	t.Parallel()
	for _, w := range []struct{ why, reason, text string }{
		{"no_buy_price", "no_data", "No buy price set"},
		{"no_iv", "missing_input", "not stored"},
		{"no_forecast", "missing_input", "No forecast volatility stored (short_history)"},
		{"no_rate", "no_data", "No cash rate within 7 days (DGS3MO)"},
		{"no_fair_price", "no_data", "The model price could not be made"},
		{"no_atm", "no_data", "no ATM quote"},
		{"zero_dte", "unreadable", unreadable},
		{"invented", "unreadable", unreadable},
	} {
		if g := asGap(t, whyGap(w.why, "short_history")); g.Reason != w.reason || g.Text != w.text {
			t.Errorf("%s = %+v", w.why, g)
		}
	}
}

func TestHintsNameDataDay(t *testing.T) {
	t.Parallel()
	for _, c := range evidenceCols(evidenceSet{chainDay: "2026-11-17", rateDay: "2026-11-16"}) {
		h := *c.Hint
		if len(h) > 200 {
			t.Errorf("%s hint has %d characters", c.ID, len(h))
		}
		switch c.ID {
		case "forecast_vol", "ivp", "vol_edge":
			if !strings.HasSuffix(h, "data day in the badge") {
				t.Errorf("%s hint = %q", c.ID, h)
			}
		case "bid_vs_forecast":
			if !strings.HasSuffix(h, "chain 2026-11-17, rate 2026-11-16") {
				t.Errorf("%s hint = %q", c.ID, h)
			}
		default:
			if !strings.HasSuffix(h, "data day 2026-11-17") {
				t.Errorf("%s hint = %q", c.ID, h)
			}
		}
	}
}

func TestLongestHintText(t *testing.T) {
	t.Parallel()
	want := "Bid minus a model price from the forecast vol (a proxy, sign often wrong; European, no dividends). " +
		"DGS3MO is the cash rate for every currency (a trial). Data days: chain 2026-11-17, rate 2026-11-16"
	for _, c := range evidenceCols(evidenceSet{chainDay: "2026-11-17", rateDay: "2026-11-16"}) {
		if c.ID == "bid_vs_forecast" && (*c.Hint != want || len(*c.Hint) != 197) {
			t.Errorf("hint = %q (%d)", *c.Hint, len(*c.Hint))
		}
	}
}

func TestLabelsNeverGate(t *testing.T) {
	t.Parallel()
	labelled := func(labels []string) outPage {
		f := &fakeEval{fn: func(optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
			e := fullEvidence()
			e.Labels = labels
			return e, nil
		}}
		return buildOut(t, evConfig([]store.Candidate{okPut(2), okPut(1)}, f), nil)
	}
	with := labelled([]string{"no_edge_at_bid", "delta_outside_band", "ex_div_inside_life"})
	without := labelled([]string{})
	want := []string{"bid not above the forecast price", "delta outside the trial band", "ex-dividend inside the life"}
	if got := firstRow(with).Cells["labels"].Badges; !slices.Equal(got, want) {
		t.Errorf("badges = %v", got)
	}
	a, b := with.Groups[0], without.Groups[0]
	if !slices.Equal(rowIDs(a.Main), rowIDs(b.Main)) || a.Counts != b.Counts || with.RunStatus != without.RunStatus {
		t.Errorf("labels changed the rows: %v / %v", rowIDs(a.Main), rowIDs(b.Main))
	}
	for i := range a.Main.Rows {
		if a.Main.Rows[i].Cells["rank"].Text != b.Main.Rows[i].Cells["rank"].Text {
			t.Errorf("rank of row %d changed", i)
		}
	}
}
