// builder.go builds page.company_data_page.v2: one row per stock that is not retired, each filled by
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
	"github.com/andrewrong/optdesk/internal/pages/shared"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// StockIDReader reads the instruments of type stock whose name is in names.
type StockIDReader interface {
	StockIDs(ctx context.Context, names []string) ([]store.InstrumentName, error)
}

// stockIDBatch is the most names one StockIDs call is given.
const stockIDBatch = 500

// Input is what Build reads: the clock as a value, the day and the readers.
type Input struct {
	Now         time.Time
	Today       contracts.Day
	Underlyings underlying.Reader
	Bars        bar.PriceBarReader
	Facts       companydataapi.PeriodFactReader
	Earnings    store.EarningsDateReader
	Calc        companydataapi.Calculator
	// StockIDs resolves the symbol of a row to its instrument id; nil gives no bars and HasInstrument false.
	StockIDs StockIDReader
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
	// BarsSource and BarsSourceNote are left out when every bar is ibkr.
	BarsSource     string `json:"bars_source,omitempty"`
	BarsSourceNote string `json:"bars_source_note,omitempty"`
}

// Build returns the JSON of page.company_data_page.v2. A failed read of one company gives that company a
// gap line and no row, and the other rows stay; a failed read of the list gives one gap and no row. Only a
// nil dependency is an error.
func Build(ctx context.Context, in Input) ([]byte, error) {
	if in.Underlyings == nil || in.Bars == nil || in.Facts == nil || in.Earnings == nil || in.Calc == nil {
		return nil, errors.New("company_data_page: a reader or the calculator is nil")
	}
	vm := viewModel{Schema: pageName, SchemaVersion: 2, RunID: runID, BuiltAt: in.Now.Format(timeLayout),
		DataHealthPath: dataHealthPath, Columns: columnsVM(), Companies: []Company{}, Glossary: glossaryRows(), Gaps: []Gap{}}
	rows, err := in.Underlyings.Current(ctx)
	if err != nil {
		vm.Gaps = append(vm.Gaps, Gap{"gap", gapUnderlyings, textUnderlyings})
		return json.Marshal(vm)
	}
	var stocks []underlying.Row
	for _, u := range rows {
		if u.Kind == underlying.KindStock && u.Status != underlying.StatusRetired {
			stocks = append(stocks, u)
		}
	}
	ids, idsErr := stockIDsOf(ctx, in.StockIDs, stocks)
	var read []contracts.PriceBar
	for _, u := range stocks {
		if idsErr != nil {
			vm.Gaps = append(vm.Gaps, Gap{"gap", gapRead, readFailedText(u.Key)})
			continue
		}
		c, bars, err := companyRow(ctx, in, u, ids[u.Symbol])
		if err != nil {
			vm.Gaps = append(vm.Gaps, Gap{"gap", gapRead, readFailedText(u.Key)})
			continue
		}
		vm.Companies = append(vm.Companies, c)
		read = append(read, bars...)
	}
	if kind, note := shared.BarSourceOf(read); kind != shared.BarSourceIbkr {
		vm.BarsSource, vm.BarsSourceNote = kind, note
	}
	return json.Marshal(vm)
}

// stockIDsOf asks StockIDs once per batch of 500 distinct symbols and returns the id of each symbol that came
// back exactly once; a symbol that is absent or came back twice is not in the map. A nil reader gives an empty map.
func stockIDsOf(ctx context.Context, r StockIDReader, stocks []underlying.Row) (map[string]int64, error) {
	out := map[string]int64{}
	if r == nil {
		return out, nil
	}
	var names []string
	seen := map[string]bool{}
	for _, u := range stocks {
		if u.Symbol != "" && !seen[u.Symbol] {
			seen[u.Symbol] = true
			names = append(names, u.Symbol)
		}
	}
	count := map[string]int{}
	for i := 0; i < len(names); i += stockIDBatch {
		got, err := r.StockIDs(ctx, names[i:min(i+stockIDBatch, len(names))])
		if err != nil {
			return nil, err
		}
		for _, g := range got {
			count[g.Name]++
			out[g.Name] = g.InstrumentID
		}
	}
	for n, c := range count {
		if c != 1 || out[n] == 0 {
			delete(out, n)
		}
	}
	return out, nil
}

func readFailedText(key string) string {
	t := []rune(key + textReadFailed)
	if len(t) > 200 {
		t = t[:200]
	}
	return string(t)
}

func columnsVM() []column { return columns }

func glossaryRows() []glossaryRow { return glossary }
