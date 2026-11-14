package option_boards

import (
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// tableColumns are the 12 columns of board 5 in the fixture order.
func tableColumns() []column {
	hint := "IV percentile over the past year, no source yet"
	return []column{
		{ID: "symbol", Label: "Underlying", Align: "left"},
		{ID: "verdict", Label: "Verdict", Align: "left"},
		{ID: "annualised", Label: "Annualised", Align: "right"},
		{ID: "strike_pct", Label: "strike %", Align: "right"},
		{ID: "dte", Label: "DTE", Align: "right"},
		{ID: "delta", Label: "Δ", Align: "right"},
		{ID: "bid_ask", Label: "Bid/Ask", Align: "right"},
		{ID: "spread_pct", Label: "spread %", Align: "right"},
		{ID: "volume", Label: "Vol", Align: "right"},
		{ID: "open_interest", Label: "OI", Align: "right"},
		{ID: "ivp", Label: "IVP", Align: "right", Hint: &hint},
		{ID: "earnings", Label: "E", Align: "left"},
	}
}

func buildTable(rows []store.Candidate, eod map[int]contracts.OptionEod, names displayNames, qs quoteSource) table {
	out := table{Columns: tableColumns(), Rows: make([]row, 0, len(rows))}
	for _, r := range rows {
		out.Rows = append(out.Rows, buildRow(r, eod, names, qs))
	}
	return out
}

// verdictRank is the sort key of a verdict cell.
func verdictRank(key string) int {
	switch key {
	case "worth":
		return 0
	case "pending":
		return 1
	}
	return 2
}

// ratioCell is the text cell of a percentage, or the gap missing_input naming the absent input.
func ratioCell(r ratioResult) any {
	if !r.OK {
		return mkGap("missing_input", "missing "+r.Missing)
	}
	return textCell{Kind: "text", Text: r.Text, Value: r.Value}
}

// keyValue is the sort key of a text moved by scale places, nil when the text is not exact.
func keyValue(text string, scale int) any {
	if v, ok := scaledInt(text, scale); ok {
		return v
	}
	return nil
}

// countCell is a count of contracts from the text of an end-of-day column; a gap when the text is "n/a", a text
// cell when it is not a whole number.
func countCell(text string) any {
	if text == gapText {
		return mkGap("no_data", gapText)
	}
	if v, ok := scaledInt(text, 0); ok && v >= 0 {
		return countAmount{Value: v, Text: text, Unit: "contracts"}
	}
	return textCell{Kind: "text", Text: text}
}

// earningsCell is the E cell: "E MM-DD in window" when the gate events blocks, else the tag text.
func earningsCell(r store.Candidate) textCell {
	text := earningsTag(r)
	if gates, err := parseGates(r.Gates); err == nil {
		if s, ok := gateStatus(gates, gateEvents); ok && s == gateBlock && r.NextEarnings != nil {
			text += " in window"
		}
	}
	return textCell{Kind: "text", Text: text}
}

func buildRow(r store.Candidate, eod map[int]contracts.OptionEod, names displayNames, qs quoteSource) row {
	symbol := names.of(r)
	v := labelOf(r)
	e := eodTextFor(eod, r.InstrumentID)
	bid, askText, deltaStr, volume := r.BidEod, r.Ask, r.Delta, e.Volume
	if l := qs.live(r.InstrumentID); l != nil {
		bid, askText, deltaStr, volume = l.Bid, textPtr(l.Ask), textPtr(l.Delta), l.Volume
	}
	var ann any
	if hasBid(bid) {
		ann, _ = annualisedOf(r.AnnualizedYield)
	} else {
		ann = mkGap("missing_input", "missing bid")
	}
	ask := gapText
	if askText != nil {
		ask = moneyText(*askText)
	}
	var delta any = mkGap("no_data", gapText)
	if deltaStr != nil {
		if rounded, ok := roundDecimal(*deltaStr, 4); ok {
			text, _ := fixedDecimals(*deltaStr, 2)
			delta = textCell{Kind: "text", Text: signedText(text), Value: keyValue(rounded, 4)}
		}
	}
	cells := []cellAt{
		{Column: "symbol", Cell: textCell{Kind: "text", Text: symbol, Value: symbol}},
		{Column: "verdict", Cell: textCell{Kind: "text", Text: v.Label, Value: verdictRank(v.Key)}},
		{Column: "annualised", Cell: ann},
		{Column: "strike_pct", Cell: ratioCell(strikePct(r.Strike, r.Spot))},
		{Column: "dte", Cell: textCell{Kind: "text", Text: fmt.Sprintf("%d days", r.DTE), Value: r.DTE}},
		{Column: "delta", Cell: delta},
		{Column: "bid_ask", Cell: textCell{Kind: "text", Text: moneyText(bid) + " / " + ask, Value: keyValue(bid, 2)}},
		{Column: "spread_pct", Cell: ratioCell(spreadPct(bid, askText))},
		{Column: "volume", Cell: countCell(volume)},
		{Column: "open_interest", Cell: countCell(e.OpenInterest)},
		{Column: "ivp", Cell: mkGap("no_data", gapText)},
		{Column: "earnings", Cell: earningsCell(r)},
	}
	return row{ID: fmt.Sprintf("r%d", r.InstrumentID), Cells: cells, Quote: qs.of(r.InstrumentID)}
}
