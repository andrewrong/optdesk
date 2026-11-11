// Package companies_page builds the page.companies_page.v2 view model. It arranges figures that
// the compute functions of valuationapi and the owner wrote; it holds no formula, reads no clock
// of its own and makes no request. Design: docs/design/valuation.md#companies-page-band.
package companies_page

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	pageTitle     = "Companies"
	noReaderText  = "no band: reader not set"
	noMoatText    = "moat record not entered"
	noQuoteText   = "no quote"
	notBuiltText  = "%s block not built"
	staleAfterHrs = 24
)

// Subject is one company row. Issuer is the issuer text of the watch list (cik:<10 digits> or
// edinet:<code>), Symbol the contract symbol, HandPrice the text of the owner's own buy price,
// empty when none; it is shown as it is and never parsed or changed.
type Subject struct {
	Issuer, Market, Symbol, HandPrice string
}

// OwnerText gives the owner's own words. ok is false when none is entered.
type OwnerText interface {
	Moat(ctx context.Context, symbol string) (text string, ok bool, err error)
	FifthQuestion(ctx context.Context, symbol string) (text string, ok bool, err error)
}

// PriceOverride gives the owner's own price line of a symbol, shown labelled as his. ok is false
// when none is entered.
type PriceOverride interface {
	Line(ctx context.Context, symbol string) (price string, ok bool, err error)
}

// Config holds the inputs of Build. A nil reader or function gives a gap, never a guess.
// Subjects are the rows; Held, when set, names the issuers whose rows go to the holdings block.
type Config struct {
	Zone     *time.Location
	Today    func() contracts.Day
	Subjects []Subject
	Held     func(ctx context.Context) (map[string]bool, error)

	Read   store.ReadingViewReader
	Facts  store.FactReader
	Series store.SeriesReader
	Spot   func(ctx context.Context, symbol string, day contracts.Day) (string, bool, error)
	Offers func(ctx context.Context, symbol string, day contracts.Day) ([]string, error)

	StandardOf    valuationapi.StandardOfFunc
	BuildFilings  valuationapi.BuildFilingsFunc
	Band          valuationapi.BandFunc
	ImpliedGrowth valuationapi.ImpliedGrowthFunc
	ReturnAtPrice valuationapi.ReturnAtPriceFunc
	Questions     valuationapi.QuestionsFunc
	Diagnosis     valuationapi.DiagnosisFunc

	Text     OwnerText
	Override PriceOverride
}

type cell struct {
	Text       string `json:"text"`
	PriceState string `json:"price_state,omitempty"`
	HandPrice  string `json:"hand_price,omitempty"`
}

type column struct {
	Text    string `json:"text"`
	Numeric bool   `json:"numeric"`
}

type row struct {
	Cells []cell `json:"cells"`
}

type holdingRow struct {
	Cells []cell   `json:"cells"`
	Moat  []string `json:"moat"`
}

type gap struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type header struct {
	Title     string `json:"title"`
	DataDay   string `json:"data_day"`
	Line      string `json:"line"`
	HeldCount *int   `json:"held_count"`
}

type watchBlock struct {
	Title    string   `json:"title"`
	Columns  []column `json:"columns"`
	Rows     []row    `json:"rows"`
	Sentence *string  `json:"sentence"`
}

type holdingsBlock struct {
	Columns []column     `json:"columns"`
	Rows    []holdingRow `json:"rows"`
}

type stale struct {
	DataTime   string `json:"data_time"`
	StaleAfter string `json:"stale_after"`
	Text       string `json:"text"`
}

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

type viewModel struct {
	Schema        string         `json:"schema"`
	SchemaVersion int            `json:"schema_version"`
	RunID         string         `json:"run_id"`
	BuildID       string         `json:"build_id"`
	Scope         string         `json:"scope"`
	GeneratedAt   string         `json:"generated_at"`
	DataTimes     []sourceTime   `json:"data_times"`
	RunStatus     string         `json:"run_status"`
	Title         string         `json:"title"`
	Header        header         `json:"header"`
	Changes       *struct{}      `json:"changes"`
	Glance        *struct{}      `json:"glance"`
	Holdings      *holdingsBlock `json:"holdings"`
	Unvalued      *string        `json:"unvalued"`
	WatchList     watchBlock     `json:"watch_list"`
	Notes         []string       `json:"notes"`
	Gaps          []gap          `json:"gaps"`
	Stale         stale          `json:"stale"`
	Core          any            `json:"core"`
	Diagnosis     any            `json:"diagnosis"`
	Questions     any            `json:"questions"`
}

var zoneOwner = time.FixedZone("+08:00", 8*3600)

var columns = []column{{Text: "Company"}, {Text: "Price line", Numeric: true}, {Text: "Last", Numeric: true}}

