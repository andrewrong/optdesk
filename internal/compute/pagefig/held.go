package pagefig

import (
	"math/big"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var _ blockapi.HeldFigure = heldFigure{}

type heldFigure struct{}

// NewHeld returns the figure of held companies: one row per company with value, share and next earnings.
func NewHeld() blockapi.HeldFigure { return heldFigure{} }

type heldKey struct{ underlying, currency string }

type heldAcc struct {
	row   blockapi.HeldRow
	sum   *big.Rat
	gap   bool
	alias map[string]bool
}

func (heldFigure) HeldCompanies(in blockapi.HeldIn) blockapi.HeldResult {
	var order []heldKey
	accs := map[heldKey]*heldAcc{}
	for _, l := range in.Lines {
		k := heldKey{l.Underlying, l.Currency}
		a := accs[k]
		if a == nil {
			a = &heldAcc{sum: new(big.Rat), alias: map[string]bool{}}
			a.row.Underlying, a.row.Currency = l.Underlying, l.Currency
			accs[k] = a
			order = append(order, k)
		}
		if !a.alias[l.Alias] {
			a.alias[l.Alias] = true
			a.row.Aliases = append(a.row.Aliases, l.Alias)
		}
		if l.MarkDay != "" && (a.row.OldestMark == "" || l.MarkDay < a.row.OldestMark) {
			a.row.OldestMark = l.MarkDay
		}
		v, ok := parse(l.Value.Value)
		if l.Value.Gap != "" || !ok {
			a.gap = true
			continue
		}
		a.sum.Add(a.sum, v)
	}
	companies := map[heldKey]blockapi.HeldCompanyIn{}
	for _, c := range in.Companies {
		companies[heldKey{c.Underlying, c.Currency}] = c
	}
	total, totalOK := parse(in.TotalUSD.Value)
	totalOK = totalOK && in.TotalUSD.Gap == ""

	type ranked struct {
		row blockapi.HeldRow
		val *big.Rat // nil for a gap row
	}
	var rs []ranked
	rowsSum := new(big.Rat)
	for _, k := range order {
		a := accs[k]
		r := a.row
		var val *big.Rat
		if a.gap {
			r.Value, r.Share = gap("missing_quote"), gap("missing_quote")
		} else {
			val = a.sum
			rowsSum.Add(rowsSum, val)
			r.Value = compute.FigureValue{Value: money(val)}
			r.Share = share(val, total, totalOK && total.Sign() != 0)
			if !totalOK {
				r.Share = gap(heldTotalGap(in.TotalUSD))
			}
		}
		r.Next, r.DaysTo, r.EarnReason = heldEarnings(companies[k], in.Today)
		rs = append(rs, ranked{r, val})
	}
	sort.SliceStable(rs, func(i, j int) bool {
		a, b := rs[i], rs[j]
		if (a.val == nil) != (b.val == nil) {
			return a.val != nil
		}
		if a.val != nil {
			if c := a.val.Cmp(b.val); c != 0 {
				return c > 0
			}
		}
		return a.row.Underlying < b.row.Underlying
	})
	out := blockapi.HeldResult{}
	have := false
	for _, x := range rs {
		out.Rows = append(out.Rows, x.row)
		have = have || x.val != nil
	}
	if have {
		out.RowsSum = compute.FigureValue{Value: money(rowsSum)}
	} else {
		out.RowsSum = gap("no_data")
	}
	return out
}

// heldTotalGap is the gap of an unusable total: its own gap, else no_data.
func heldTotalGap(t compute.FigureValue) string {
	if t.Gap != "" {
		return t.Gap
	}
	return "no_data"
}

func parseHeldDay(d contracts.Day) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", string(d))
	return t, err == nil
}

// heldEarnings gives the next date, the days to it and, when there is no date, the reason.
func heldEarnings(c blockapi.HeldCompanyIn, today contracts.Day) (contracts.Day, int, string) {
	switch c.Resolve {
	case "ok":
	case "no_issuer":
		return "", 0, "no_issuer"
	default:
		return "", 0, "no_underlying"
	}
	e := c.Earnings
	if !e.Read {
		return "", 0, "no_reader"
	}
	date := e.Override
	if date == "" {
		if len(e.Rows) == 0 {
			return "", 0, "no_row"
		}
		newest := e.Rows[0]
		for _, r := range e.Rows[1:] {
			if r.ObservedOn >= newest.ObservedOn {
				newest = r
			}
		}
		switch newest.Status {
		case "unsupported":
			return "", 0, "unsupported"
		case "error":
			return "", 0, "row_error"
		case "listed":
		default:
			return "", 0, "no_row"
		}
		date = newest.Report
	}
	d, ok := parseHeldDay(date)
	t, tok := parseHeldDay(today)
	if !ok || !tok {
		return "", 0, "no_row"
	}
	if d.Before(t) {
		return "", 0, "past_date"
	}
	return date, int(d.Sub(t).Hours() / 24), ""
}
