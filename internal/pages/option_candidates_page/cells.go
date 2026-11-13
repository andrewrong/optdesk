package option_candidates_page

import (
	"encoding/json"
	"strconv"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type gap struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

func mkGap(reason, text string) gap { return gap{Kind: "gap", Reason: reason, Text: text} }

type valueCell struct {
	Text   string       `json:"text"`
	Value  *json.Number `json:"value"`
	Badges []string     `json:"badges"`
	Tone   string       `json:"tone"`
}

func textCell(text string) valueCell {
	return valueCell{Text: text, Badges: []string{}, Tone: "none"}
}

func unreadableCell() gap { return mkGap("unreadable", unreadable) }

// numCell shows the stored decimal text as it is, with the text as its sort value.
func numCell(text string) any {
	v, ok := numberLiteral(text)
	if !ok {
		return unreadableCell()
	}
	c := textCell(text)
	c.Value = &v
	return c
}

func numCellPtr(p *string) any {
	if p == nil {
		return textCell(dash)
	}
	return numCell(*p)
}

func intCell(n int) any { return numCell(strconv.Itoa(n)) }

func rankCell(p *int) any {
	if p == nil {
		return textCell(dash)
	}
	return intCell(*p)
}

func yieldCell(p *string) any {
	if p == nil {
		return textCell(dash)
	}
	text, err := shiftPoint(*p)
	v, ok := numberLiteral(*p)
	if err != nil || !ok {
		return unreadableCell()
	}
	c := textCell(text)
	c.Value = &v
	return c
}

func plainCell(s string) any {
	if s == "" {
		return textCell(dash)
	}
	return textCell(s)
}

func dayCell(d contracts.Day) any { return plainCell(string(d)) }

func dayCellPtr(p *contracts.Day) any {
	if p == nil {
		return textCell(dash)
	}
	return dayCell(*p)
}

func earningsCell(c store.Candidate) any {
	if c.NextEarnings == nil {
		return earningsGap(c.EarningsState)
	}
	cell := earningsDateCell(*c.NextEarnings)
	if vc, ok := cell.(valueCell); ok && c.EarningsState != "" {
		vc.Badges = append(vc.Badges, c.EarningsState)
		return vc
	}
	return cell
}

func marksCell(c store.Candidate) any {
	marks, err := parseMarks(c.Marks)
	if err != nil {
		return unreadableCell()
	}
	cell := textCell(dash)
	for _, m := range marks {
		if h, ok := markHints[m]; ok {
			m += fitSeparator + h
		}
		cell.Badges = append(cell.Badges, m)
	}
	return cell
}

func fitCell(c store.Candidate) any {
	fits, err := parseFits(c.Fits)
	if err != nil {
		return unreadableCell()
	}
	cell := textCell(dash)
	for _, f := range fits {
		word := f[1]
		if h, ok := fitHints[word]; ok {
			word = h
		}
		cell.Badges = append(cell.Badges, f[0]+fitSeparator+word)
	}
	return cell
}

func statusCell(c store.Candidate) any {
	cell := textCell(c.Status)
	cell.Badges = []string{c.Status}
	switch c.Status {
	case "ok":
		cell.Tone = "up"
	case "blocked":
		cell.Tone = "down"
	default:
		cell.Tone = "flat"
	}
	return cell
}

func contractCell(c store.Candidate) any {
	return textCell(c.Right + fitSeparator + c.Strike + fitSeparator + string(c.Expiry))
}

// guardCell is the reason of the guard gate, or the pass.
func guardCell(c store.Candidate) any {
	gates, err := parseGates(c.Gates)
	if err != nil {
		return unreadableCell()
	}
	for _, g := range gates {
		if g.Gate == "guard" {
			if g.Status == "pass" {
				return textCell(passText)
			}
			return textCell(reasonText(g.Reason))
		}
	}
	return textCell(dash)
}

// reasonsCell is the reasons of the gates in one line.
func reasonsCell(c store.Candidate) any {
	gates, err := parseGates(c.Gates)
	if err != nil {
		return unreadableCell()
	}
	return plainCell(reasonsLine(reasonsOf(gates)))
}

type column struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Align string  `json:"align"`
	Hint  *string `json:"hint"`
}

// colDef is a column and the way its cell is read from a stored row.
type colDef struct {
	column
	cell func(store.Candidate) any
	// cellAt, when set, is used instead of cell: it reads the cell by the row's position in the table.
	cellAt func(int, store.Candidate) any
}

func col(id, label, align string, cell func(store.Candidate) any) colDef {
	return colDef{column: column{ID: id, Label: label, Align: align}, cell: cell}
}

func (d colDef) hinted(h string) colDef {
	d.Hint = &h
	return d
}

func company(c store.Candidate) any { return plainCell(c.Issuer) }
func alias(c store.Candidate) any   { return plainCell(c.Alias) }

var (
	colRank      = col("rank", "Rank", "right", func(c store.Candidate) any { return rankCell(c.Rank) })
	colCompany   = col("company", "Company", "left", company)
	colAlias     = col("alias", "Account", "left", alias)
	colExpiry    = col("expiry", "Expiry", "left", func(c store.Candidate) any { return dayCell(c.Expiry) })
	colDTE       = col("dte", "DTE", "right", func(c store.Candidate) any { return intCell(c.DTE) })
	colStrike    = col("strike", "Strike", "right", func(c store.Candidate) any { return numCell(c.Strike) })
	colBid       = col("bid", bidLabel, "right", func(c store.Candidate) any { return numCell(c.BidEod) }).hinted(bidHint)
	colYield     = col("yield", "Annualised yield", "right", func(c store.Candidate) any { return yieldCell(c.AnnualizedYield) })
	colBreakEven = col("break_even", "Break-even", "right", func(c store.Candidate) any { return numCell(c.BreakEven) })
	colDelta     = col("delta", "Delta", "right", func(c store.Candidate) any { return numCellPtr(c.Delta) })
	colIV        = col("iv", "IV", "right", func(c store.Candidate) any { return numCellPtr(c.Iv) })
	colITM       = colDef{column: column{ID: itmID, Label: "ITM probability", Align: "right", Hint: ptr(itmHint)}}
	colExcess    = colDef{column: column{ID: excessID, Label: "Vs cash", Align: "right"}}
	colMarks     = col("marks", "Marks", "left", marksCell)
	colFit       = col("fit", "Account cash", "left", fitCell)
	colEarnings  = col("next_earnings", "Next earnings", "left", earningsCell)
	colExtrinsic = col("extrinsic", "Extrinsic value", "right", func(c store.Candidate) any { return numCellPtr(c.Extrinsic) })
	colExDate    = col("next_ex_date", "Next ex-dividend date", "left", func(c store.Candidate) any { return dayCellPtr(c.NextExDate) })
	colExAmount  = col("ex_amount", "Dividend", "right", func(c store.Candidate) any { return numCellPtr(c.ExAmount) })
	colStatus    = col("status", "Status", "left", statusCell)
	colGuard     = col("guard", "Guard", "left", guardCell)
	colContract  = col("contract", "Contract", "left", contractCell)
	colReasons   = col("reasons", "Reasons", "left", reasonsCell)
)

var otherColumns = []colDef{colCompany, colContract, colAlias, colStatus, colReasons}

func ptr(s string) *string { return &s }