// Build returns the page.companies_page.v2 JSON read through cfg. generatedAt is written in the
// zone of cfg (+08:00 when none). The price line cell of every row is read from the band; the
// core, diagnosis and questions blocks are built from the compute functions and are gaps while
// a reader or function they need is not set.
func Build(ctx context.Context, cfg Config, generatedAt time.Time) ([]byte, error) {
	if cfg.Today == nil {
		return nil, fmt.Errorf("companies_page: Config.Today is not set")
	}
	today := cfg.Today()
	var held map[string]bool
	if cfg.Held != nil {
		var err error
		if held, err = cfg.Held(ctx); err != nil {
			return nil, fmt.Errorf("companies_page: held: %w", err)
		}
	}
	vm := viewModel{
		Schema: "companies_page", SchemaVersion: 2,
		RunID: "companies-" + string(today), BuildID: "companies-" + string(today), Scope: "companies",
		DataTimes: []sourceTime{}, RunStatus: "ok", Title: pageTitle,
		Notes: []string{}, Gaps: []gap{},
		Core:      gap{Reason: "not_built", Text: fmt.Sprintf(notBuiltText, "core")},
		Diagnosis: gap{Reason: "not_built", Text: fmt.Sprintf(notBuiltText, "diagnosis")},
		Questions: gap{Reason: "not_built", Text: fmt.Sprintf(notBuiltText, "questions")},
	}
	at := generatedAt.In(cfg.zoneAt(generatedAt))
	vm.GeneratedAt = at.Format("2006-01-02T15:04:05-07:00")
	vm.Stale = stale{
		DataTime:   vm.GeneratedAt,
		StaleAfter: at.Add(staleAfterHrs * time.Hour).Format("2006-01-02T15:04:05-07:00"),
		Text:       "Data of " + string(today)[5:],
	}
	vm.WatchList = watchBlock{Title: "Watch list", Columns: columns, Rows: []row{}}
	var holds []holdingRow
	var unvalued string
	var why reasonGroups
	for _, s := range cfg.Subjects {
		pc, reason, err := cfg.priceCell(ctx, s, today)
		if err != nil {
			return nil, err
		}
		last := noQuoteText
		if cfg.Spot != nil {
			if q, ok, err := cfg.Spot(ctx, s.Symbol, today); err != nil {
				return nil, fmt.Errorf("companies_page: spot of %s: %w", s.Symbol, err)
			} else if ok && q != "" {
				last = q
			}
		}
		cells := []cell{{Text: s.Symbol}, pc, {Text: last}}
		if reason != "" {
			why.add(reason, s.Symbol)
		}
		if held[s.Issuer] {
			moat, err := cfg.moat(ctx, s.Symbol)
			if err != nil {
				return nil, err
			}
			holds = append(holds, holdingRow{Cells: cells, Moat: []string{moat}})
			continue
		}
		vm.WatchList.Rows = append(vm.WatchList.Rows, row{Cells: cells})
	}
	if !cfg.bandReady() && len(cfg.Subjects) > 0 {
		vm.Gaps = append(vm.Gaps, gap{Reason: "no_reader", Text: "price line: a reader or compute function is not set"})
	}
	if cfg.Held != nil {
		n := len(holds)
		vm.Holdings = &holdingsBlock{Columns: columns, Rows: append([]holdingRow{}, holds...)}
		vm.Header.HeldCount = &n
	}
	if unvalued = why.line(); unvalued != "" {
		vm.Unvalued = &unvalued
	}
	if err := cfg.fillBlocks(ctx, &vm, today); err != nil {
		return nil, err
	}
	vm.Header.Title = pageTitle
	vm.Header.DataDay = string(today)
	vm.Header.Line = fmt.Sprintf("%s · watching %d companies", string(today)[5:], len(cfg.Subjects))
	return json.Marshal(vm)
}

// fillBlocks builds the core, diagnosis and questions blocks; they stay gaps while a reader or
// function the band needs is not set.
func (c Config) fillBlocks(ctx context.Context, vm *viewModel, today contracts.Day) error {
	if !c.bandReady() {
		return nil
	}
	figs, err := c.gather(ctx, today)
	if err != nil {
		return err
	}
	if vm.Core, err = c.coreBlockOf(ctx, figs, today); err != nil {
		return err
	}
	vm.Diagnosis = c.diagnosisBlockOf(figs)
	vm.Questions, err = c.questionsBlockOf(ctx, figs, today)
	return err
}

// moat is the owner's moat text of a symbol, or the not-entered line.
func (c Config) moat(ctx context.Context, symbol string) (string, error) {
	if c.Text == nil {
		return noMoatText, nil
	}
	text, ok, err := c.Text.Moat(ctx, symbol)
	if err != nil {
		return "", fmt.Errorf("companies_page: moat of %s: %w", symbol, err)
	}
	if !ok || text == "" {
		return noMoatText, nil
	}
	return text, nil
}

// zoneAt is the zone of cfg when generatedAt is +08:00 in it, else +08:00: the schema fixes
// generated_at to +08:00.
func (c Config) zoneAt(t time.Time) *time.Location {
	if c.Zone == nil {
		return zoneOwner
	}
	if _, off := t.In(c.Zone).Zone(); off != 8*3600 {
		return zoneOwner
	}
	return c.Zone
}

// reasonGroups collects the companies of each unvalued reason, reasons in order of first appearance.
type reasonGroups struct {
	order     []string
	companies map[string][]string
}

func (g *reasonGroups) add(reason, symbol string) {
	if g.companies == nil {
		g.companies = map[string][]string{}
	}
	if _, seen := g.companies[reason]; !seen {
		g.order = append(g.order, reason)
	}
	g.companies[reason] = append(g.companies[reason], symbol)
}

// line is each reason once with its companies: "fewer than 5 years: A, B; no standard: C".
func (g *reasonGroups) line() string {
	groups := make([]string, 0, len(g.order))
	for _, r := range g.order {
		groups = append(groups, r+": "+strings.Join(g.companies[r], ", "))
	}
	return strings.Join(groups, "; ")
}
