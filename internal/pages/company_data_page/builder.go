// builder.go builds page.company_data_page.v1: one row per stock that is not retired, each filled by
// rows.go from the stored values and Calculator.Summarize. It computes nothing (docs/design/data-collection.md,
// Decisions 9 to 12).
package company_data_page

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// Input is what Build reads: the clock as a value, the day and the readers.
type Input struct {
	Now         time.Time
	Today       contracts.Day
	Underlyings underlying.Reader
	Bars        bar.PriceBarReader
	Facts       companydataapi.PeriodFactReader
	Earnings    store.EarningsDateReader
	Calc        companydataapi.Calculator
}

// Missing is one reason a company lacks something, with the sentence Summarize gave.
type Missing struct {
	Code string `json:"code"`
	Text string `json:"text"`
}

// Company is one row of the page.
type Company struct {
	Key           string    `json:"key"`
	Symbol        string    `json:"symbol"`
	Name          string    `json:"name"`
	Market        string    `json:"market"`
	Issuer        string    `json:"issuer"`
	FiscalYearEnd string    `json:"fiscal_year_end"`
	PriceTo       *string   `json:"price_to"`
	BarCount      string    `json:"bar_count"`
	Statements    bool      `json:"statements"`
	LatestQuarter string    `json:"latest_quarter"`
	EPSQuarters   string    `json:"eps_quarters"`
	NextReport    *string   `json:"next_report"`
	Missing       []Missing `json:"missing"`
	QuartersPath  string    `json:"quarters_path"`
}

// Gap says that part of the page could not be built.
type Gap struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type viewModel struct {
	Schema         string        `json:"schema"`
	SchemaVersion  int           `json:"schema_version"`
	RunID          string        `json:"run_id"`
	BuiltAt        string        `json:"built_at"`
	DataHealthPath string        `json:"data_health_path"`
	Columns        []column      `json:"columns"`
	Companies      []Company     `json:"companies"`
	Glossary       []glossaryRow `json:"glossary"`
	Gaps           []Gap         `json:"gaps"`
}

// Build returns the JSON of page.company_data_page.v1. A failed read of one company gives that company a
// gap line and no row, and the other rows stay; a failed read of the list gives one gap and no row. Only a
// nil dependency is an error.
func Build(ctx context.Context, in Input) ([]byte, error) {
	if in.Underlyings == nil || in.Bars == nil || in.Facts == nil || in.Earnings == nil || in.Calc == nil {
		return nil, errors.New("company_data_page: a reader or the calculator is nil")
	}
	vm := viewModel{Schema: pageName, SchemaVersion: 1, RunID: runID, BuiltAt: in.Now.Format(timeLayout),
		DataHealthPath: dataHealthPath, Columns: columnsVM(), Companies: []Company{}, Glossary: glossaryRows(), Gaps: []Gap{}}
	rows, err := in.Underlyings.Current(ctx)
	if err != nil {
		vm.Gaps = append(vm.Gaps, Gap{"gap", gapUnderlyings, textUnderlyings})
		return json.Marshal(vm)
	}
	for _, u := range rows {
		if u.Kind != underlying.KindStock || u.Status == underlying.StatusRetired {
			continue
		}
		c, err := companyRow(ctx, in, u)
		if err != nil {
			vm.Gaps = append(vm.Gaps, Gap{"gap", gapRead, readFailedText(u.Key)})
			continue
		}
		vm.Companies = append(vm.Companies, c)
	}
	return json.Marshal(vm)
}

func readFailedText(key string) string {
	t := key + ": the stored values could not be read, so the company has no row."
	if len(t) > 200 {
		t = t[:200]
	}
	return t
}

func columnsVM() []column { return columns }

func glossaryRows() []glossaryRow { return glossary }
