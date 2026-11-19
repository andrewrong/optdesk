package option_candidates_page

import (
	"fmt"
	"strconv"

	"github.com/andrewrong/optdesk/internal/compute/optionshortlistapi"
	"github.com/andrewrong/optdesk/internal/store"
)

type filterView struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value string `json:"value"`
	Hint  string `json:"hint"`
}

type droppedItem struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Count string `json:"count"`
	Hint  string `json:"hint"`
}

// shortListView is the property short_put_list (candidates-page-v2.md Interface).
type shortListView struct {
	RunDay       string        `json:"run_day"`
	Counts       string        `json:"counts"`
	Caveat       *string       `json:"caveat"`
	Filters      []filterView  `json:"filters"`
	Dropped      *string       `json:"dropped"`
	DroppedItems []droppedItem `json:"dropped_items"`
	Columns      []tierColumn  `json:"columns"`
	Groups       []shortGroup  `json:"groups"`
	FoldedTitle  string        `json:"folded_title"`
}

// shortListOfKind is the list of the short-put group, nil for any other kind, without a Shortlist reader or
// without rows. It calls Select once; an error of Select is returned. tiers may be nil.
func shortListOfKind(c Config, kind string, gr groupRead, tiers *tiersView) (*shortListView, error) {
	if kind != kindShortPut || c.Shortlist == nil || len(gr.rows) == 0 {
		return nil, nil
	}
	res, err := c.Shortlist.Select(shortlistInputOf(gr.rows, gr.tickers))
	if err != nil {
		return nil, fmt.Errorf("option_candidates_page: shortlist: %w", err)
	}
	v := &shortListView{
		RunDay:       slRunDayHead + string(gr.day),
		Counts:       countsLine(res),
		Caveat:       caveatOf(res.StaleChain),
		Filters:      filtersOf(res),
		DroppedItems: []droppedItem{},
		Columns:      shortColumns(),
		Groups:       shortGroupsOf(res, gr.rows, tiers),
		FoldedTitle:  slFoldedHead + thousands(len(gr.rows)) + slFoldedTail,
	}
	v.Dropped, v.DroppedItems = droppedOf(res)
	return v, nil
}

func countsLine(res optionshortlistapi.Result) string {
	return slScanned + thousands(res.Scanned) + slPutsSep + thousands(res.Shown) + slRows +
		strconv.Itoa(len(res.Groups)) + slUnderlyings
}

func caveatOf(stale int) *string {
	if stale == 0 {
		return nil
	}
	s := zhCaveatHead + strconv.Itoa(stale) + zhCaveatTail
	return &s
}

