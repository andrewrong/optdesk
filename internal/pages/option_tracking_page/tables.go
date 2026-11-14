package option_tracking_page

import (
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/trackapi"
)

type column struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Align string  `json:"align"`
	Hint  *string `json:"hint"`
}

type row struct {
	ID    string         `json:"id"`
	Cells map[string]any `json:"cells"`
}

type table struct {
	Columns []column `json:"columns"`
	Rows    []row    `json:"rows"`
}

// colDef is a column and the way its cell is read from a row of the tracker.
type colDef[T any] struct {
	column
	cell func(T) any
}

func col[T any](id, label, align string, cell func(T) any) colDef[T] {
	return colDef[T]{column: column{ID: id, Label: label, Align: align}, cell: cell}
}

func (d colDef[T]) hinted(h string) colDef[T] {
	d.Hint = &h
	return d
}

type opt = trackapi.TrackedOption

var openColumns = []colDef[opt]{
	col("company", "Company", "left", company),
	col("contract", "Contract", "left", openContract),
	col("qty", "Quantity", "right", qty),
	col("dte", "DTE", "right", dteCell),
	col("moneyness", "Moneyness", "right", moneynessCell).hinted("Percent distance from the strike, positive is out of the money"),
	col("spot", "Stock close", "right", func(o opt) any { return decimalCell(o.Spot, markCode(o), 2) }),
	col("mid", "Mid price", "right", func(o opt) any { return decimalCell(o.Mid, markCode(o), 2) }).hinted(markNote),
	col("premium", "Premium collected", "right", func(o opt) any { return moneyCell(o.PremiumIncome, o.Currency) }),
	col("kept", "Kept", "right", func(o opt) any { return decimalCell(o.Kept, markCode(o), 2) }),
	col("kept_percent", "Kept %", "right", func(o opt) any { return decimalCell(o.KeptPercent, markCode(o), 2) }),
	col("delta", "Delta", "right", func(o opt) any { return decimalCell(o.Delta, markCode(o), 2) }),
	col("iv", "IV", "right", func(o opt) any { return decimalCell(o.Iv, markCode(o), 4) }),
	col("results", "Next earnings", "left", resultsCell),
	col("ex", "Ex-dividend date", "left", exCell),
	col("guard", "Early assignment", "left", guardCell),
}

var recentColumns = []colDef[opt]{
	col("company", "Company", "left", company),
	col("contract", "Contract", "left", openContract),
	col("qty", "Quantity", "right", qty),
	col("status", "Status", "left", statusCell),
	col("closed_day", "Closed day", "left", closedDayCell),
	col("premium", "Premium collected", "right", func(o opt) any { return moneyCell(o.PremiumIncome, o.Currency) }),
	col("commissions", "Commissions", "right", func(o opt) any { return decimalCell(o.Commissions, codeNoValue, 2) }),
	col("realized", "Realized P&L", "right", func(o opt) any { return decimalCell(o.RealizedPnL, codeNoValue, 2) }),
}

var reconColumns = []colDef[trackapi.TrackRecon]{
	col("company", "Company", "left", reconCompany),
	col("contract", "Contract", "left", reconContract),
	col("qty", "Quantity", "right", reconQty),
	col("code", "Difference", "left", reconCode),
	col("note", "Note", "left", reconNote),
}

// safeID keeps the characters a row id may hold and cuts it to 96.
func safeID(s string) string {
	b := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == ':', r == '_', r == '-':
			return r
		}
		return '_'
	}, s)
	if b == "" {
		return "x"
	}
	if len(b) > 96 {
		b = b[:96]
	}
	return b
}

// makeTable formats rows in the order they come; it adds nothing and sorts nothing.
func makeTable[T any](cols []colDef[T], rows []T, id func(T) string) *table {
	t := &table{Columns: make([]column, 0, len(cols)), Rows: make([]row, 0, len(rows))}
	for _, c := range cols {
		t.Columns = append(t.Columns, c.column)
	}
	for _, r := range rows {
		cells := make(map[string]any, len(cols))
		for _, c := range cols {
			cells[c.ID] = c.cell(r)
		}
		t.Rows = append(t.Rows, row{ID: id(r), Cells: cells})
	}
	return t
}

func posID(o opt) string { return "pos-" + safeID(o.PositionID) }

func chkID(r trackapi.TrackRecon) string {
	return "chk-" + safeID(strings.Join([]string{r.Code, r.Underlying, string(r.Expiry), r.Strike, r.Right}, idSep))
}

func openTable(rows []opt) *table                  { return makeTable(openColumns, rows, posID) }
func recentTable(rows []opt) *table                { return makeTable(recentColumns, rows, posID) }
func reconTable(rows []trackapi.TrackRecon) *table { return makeTable(reconColumns, rows, chkID) }

// accountTitle is the heading of an account, as the other account pages write it.
func accountTitle(alias string) string { return labelAccount + alias + labelClose }

// formatAccount turns the tracker result of one alias into an Account. stale is true when an
// open row has no fresh mark. A table with no rows is null, except the check table of an
// account that was captured, where no row means nothing differs.
func formatAccount(alias string, res trackapi.TrackResult, heldKnown bool) (a account, stale bool) {
	a = account{Alias: alias, Title: accountTitle(alias), LongLegsLeftOut: res.LongLegsLeftOut}
	if len(res.Open) > 0 {
		a.Open = openTable(res.Open)
	} else {
		a.Title += titleSep + noOpenText
	}
	if len(res.Recent) > 0 {
		a.Recent = recentTable(res.Recent)
	}
	if heldKnown {
		a.Recon = reconTable(res.Recon)
	}
	for _, o := range res.Open {
		if o.MarkStatus == codeNoMark {
			stale = true
		}
		if a.MarkDay == nil && o.MarkDay != "" {
			d := string(o.MarkDay)
			a.MarkDay = &d
		}
	}
	if a.Open == nil && a.Recent == nil && a.Recon == nil {
		g := gap{Kind: "gap", Reason: reasonData, Text: noOpenText}
		a.Gap = &g
	}
	return a, stale
}
