package option_boards

import "fmt"

const (
	maxContractRows = 200
	contractsBasis  = "Archived close quotes; Theta, Gamma and Vega from the archived row of the data day"
)

func contractColumns() []string {
	return []string{"Expiry", "Strike", "Bid", "Ask", "Δ", "IV", "Θ/day", "Γ", "Vega", "Volume", "Annualised"}
}

// buildContractRows is one line per stored row of the company in the stored order, at most maxContractRows;
// the second result is the note of the rows cut.
func buildContractRows(rows []stockRow) ([]contractLine, string, error) {
	out := []contractLine{}
	cut := 0
	for _, r := range rows {
		if len(out) == maxContractRows {
			cut++
			continue
		}
		c := r.C
		md, err := monthDay(c.Expiry)
		if err != nil {
			return nil, "", err
		}
		text, err := expiryLabel(c.Expiry, c.DTE)
		if err != nil {
			return nil, "", err
		}
		ask, iv, yield := gapText, gapText, gapText
		if c.Ask != nil {
			ask = *c.Ask
		}
		if c.Iv != nil {
			if iv, err = shiftPercent(*c.Iv); err != nil {
				return nil, "", fmt.Errorf("option_boards: iv of instrument %d: %w", c.InstrumentID, err)
			}
		}
		if c.AnnualizedYield != nil {
			if yield, err = shiftPercent(*c.AnnualizedYield); err != nil {
				return nil, "", fmt.Errorf("option_boards: yield of instrument %d: %w", c.InstrumentID, err)
			}
		}
		delta := gapText
		if c.Delta != nil {
			delta = signedText(*c.Delta)
		}
		bid, theta, gamma, vega, volume := c.BidEod, r.Eod.Theta, r.Eod.Gamma, r.Eod.Vega, r.Eod.Volume
		if l := r.Live; l != nil {
			bid, ask, delta, iv, theta, gamma, vega, volume = l.Bid, l.Ask, l.DeltaText(), l.IV, l.Theta, l.Gamma, l.Vega, l.Volume
		}
		if !hasBid(bid) {
			bid, yield = gapText, gapText
		}
		out = append(out, contractLine{
			Expiry: string(c.Expiry), ExpiryText: text,
			Quote: r.Quote,
			Cells: []string{md, c.Strike, bid, ask, delta, iv, theta, gamma, vega, volume, yield},
		})
	}
	note := ""
	if cut > 0 {
		note = fmt.Sprintf(". Contract table shows the first %d rows, %d more not shown", maxContractRows, cut)
	}
	return out, note, nil
}