// thousands is n with a comma between each three digits.
func thousands(n int) string {
	s := strconv.Itoa(n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 && s[i-1] != '-' {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	return string(out)
}

func rangeText(lo, hi, suffix string) string {
	if lo == "" || hi == "" {
		return dash
	}
	return lo + slRange + hi + suffix
}

// filtersOf are the six chips. The presence chips are fixed text: Bounds has no field for them.
func filtersOf(res optionshortlistapi.Result) []filterView {
	b := res.Bounds
	return []filterView{
		{slChipExpiryID, slChipExpiryLabel, rangeText(res.DTEMin, res.DTEMax, slDaysSuffix), zhHintExpiry},
		{slChipStrikeID, slChipStrikeLabel, rangeText(b.OTMMinPct, b.OTMMaxPct, slPct), zhHintStrike},
		{slChipDeltaID, slChipDeltaLabel, rangeText(b.DeltaMin, b.DeltaMax, ""), zhHintDelta},
		{slChipCapID, slChipCapLabel, strconv.Itoa(b.PerUnderlying), zhHintCap},
		{slChipBidID, slChipBidLabel, slChipPresent, zhHintBidPresent},
		{slChipOIID, slChipOILabel, slChipPresent, zhHintOIPresent},
	}
}

func shortColumns() []tierColumn {
	col := func(id, label, hint string) tierColumn { return tierColumn{ID: id, Label: label, Hint: &hint} }
	return []tierColumn{
		col(slColStrikeID, slColStrikeLabel, zhColHintStrike), col(slColExpiryID, slColExpiryLabel, zhColHintExpiry),
		col(slColOTMID, slColOTMLabel, zhColHintOTM), col(slColDeltaID, slColDeltaLabel, zhColHintDelta),
		col(slColPremiumID, slColPremiumLabel, zhColHintPremium),
		col(slColAnnualizedID, slColAnnualizedLabel, zhColHintAnnualized),
		col(slColBuyID, slColBuyLabel, zhColHintBuy), col(slColLiquidityID, slColLiquidityLabel, zhColHintLiquidity),
	}
}

// droppedOf is the "Not shown K" line and one item per code of the result in its order; a zero count is not
// written, and no line is written when nothing was dropped.
func droppedOf(res optionshortlistapi.Result) (*string, []droppedItem) {
	items := []droppedItem{}
	total := 0
	for _, d := range res.Dropped {
		if d.N == 0 {
			continue
		}
		total += d.N
		hint, ok := dropHints[d.Code]
		if !ok {
			hint = zhDropUnknown
		}
		items = append(items, droppedItem{Code: d.Code, Label: dropLabel(d.Code, res.Bounds), Count: strconv.Itoa(d.N), Hint: hint})
	}
	if total == 0 {
		return nil, items
	}
	line := slDroppedHead + strconv.Itoa(total)
	return &line, items
}

func dropLabel(code string, b optionshortlistapi.Bounds) string {
	switch code {
	case "otm_low":
		return slDropOTMLow + b.OTMMinPct + slPct
	case "otm_high":
		return slDropOTMHigh + b.OTMMaxPct + slPct
	case "over_cap":
		return slDropOverHead + strconv.Itoa(b.PerUnderlying) + slDropOverTail
	}
	if l, ok := dropLabels[code]; ok {
		return l
	}
	return code
}

// shortGroupsOf writes the groups of the result in its order, each row with its stored row and, when there is
// one, its tier row. A placed row with no stored row is not shown.
func shortGroupsOf(res optionshortlistapi.Result, rows []store.Candidate, tiers *tiersView) []shortGroup {
	byKey := make(map[string]store.Candidate, len(rows))
	for i, r := range rows {
		byKey[tierKey(r, i)] = r
	}
	tierRows := tierRowsOf(tiers)
	out := make([]shortGroup, 0, len(res.Groups))
	for _, g := range res.Groups {
		sg := shortGroup{Rows: []shortRow{}}
		for _, p := range g.Rows {
			c, ok := byKey[p.Key]
			if !ok {
				continue
			}
			if len(sg.Rows) == 0 {
				sg.Ticker, sg.Spot = c.Issuer, spotText(c)
			}
			var t *tierRow
			if tr, ok := tierRows[p.Key]; ok {
				t = &tr
			}
			sg.Rows = append(sg.Rows, shortRowOf(p, c, t))
		}
		if len(sg.Rows) == 0 {
			continue
		}
		sg.Summary = strconv.Itoa(len(sg.Rows)) + rowWord(len(sg.Rows)) + slBest + bestText(g.Best)
		out = append(out, sg)
	}
	return out
}

func spotText(c store.Candidate) string {
	if m, ok := moneyText(deref(c.Spot)); ok {
		return m
	}
	return noSpotText
}

func bestText(best string) string {
	if p, ok := percentText(best); ok {
		return p
	}
	return unreadable
}

// tierRowsOf indexes the rows of the tier view by key; nil gives an empty index.
func tierRowsOf(v *tiersView) map[string]tierRow {
	out := map[string]tierRow{}
	if v == nil {
		return out
	}
	for _, t := range v.Tiers {
		for _, r := range t.Rows {
			out[r.ID] = r
		}
	}
	return out
}

func rowWord(n int) string {
	if n == 1 {
		return slRowOne
	}
	return slRowMany
}
