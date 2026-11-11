package option_boards

import (
	"fmt"
	"sort"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// The schema limits of the matrix.
const (
	maxStrikes  = 100
	maxExpiries = 30
	maxCells    = 2000

	matrixLegend = "★ top of the underlying · green: worth a look · thin: bid/ask incomplete · earn: earnings in window"
)

func cellKey(id int) string { return fmt.Sprintf("c%d", id) }

// expiryLabel is "11-13 Fri · 3 days" of a stored expiry and DTE.
func expiryLabel(expiry contracts.Day, dte int) (string, error) {
	md, err := monthDay(expiry)
	if err != nil {
		return "", err
	}
	wd, err := weekdayText(expiry)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %s · %d days", md, wd, dte), nil
}

// deltaText is the delta text "Δ −0.15", "Δ n/a" for null.
func deltaText(d *string) string {
	if d == nil {
		return "Δ n/a"
	}
	return "Δ " + signedText(*d)
}

// yieldText is "Annualised 12.3456%" or "Annualised n/a".
func yieldText(y *string) (string, error) {
	if y == nil {
		return "Annualised n/a", nil
	}
	pct, err := shiftPercent(*y)
	if err != nil {
		return "", err
	}
	return "Annualised " + pct, nil
}

// hasBid is false for the empty stored bid and for the gap text: such a row has no premium and no yield.
func hasBid(bid string) bool { return bid != "" && bid != gapText }

// premiumText is "Premium 1.20", or "Premium n/a" for a missing bid.
func premiumText(bid string) string {
	if !hasBid(bid) {
		return "Premium n/a"
	}
	return "Premium " + bid
}

func volumeText(v string) string {
	if v == gapText {
		return "Volume n/a"
	}
	return "Volume " + v
}

// buildMatrix draws one cell per stored row. Rows over the schema limits (cells, distinct strikes, distinct
// expiries) are cut in the stored order; the note says how many.
func buildMatrix(rows []stockRow, chosen int) (matrixBlock, string, error) {
	m := matrixBlock{Expiries: []matrixExpiry{}, Strikes: []string{}, Cells: []matrixCell{}, Legend: matrixLegend}
	expiries := map[contracts.Day]int{}
	strikes := map[string]bool{}
	cut := 0
	for i, r := range rows {
		c := r.C
		if _, err := compareDecimal(c.Strike, c.Strike); err != nil {
			return m, "", fmt.Errorf("option_boards: strike of instrument %d: %w", c.InstrumentID, err)
		}
		_, haveExp := expiries[c.Expiry]
		if len(m.Cells) >= maxCells || (!haveExp && len(expiries) >= maxExpiries) || (!strikes[c.Strike] && len(strikes) >= maxStrikes) {
			cut++
			continue
		}
		bid := c.BidEod
		if r.Live != nil {
			bid = r.Live.Bid
		}
		yield := c.AnnualizedYield
		if !hasBid(bid) {
			yield = nil
		}
		yt, err := yieldText(yield)
		if err != nil {
			return m, "", fmt.Errorf("option_boards: yield of instrument %d: %w", c.InstrumentID, err)
		}
		if !haveExp {
			expiries[c.Expiry] = c.DTE
		}
		strikes[c.Strike] = true
		quoteGate, _ := gateStatus(r.Gates, gateQuote)
		events, _ := gateStatus(r.Gates, gateEvents)
		premium, delta, volume := premiumText(c.BidEod), deltaText(c.Delta), volumeText(r.Eod.Volume)
		if l := r.Live; l != nil {
			premium, delta, volume = premiumText(l.Bid), "Δ "+l.DeltaText(), volumeText(l.Volume)
		}
		cell := matrixCell{
			Key: cellKey(c.InstrumentID), Expiry: string(c.Expiry), Strike: c.Strike,
			YieldText: yt, Premium: premium, Delta: delta, Volume: volume,
			Shade: "none", Star: c.RankInUnderlying != nil && *c.RankInUnderlying == 1,
			Out: false, Thin: quoteGate != gatePass, Earnings: events == gateBlock, Chosen: i == chosen, Quote: r.Quote,
		}
		if r.ok() {
			cell.Shade = "worth"
		}
		m.Cells = append(m.Cells, cell)
	}
	for e, dte := range expiries {
		text, err := expiryLabel(e, dte)
		if err != nil {
			return m, "", err
		}
		m.Expiries = append(m.Expiries, matrixExpiry{Expiry: string(e), Text: text})
	}
	sort.Slice(m.Expiries, func(a, b int) bool { return m.Expiries[a].Expiry < m.Expiries[b].Expiry })
	for s := range strikes {
		m.Strikes = append(m.Strikes, s)
	}
	sort.Slice(m.Strikes, func(a, b int) bool {
		c, _ := compareDecimal(m.Strikes[a], m.Strikes[b])
		if c == 0 {
			return m.Strikes[a] < m.Strikes[b]
		}
		return c < 0
	})
	note := ""
	if cut > 0 {
		note = fmt.Sprintf(". Matrix over the limit, %d contracts not shown", cut)
	}
	return m, note, nil
}
