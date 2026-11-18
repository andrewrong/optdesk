package option_candidates_page

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/store"
)

var han = regexp.MustCompile(`[\p{Han}]`)

// amberRows is a Fits row and a Check row stored as blocked by the quote gate.
func amberRows() []store.Candidate {
	rows := []store.Candidate{tierPut(1, "ok", "85.00", "0.42"), tierPut(2, "blocked", "87.50", "0.60")}
	rows[1].Gates = `[{"gate":"quote","status":"block","reason":"wide_spread"}]`
	return rows
}

func TestFieldsHaveEvidenceForBlockedAmber(t *testing.T) {
	t.Parallel()
	rows := amberRows()
	ev := &fakeEval{}
	c := tierConfig(rows, &fakeTierer{res: resultOf(fitsVerdict(keyOf(rows, 0)), checkVerdict(keyOf(rows, 1), "wide_quote"))})
	c.Evidence = ev
	view := buildTiersOf(t, c)
	r := tierByID(view, "check").Rows[0]
	for _, id := range []string{"below_buy", "one_sigma", "bid_vs_forecast", "skew", "forecast_vol", "ivp", "vol_edge", "labels"} {
		f := fieldByID(r, id)
		if f == nil || (f.Value == nil && f.Gap == nil) {
			t.Errorf("field %s = %+v", id, f)
		}
	}
	if f := fieldByID(r, "one_sigma"); f == nil || deref(f.Value) == "" {
		t.Errorf("one_sigma = %+v", f)
	}
	var strikes []string
	for _, r := range ev.rows {
		strikes = append(strikes, r.Strike)
	}
	if !slices.Contains(strikes, "87.50") {
		t.Errorf("Evaluate was not asked about the blocked amber row: %v", strikes)
	}
}

func TestExpansionNoDeltaOutsideBand(t *testing.T) {
	t.Parallel()
	rows := amberRows()
	ev := &fakeEval{fn: func(optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
		e := fullEvidence()
		e.Labels = []string{"delta_outside_band", "ex_div_inside_life"}
		return e, nil
	}}
	c := tierConfig(rows, &fakeTierer{res: resultOf(fitsVerdict(keyOf(rows, 0)), checkVerdict(keyOf(rows, 1), "wide_quote"))})
	c.Evidence = ev
	view := buildTiersOf(t, c)
	for _, id := range []string{"fits", "check"} {
		f := fieldByID(tierByID(view, id).Rows[0], "labels")
		if f == nil {
			t.Fatalf("%s: no labels field", id)
		}
		var got []string
		for _, b := range f.Badges {
			got = append(got, b.Label)
		}
		all := strings.Join(got, "|")
		if strings.Contains(all, "delta outside") {
			t.Errorf("%s: the expansion carries delta_outside_band: %v", id, got)
		}
		if !strings.Contains(all, "ex-dividend inside the life") {
			t.Errorf("%s: other labels were lost: %v", id, got)
		}
	}
}

func TestBadgeGlossIsChinese(t *testing.T) {
	t.Parallel()
	for code := range badgeTexts {
		b := badgeOf(code, badgeToneNone, store.Candidate{})
		if han.MatchString(b.Label) || b.Label == "" {
			t.Errorf("%s: label %q", code, b.Label)
		}
		if !han.MatchString(b.Gloss) {
			t.Errorf("%s: gloss %q has no Chinese character", code, b.Gloss)
		}
	}
	u := badgeOf("brand_new_code", badgeToneInfo, store.Candidate{})
	if u.Label != "brand_new_code" || !han.MatchString(u.Gloss) {
		t.Errorf("unknown code = %+v", u)
	}
}

func TestHintsEndWithDataDay(t *testing.T) {
	t.Parallel()
	rows := amberRows()
	c := tierConfig(rows, &fakeTierer{res: resultOf(fitsVerdict(keyOf(rows, 0)), checkVerdict(keyOf(rows, 1), "wide_quote"))})
	c.Account, c.Aliases = &fakeAccount{values: cashAt("A", "2026-11-09", "120000.00")}, []string{"A"}
	view := buildTiersOf(t, c)
	tail := "；数据日 2026-11-09"
	for _, col := range view.Columns {
		if col.Hint != nil && !strings.HasSuffix(*col.Hint, tail) {
			t.Errorf("column %s hint %q", col.ID, *col.Hint)
		}
	}
	r := tierByID(view, "fits").Rows[0]
	for i, f := range r.Fields {
		if f.Hint == nil {
			t.Errorf("field %d %s has no hint", i, f.ID)
			continue
		}
		dated := strings.Contains(*f.Hint, "2026-11-09") || strings.Contains(*f.Hint, "data day")
		if utf8.RuneCountInString(*f.Hint) > 200 || !dated {
			t.Errorf("field %d %s hint %v", i, f.ID, f.Hint)
		}
	}
	for _, id := range []string{"iv", "delta", "itm_probability", "vs_cash", "account_cash", "bid_per_share"} {
		if f := fieldByID(r, id); f == nil || !strings.HasSuffix(*f.Hint, tail) {
			t.Errorf("field %s = %+v", id, f)
		}
	}
}
