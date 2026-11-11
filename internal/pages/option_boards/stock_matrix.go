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

	matrixLegend = "★ 标的内排名第一 · 绿底 值得看 · 薄 买卖价不全 · 财 财报在期内"
)

func cellKey(id int) string { return fmt.Sprintf("c%d", id) }

// expiryLabel is "11-13 周五 · 22 天" of a stored expiry and DTE.
func expiryLabel(expiry contracts.Day, dte int) (string, error) {
	md, err := monthDay(expiry)
	if err != nil {
		return "", err
	}
	wd, err := weekdayText(expiry)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %s · %d 天", md, wd, dte), nil
}

// deltaText is the delta text "Δ −0.15", "Δ 缺" for null.
func deltaText(d *string) string {
	if d == nil {
		return "Δ 缺"
	}
	return "Δ " + signedText(*d)
}

// yieldText is "年化 12.3456%" or "年化 缺".
func yieldText(y *string) (string, error) {
	if y == nil {
		return "年化 缺", nil
	}
	pct, err := shiftPercent(*y)
	if err != nil {
		return "", err
	}
	return "年化 " + pct, nil
}

func volumeText(v string) string {
	if v == gapText {
		return "成交量 缺"
	}
	return "成交量 " + v
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
		yt, err := yieldText(c.AnnualizedYield)
		if err != nil {
			return m, "", fmt.Errorf("option_boards: yield of instrument %d: %w", c.InstrumentID, err)
		}
		if !haveExp {
			expiries[c.Expiry] = c.DTE
		}
		strikes[c.Strike] = true
		quoteGate, _ := gateStatus(r.Gates, gateQuote)
		events, _ := gateStatus(r.Gates, gateEvents)
		cell := matrixCell{
			Key: cellKey(c.InstrumentID), Expiry: string(c.Expiry), Strike: c.Strike,
			YieldText: yt, Premium: "权利金 " + c.BidEod, Delta: deltaText(c.Delta), Volume: volumeText(r.Eod.Volume),
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
		note = fmt.Sprintf("。矩阵超出上限,有 %d 个合约未显示", cut)
	}
	return m, note, nil
}
