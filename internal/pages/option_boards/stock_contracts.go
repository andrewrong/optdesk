package option_boards

import "fmt"

const (
	maxContractRows = 200
	contractsBasis  = "存档收盘报价;Θ、Γ、Vega 取数据日的存档行"
)

func contractColumns() []string {
	return []string{"到期", "行权价", "买价", "卖价", "Δ", "IV", "Θ/天", "Γ", "Vega", "成交量", "年化"}
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
		out = append(out, contractLine{
			Expiry: string(c.Expiry), ExpiryText: text,
			Cells: []string{md, c.Strike, c.BidEod, ask, delta, iv, r.Eod.Theta, r.Eod.Gamma, r.Eod.Vega, r.Eod.Volume, yield},
		})
	}
	note := ""
	if cut > 0 {
		note = fmt.Sprintf("。合约表只显示前 %d 行,另有 %d 行未显示", maxContractRows, cut)
	}
	return out, note, nil
}
