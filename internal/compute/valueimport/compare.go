package valueimport

import (
	"strings"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const hkcnDiffers = "HK and CN differ: the site has one HKCN column"

func decEq(a, b string) bool {
	x, err1 := decimal.NewFromString(strings.TrimSpace(a))
	y, err2 := decimal.NewFromString(strings.TrimSpace(b))
	return err1 == nil && err2 == nil && x.Equal(y)
}

// withinRounding is true when the difference is at most the larger of the relative and absolute tolerance.
func withinRounding(site, file string) bool {
	s, err1 := decimal.NewFromString(site)
	f, err2 := decimal.NewFromString(file)
	if err1 != nil || err2 != nil {
		return false
	}
	tol := f.Abs().Mul(decimal.RequireFromString(valueimportapi.RoundTolerance))
	if abs := decimal.RequireFromString(valueimportapi.RoundToleranceAbs); abs.GreaterThan(tol) {
		tol = abs
	}
	return s.Sub(f).Abs().LessThanOrEqual(tol)
}

func laterThan(a, b contracts.Day) bool { return a > b }

// planR decides the R version: refused when HK and CN differ, same, older or insert.
func planR(f *file, st valueimportapi.Stored) valueimportapi.RPlan {
	note := []rune(f.rNote)
	if len(note) > 600 {
		note = note[:600]
	}
	p := valueimportapi.RPlan{US: f.rUS, JP: f.rJP, HKCN: f.rHK, Note: string(note)}
	if st.R != nil {
		p.AISwitch, p.AIExpiry = st.R.AISwitch, st.R.AIExpiry
	}
	switch {
	case !decEq(f.rHK, f.rCN):
		p.Action, p.Problem = valueimportapi.ActionRefused, hkcnDiffers
	case st.R != nil && decEq(f.rUS, st.R.US) && decEq(f.rJP, st.R.JP) && decEq(f.rHK, st.R.HKCN):
		p.Action = valueimportapi.ActionSame
	case st.R != nil && !laterThan(contracts.Day(f.asOf), st.R.EffectiveDate):
		p.Action = valueimportapi.ActionOlder
	default:
		p.Action = valueimportapi.ActionInsert
	}
	return p
}

func marginEq(a, b []valuelinesapi.MarginRow) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !decEq(a[i].PFrom, b[i].PFrom) || !decEq(a[i].M, b[i].M) || !decEq(a[i].W, b[i].W) ||
			!decEq(a[i].H, b[i].H) || !decEq(a[i].S, b[i].S) {
			return false
		}
	}
	return true
}

func planMargin(f *file, rows []valuelinesapi.MarginRow, st valueimportapi.Stored) valueimportapi.MarginPlan {
	p := valueimportapi.MarginPlan{Rows: rows, EffectiveDate: contracts.Day(f.asOf)}
	switch {
	case len(st.Margin) > 0 && marginEq(rows, st.Margin):
		p.Action = valueimportapi.ActionSame
	case len(st.Margin) > 0 && !laterThan(p.EffectiveDate, st.MarginDate):
		p.Action = valueimportapi.ActionOlder
	default:
		p.Action = valueimportapi.ActionInsert
	}
	return p
}

// tables are the R and margin that will be the newest after the import.
type tables struct {
	r      valuelinesapi.RValues
	margin []valuelinesapi.MarginRow
	hasR   bool
	hasM   bool
}

func effectiveTables(rp valueimportapi.RPlan, mp valueimportapi.MarginPlan, st valueimportapi.Stored) tables {
	var t tables
	if rp.Action == valueimportapi.ActionInsert || rp.Action == valueimportapi.ActionSame {
		t.r, t.hasR = valuelinesapi.RValues{US: rp.US, JP: rp.JP, HKCN: rp.HKCN}, true
	} else if st.R != nil {
		t.r, t.hasR = valuelinesapi.RValues{US: st.R.US, JP: st.R.JP, HKCN: st.R.HKCN}, true
	}
	if mp.Action == valueimportapi.ActionInsert || mp.Action == valueimportapi.ActionSame {
		t.margin, t.hasM = mp.Rows, true
	} else if len(st.Margin) > 0 {
		t.margin, t.hasM = st.Margin, true
	}
	return t
}

func fileRFor(f *file, market string) string {
	switch market {
	case "US":
		return f.rUS
	case "JP":
		return f.rJP
	case "HK":
		return f.rHK
	}
	return f.rCN
}

func currencyOK(rMarket, cur string) bool {
	switch rMarket {
	case valuelinesapi.MarketUS:
		return cur == "USD"
	case valuelinesapi.MarketJP:
		return cur == "JPY"
	}
	return cur == "HKD" || cur == "CNY"
}

// refusal runs the checks of a row against the file's R, the effective tables and the calculator.
// It returns the site's buy-zone top and the reason, empty when the row passes.
func refusal(f *file, c company, row *valueimportapi.RowPlan, rp valueimportapi.RPlan, t tables, calc valuelinesapi.Calculator) string {
	if !decEq(c.r, fileRFor(f, c.market)) {
		return "R of the row differs from the R of its market in the file"
	}
	if rp.Problem != "" && row.RMarket == valuelinesapi.MarketHKCN {
		return rp.Problem
	}
	if !t.hasR || !t.hasM {
		return "no R version or margin table to compute the lines"
	}
	lines, err := calc.Lines(valuelinesapi.Input{V: c.v, P: c.p, Market: row.RMarket, R: t.r, Margin: t.margin})
	if err != nil {
		return "the site cannot compute the lines: V or P is out of range"
	}
	row.SiteBuyTop = lines.BuyHigh
	if !withinRounding(lines.BuyHigh, c.bHi) {
		return "the site computes another buy-zone top"
	}
	probs := calc.Check(valuelinesapi.CheckInput{Type: row.ReviewType, VersionDate: string(row.VersionDate), V: c.v, P: c.p, Margin: t.margin})
	if len(probs) > 0 {
		texts := make([]string, len(probs))
		for i, p := range probs {
			texts[i] = p.Text
		}
		return strings.Join(texts, "; ")
	}
	return ""
}

// notInFile lists the stored keys that no row of the file uses, in stored order and once.
func notInFile(st valueimportapi.Stored, used map[string]bool) []string {
	var out []string
	seen := map[string]bool{}
	add := func(k string) {
		if k != "" && !used[k] && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for _, p := range st.Profiles {
		add(p.Key)
	}
	for _, c := range st.Current {
		add(c.Key)
	}
	return out
}
