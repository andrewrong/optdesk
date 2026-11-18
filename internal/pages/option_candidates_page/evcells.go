package option_candidates_page

import (
	"strings"

	"github.com/andrewrong/optdesk/internal/store"
)

const (
	colBelowBuyID = "below_buy"
	colOneSigmaID = "one_sigma"
	colBidFcstID  = "bid_vs_forecast"
	colSkewID     = "skew"
	colForecastID = "forecast_vol"
	colIVPID      = "ivp"
	colVolEdgeID  = "vol_edge"
	colLabelsID   = "labels"
)

// evidenceCols are the eight evidence columns of a short-put group, with the data days in their hints.
func evidenceCols(set evidenceSet) []colDef {
	day := string(set.chainDay)
	rateDay := string(set.rateDay)
	if rateDay == "" {
		rateDay = evNoRateDay
	}
	withDay := func(h string) string { return h + evDayHead + day }
	cell := func(f func(rowEvidence, store.Candidate) any) func(int, store.Candidate) any {
		return evCell(set, f)
	}
	mk := func(id, label, hint string, f func(rowEvidence, store.Candidate) any) colDef {
		d := colDef{column: column{ID: id, Label: label, Align: "right"}, cellAt: cell(f)}
		d.Hint = &hint
		return d
	}
	bid := evBidFcstHintHead + trialRateSentence + evBidFcstDays + day + evBidFcstRate + rateDay
	return []colDef{
		mk(colBelowBuyID, evBelowBuyLabel, withDay(evBelowBuyHint), func(r rowEvidence, _ store.Candidate) any {
			return belowBuyCell(r.ev.BelowBuy, r.ev.BelowBuyPct, r.ev.BelowBuyWhy)
		}),
		mk(colOneSigmaID, evOneSigmaLabel, withDay(evOneSigmaHint), func(r rowEvidence, _ store.Candidate) any { return sigmaCell(r) }),
		mk(colBidFcstID, evBidFcstLabel, bid, func(r rowEvidence, _ store.Candidate) any { return bidForecastCell(r, string(set.chainDay)) }),
		mk(colSkewID, evSkewLabel, withDay(evSkewHint), func(r rowEvidence, _ store.Candidate) any { return skewCell(r) }),
		mk(colForecastID, evForecastLabel, evForecastHint+evDayHead+evBadgeDay, func(r rowEvidence, _ store.Candidate) any { return forecastCell(r) }),
		mk(colIVPID, evIVPLabel, evIVPHint+evDayHead+evBadgeDay, func(r rowEvidence, _ store.Candidate) any { return ivpCell(r) }),
		mk(colVolEdgeID, evVolEdgeLabel, evVolEdgeHint+evDayHead+evBadgeDay, func(r rowEvidence, _ store.Candidate) any { return volEdgeCell(r) }),
		mk(colLabelsID, evLabelsLabel, withDay(evLabelsHint), func(r rowEvidence, c store.Candidate) any { return labelsCell(r, c) }),
	}
}

// evCell reads the evidence of the row at position i; a row with no evidence, no spot, no reader or
// an unreadable answer has the same gap in every column.
func evCell(set evidenceSet, f func(rowEvidence, store.Candidate) any) func(int, store.Candidate) any {
	return func(i int, c store.Candidate) any {
		if i < 0 || i >= len(set.rows) {
			return mkGap("no_reader", gapNoReader)
		}
		r := set.rows[i]
		switch r.state {
		case evNoSpot:
			return mkGap("no_data", gapNoSpot)
		case evNoReader:
			return mkGap("no_reader", gapNoReader)
		case evUnreadable:
			return unreadableCell()
		case evOK:
		}
		return f(r, c)
	}
}

// whyGap is the gap of a figure the Evaluator could not form, by its Why code; word is the reason word
// of the forecast reading and is added to no_forecast. A code with no text is unreadable.
func whyGap(why, word string) any {
	switch why {
	case "no_buy_price":
		return mkGap("no_data", gapNoBuy)
	case "no_iv":
		return mkGap("missing_input", gapNotStored)
	case "no_forecast":
		text := gapNoForecast
		if word != "" {
			text += forecastBrOpen + word + ")"
		}
		return mkGap("missing_input", text)
	case "no_rate":
		return mkGap("no_data", gapNoRate)
	case "no_fair_price":
		return mkGap("no_data", gapNoFair)
	case "no_atm":
		return mkGap("no_data", gapNoATM)
	}
	return unreadableCell()
}

// valueOf is a cell of text with the decimal text key as its sort value.
func valueOf(text, key string, badges ...string) any {
	cell := figValueCell(text, key)
	if vc, ok := cell.(valueCell); ok {
		vc.Badges = append(vc.Badges, badges...)
		return vc
	}
	return cell
}

// signedMoney is money with a leading + for an amount that is neither zero nor negative, U+2212 for a negative one.
func signedMoney(text string) (string, bool) {
	m, ok := moneyText(text)
	if !ok {
		return "", false
	}
	if strings.HasPrefix(m, "-") {
		return minusText(m), true
	}
	if m == "$0.00" {
		return m, true
	}
	return "+" + m, true
}

