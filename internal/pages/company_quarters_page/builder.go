// builder.go builds page.company_quarters_page.v1. It computes nothing: every figure is decimal text from
// the Calculator, grouped and signed by format.go (docs/design/data-collection.md, Decisions 5 to 8).
package company_quarters_page

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// Input is what Build reads: the clock as a value, the day, the chosen key (the name_key of
// settings.underlying, "" for none) and the three readers.
type Input struct {
	Now         time.Time
	Today       contracts.Day
	Key         string
	Underlyings underlying.Reader
	Facts       companydataapi.PeriodFactReader
	Calc        companydataapi.Calculator
}

type company struct {
	Key           string `json:"key"`
	Symbol        string `json:"symbol"`
	Name          string `json:"name"`
	Issuer        string `json:"issuer"`
	FiscalYearEnd string `json:"fiscal_year_end"`
}

type choice struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

type cell struct {
	Text     string `json:"text"`
	Missing  bool   `json:"missing"`
	Derived  bool   `json:"derived"`
	Restated bool   `json:"restated"`
	Concept  string `json:"concept"`
	WhyText  string `json:"why_text"`
}

type quarter struct {
	Label      string          `json:"label"`
	PeriodEnd  string          `json:"period_end"`
	FilingDate *string         `json:"filing_date"`
	Cells      map[string]cell `json:"cells"`
}

// Gap says that part of the page could not be built.
type Gap struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type viewModel struct {
	Schema        string    `json:"schema"`
	SchemaVersion int       `json:"schema_version"`
	RunID         string    `json:"run_id"`
	BuiltAt       string    `json:"built_at"`
	Company       *company  `json:"company"`
	Choices       []choice  `json:"choices"`
	Columns       []column  `json:"columns"`
	Quarters      []quarter `json:"quarters"`
	Notes         []string  `json:"notes"`
	Gaps          []Gap     `json:"gaps"`
}

// Build returns the JSON of page.company_quarters_page.v1. A key that is empty or unknown gives the list
// of companies and one gap; a company with no issuer gives the gap no_cik and no table. A failed read is
// a gap, never an empty table. Only a nil dependency is an error.
func Build(ctx context.Context, in Input) ([]byte, error) {
	if in.Underlyings == nil || in.Facts == nil || in.Calc == nil {
		return nil, errors.New("company_quarters_page: a reader or the calculator is nil")
	}
	vm := viewModel{Schema: pageName, SchemaVersion: 1, RunID: runID, BuiltAt: in.Now.Format(timeLayout),
		Choices: []choice{}, Columns: columnsVM(), Quarters: []quarter{}, Notes: []string{}, Gaps: []Gap{}}
	rows, err := in.Underlyings.Current(ctx)
	if err != nil {
		vm.Gaps = append(vm.Gaps, Gap{"gap", gapReadFailed, textReadFailed})
		return json.Marshal(vm)
	}
	vm.Choices = choices(rows)
	row, ok := find(rows, in.Key)
	if !ok {
		vm.Gaps = append(vm.Gaps, Gap{"gap", gapUnknownCompany, textUnknownCompany})
		return json.Marshal(vm)
	}
	vm.Company = &company{Key: row.Key, Symbol: row.Symbol, Name: row.Name, Issuer: row.Issuer}
	switch {
	case row.Market == "JP" || strings.HasPrefix(row.Issuer, "edinet:"):
		vm.Gaps = append(vm.Gaps, Gap{"gap", gapJP, textJP})
	case row.Issuer == "":
		vm.Gaps = append(vm.Gaps, Gap{"gap", gapNoCIK, textNoCIK})
	default:
		fill(ctx, in, row, &vm)
	}
	return json.Marshal(vm)
}

func fill(ctx context.Context, in Input, row underlying.Row, vm *viewModel) {
	facts, err := in.Facts.FactsWithPeriodFiledBy(ctx, sourceSEC, row.Issuer, in.Today)
	if err != nil {
		vm.Gaps = append(vm.Gaps, Gap{"gap", gapReadFailed, textReadFailed})
		return
	}
	qs, err := in.Calc.Quarters(facts)
	if err != nil {
		vm.Gaps = append(vm.Gaps, Gap{"gap", gapReadFailed, textReadFailed})
		return
	}
	if len(qs) == 0 {
		vm.Gaps = append(vm.Gaps, Gap{"gap", gapNoStatements, textNoStatements})
		return
	}
	noLabel := false
	for i := range qs {
		q := quarterVM(&qs[i])
		if qs[i].Label == "" {
			noLabel = true
		}
		vm.Quarters = append(vm.Quarters, q)
		if vm.Company.FiscalYearEnd == "" && qs[i].FiscalYearEndMonth >= 1 && qs[i].FiscalYearEndMonth <= 12 {
			vm.Company.FiscalYearEnd = time.Month(qs[i].FiscalYearEndMonth).String()
		}
	}
	if noLabel {
		vm.Notes = append(vm.Notes, noteNoLabel)
	}
}

func quarterVM(q *companydataapi.Quarter) quarter {
	out := quarter{Label: q.Label, PeriodEnd: string(q.PeriodEnd), Cells: map[string]cell{}}
	if out.Label == "" {
		out.Label = labelPrefix + string(q.PeriodEnd)
	}
	if q.FilingDate != "" {
		d := string(q.FilingDate)
		out.FilingDate = &d
	}
	for _, c := range columns[1:] {
		out.Cells[c.Key] = cellVM(c.Key, cellOf(q, c.Key))
	}
	return out
}

func cellOf(q *companydataapi.Quarter, key string) companydataapi.Cell {
	switch key {
	case "revenue":
		return q.Revenue
	case "operating_income":
		return q.OperatingIncome
	case "net_income":
		return q.NetIncome
	case "eps_diluted":
		return q.EPSDiluted
	case "shares_diluted":
		return q.SharesDiluted
	case "operating_cash_flow":
		return q.OperatingCashFlow
	case "capex":
		return q.Capex
	case "free_cash":
		return q.FreeCash
	case "buybacks":
		return q.Buybacks
	case "dividends":
		return q.Dividends
	case "cash":
		return q.Cash
	case "long_term_debt":
		return q.LongTermDebt
	case "short_term_debt":
		return q.ShortTermDebt
	}
	return companydataapi.Cell{Missing: true}
}

func cellVM(key string, c companydataapi.Cell) cell {
	if c.Missing {
		why, ok := whyTexts[c.Why]
		if !ok {
			why = missingGeneric
		}
		return cell{Missing: true, WhyText: why}
	}
	out := cell{Text: group(c.Value), Derived: c.Derived, Restated: c.Restated, Concept: c.Concept}
	if c.Derived {
		switch key {
		case "eps_diluted":
			out.WhyText = epsDerivedGloss
		case "free_cash":
			out.WhyText = freeCashGloss
		default:
			out.WhyText = flowDerivedGloss
		}
	}
	return out
}

func columnsVM() []column { return columns }

func find(rows []underlying.Row, key string) (underlying.Row, bool) {
	if key == "" {
		return underlying.Row{}, false
	}
	for _, r := range rows {
		if r.Key == key {
			return r, true
		}
	}
	return underlying.Row{}, false
}

func choices(rows []underlying.Row) []choice {
	out := []choice{}
	for _, r := range rows {
		if r.Kind != underlying.KindStock || r.Status == underlying.StatusRetired {
			continue
		}
		out = append(out, choice{Key: r.Key, Label: r.Key, Path: pathBase + "?c=" + url.QueryEscape(r.Key)})
	}
	return out
}
