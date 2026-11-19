package option_candidates_page

import (
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// The Japanese short-put group (candidates-jp.md Decisions 12, 18, 22 to 26).

const (
	colSecuredID    = "cash_secured"
	colSecuredLabel = "Cash to secure"
	jpZone          = "Asia/Tokyo"
)

// jpSpec is the short-put group of the yen rows: the columns of the US group, read in yen.
var jpSpec = groupSpec{kind: kindShortPut, main: groupSpecs[0].main, currency: currencyJPY}

// tokyoDay is the calendar day of t in Asia/Tokyo; Japan has no summer time, so a zone database that cannot be
// loaded falls back to the fixed +09:00 offset.
func tokyoDay(t time.Time) contracts.Day {
	zone, err := time.LoadLocation(jpZone)
	if err != nil {
		zone = time.FixedZone("JST", 9*60*60)
	}
	return contracts.Day(t.In(zone).Format(dayLayout))
}

// jpNotComputed is the gap of a figure that needs the US cash rate or the USD margin rule.
func jpNotComputed() gap { return mkGap("no_data", jpNotComputedZh) }

// limitHeads are the Chinese sentence starts of the limits the JP run counts, in page order.
var limitHeads = []struct{ code, head string }{
	{"wide_spread", limitHeadWideZh},
	{"low_open_interest", limitHeadLowOIZh},
	{"small_bid_size", limitHeadSmallBidZh},
	{"no_bid", limitHeadNoBidZh},
	{"no_quote", limitHeadNoQuoteZh},
}

type limitCount struct {
	Limit string `json:"limit"`
	Count int    `json:"count"`
	Text  string `json:"text"`
}

// limitCountsOf counts the stored blocked rows by the limit that blocked them, one entry per limit even at
// zero: a row counts once under each of its blocking reasons. Rows whose gates cannot be read count nowhere.
func limitCountsOf(rows []store.Candidate) []limitCount {
	n := map[string]int{}
	for _, c := range rows {
		if c.Status != "blocked" {
			continue
		}
		gates, err := parseGates(c.Gates)
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, g := range gates {
			if g.Status == "block" && !seen[g.Reason] {
				seen[g.Reason] = true
				n[g.Reason]++
			}
		}
	}
	out := make([]limitCount, len(limitHeads))
	for i, l := range limitHeads {
		out[i] = limitCount{Limit: l.code, Count: n[l.code], Text: l.head + strconv.Itoa(n[l.code]) + limitTailZh}
	}
	return out
}

// securedCell is the cash that secures the put: strike times the stored multiplier, written with the
// multiplier used, "¥1,790,000 = ¥17,900 x 100".
func securedCell(c store.Candidate) any {
	if c.Multiplier <= 0 {
		return mkGap("missing_input", jpNoMultiplierZh)
	}
	product, ok := timesInt(c.Strike, c.Multiplier)
	if !ok {
		return unreadableCell()
	}
	total, ok1 := moneyText(product, currencyJPY)
	strike, ok2 := moneyText(c.Strike, currencyJPY)
	if !ok1 || !ok2 {
		return unreadableCell()
	}
	cell := textCell(total + " = " + strike + " x " + strconv.Itoa(c.Multiplier))
	if v, ok := numberLiteral(product); ok {
		cell.Value = &v
	}
	return cell
}

// yenColumns are the columns of the US short-put group with the money columns written in yen and the cash to
// secure added; the figures that need the US cash rate are the gap "not computed".
func yenColumns(defs []colDef) []colDef {
	out := make([]colDef, 0, len(defs)+1)
	for _, d := range defs {
		switch d.ID {
		case "strike":
			d.cell = func(c store.Candidate) any { return moneyCellIn(c.Strike, currencyJPY) }
		case "bid":
			d.cell = func(c store.Candidate) any { return moneyCellIn(c.BidEod, currencyJPY) }
		case "break_even":
			d.cell = func(c store.Candidate) any { return moneyCellIn(c.BreakEven, currencyJPY) }
		case itmID, excessID:
			d.cellAt = func(int, store.Candidate) any { return jpNotComputed() }
		}
		out = append(out, d)
		if d.ID == "break_even" {
			secured := col(colSecuredID, colSecuredLabel, "right", securedCell).hinted(jpSecuredHintZh)
			out = append(out, secured)
		}
	}
	return out
}