// signedFixed is the decimal text with places decimals and the sign rule of signedMoney.
func signedFixed(text string, places int) (string, bool) {
	f, ok := fixedText(text, places)
	if !ok {
		return "", false
	}
	if strings.HasPrefix(f, "-") {
		return minusText(f), true
	}
	if strings.Trim(f, "0.") == "" {
		return f, true
	}
	return "+" + f, true
}

// pctText1 is a compute percent field with one decimal and the percent sign.
func pctText1(text string) (string, bool) {
	f, ok := fixedText(text, 1)
	return minusText(f) + "%", ok
}

func belowBuyCell(amount, pct, why string) any {
	if why != "" {
		return whyGap(why, "")
	}
	m, ok1 := moneyText(amount)
	p, ok2 := pctText1(pct)
	if !ok1 || !ok2 {
		return unreadableCell()
	}
	return valueOf(minusText(m)+" ("+p+")", amount)
}

func sigmaCell(r rowEvidence) any {
	e := r.ev
	if e.MoveWhy != "" {
		return whyGap(e.MoveWhy, "")
	}
	m, ok1 := moneyText(e.OneSigma)
	p, ok2 := pctText1(e.OneSigmaPct)
	if !ok1 || !ok2 {
		return unreadableCell()
	}
	var badges []string
	if s, ok := fixedText(e.BreakEvenInSigmas, 1); ok {
		badges = append(badges, s+evSigmaBadge)
	}
	return valueOf(m+" ("+p+")", e.OneSigma, badges...)
}

func bidForecastCell(r rowEvidence, chainDay string) any {
	e := r.ev
	if e.EdgeWhy != "" {
		return whyGap(e.EdgeWhy, forecastWord(r))
	}
	share, ok1 := signedMoney(e.EdgePerShare)
	contract, ok2 := signedMoney(e.EdgePerContract)
	fair, ok3 := moneyText(e.Fair)
	if !ok1 || !ok2 || !ok3 {
		return unreadableCell()
	}
	badges := []string{evModelPrice + fair}
	if d := r.rd.forecast.day; d != "" && d != chainDay {
		badges = append(badges, evAsOf+d)
	}
	return valueOf(share+evPerShare+contract+evPerContract, e.EdgePerShare, badges...)
}

// forecastWord is the reason word placed after the no-forecast text: the missing reading's reason, or
// the word of the missing reader.
func forecastWord(r rowEvidence) string {
	if r.rd.noReader != "" {
		return readNoReader
	}
	return r.rd.forecast.word
}

func skewCell(r rowEvidence) any {
	e := r.ev
	if r.atmNoReader {
		return mkGap("no_reader", gapNoReader)
	}
	if e.SkewWhy != "" {
		return whyGap(e.SkewWhy, "")
	}
	pts, ok := signedFixed(e.SkewPct, 1)
	if !ok {
		return unreadableCell()
	}
	var badges []string
	if iv, ok := fixedText(e.ATMIVPct, 1); ok && r.atmStrike != "" {
		badges = append(badges, evATMBadge+iv+"%"+evATMAt+r.atmStrike)
	}
	return valueOf(pts+evPts, e.SkewPct, badges...)
}

// readingGap is the gap of a reading cell with no ok reading, or nil when there is one.
func readingGap(rd nameReadings, one reading, withText bool) any {
	if rd.noReader != "" {
		return mkGap("no_reader", rd.noReader)
	}
	if one.text != "" {
		return nil
	}
	if withText {
		return whyGap("no_forecast", one.word)
	}
	return mkGap("missing_input", one.word)
}

func asOf(one reading) []string { return []string{evAsOf + one.day} }

func forecastCell(r rowEvidence) any {
	if g := readingGap(r.rd, r.rd.forecast, true); g != nil {
		return g
	}
	p, ok := pctText1(r.ev.ForecastPct)
	if !ok {
		return unreadableCell()
	}
	badges := []string{}
	if e, ok := pctText1(r.ev.ForecastErrPct); ok {
		badges = append(badges, evLastError+e)
	}
	return valueOf(p, r.ev.ForecastPct, append(badges, asOf(r.rd.forecast)...)...)
}

func ivpCell(r rowEvidence) any {
	if g := readingGap(r.rd, r.rd.ivp, false); g != nil {
		return g
	}
	p, ok := pctText1(r.ev.IVPPct)
	if !ok {
		return unreadableCell()
	}
	return valueOf(p, r.ev.IVPPct, asOf(r.rd.ivp)...)
}

func volEdgeCell(r rowEvidence) any {
	if g := readingGap(r.rd, r.rd.volEdge, false); g != nil {
		return g
	}
	pts, ok := signedFixed(r.ev.VolEdgePct, 1)
	if !ok {
		return unreadableCell()
	}
	return valueOf(pts+evPts, r.ev.VolEdgePct, asOf(r.rd.volEdge)...)
}

func labelsCell(r rowEvidence, c store.Candidate) any {
	cell := textCell(dash)
	for _, l := range r.ev.Labels {
		if l == "delta_outside_band" && c.Delta == nil {
			continue
		}
		if t, ok := labelTexts[l]; ok {
			l = t
		}
		cell.Badges = append(cell.Badges, l)
	}
	if c.Delta == nil {
		cell.Badges = append(cell.Badges, evDeltaNone)
	}
	return cell
}
