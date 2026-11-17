package main

import (
	"context"
	"github.com/andrewrong/optdesk/internal/pages"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/valuelines"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/market_state_page"
	"github.com/andrewrong/optdesk/internal/pages/valuation_page"
	"github.com/andrewrong/optdesk/internal/pages/valuation_params_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// Design: docs/design/valuation-structure.md#7 valuation-wire-posts (Decisions 5, 9, 15, 16, 18).

// The routes of the four valuation pages under the site root, and the owner post paths.
const (
	valuationParamsRoute     = "valuation-params"
	valuationTableRoute      = "valuation"
	valuationCompanyRoute    = "valuation-company"
	valuationFormRoute       = "valuation-form"
	valuationPostRoute       = "valuation-post"
	valuationParamsPostRoute = "valuation-params-post"
)

// The names of the four page families, after the highest order pinned in pagereg_test.go (data_health_page 130).
const (
	valuationParamsName  = "valuation_params"
	valuationTableName   = "valuation_table"
	valuationCompanyName = "valuation_company"
	valuationFormName    = "valuation_form"
)

// siteValuationKey is the key of the valuation reader that the site opener puts in siteReaders.ext.
const siteValuationKey = "valuation_reader"

// valuationYieldWindowDays is how far back from the page day the latest DGS10 point is looked for.
const valuationYieldWindowDays = 14

func init() {
	registerPage(pageFamily{name: valuationParamsName, route: valuationParamsRoute, order: 140, build: buildValuationParams,
		named: valuationNamed(valuationParamsName, buildValuationParams)})
	registerPage(pageFamily{name: valuationTableName, route: valuationTableRoute, order: 150, build: buildValuationTable,
		named: valuationNamed(valuationTableName, buildValuationTable)})
	registerPage(pageFamily{name: valuationCompanyName, route: valuationCompanyRoute, order: 160, build: buildValuationCompany,
		named: valuationNamed(valuationCompanyName, buildValuationCompany)})
	registerPage(pageFamily{name: valuationFormName, route: valuationFormRoute, order: 170, build: buildValuationForm,
		named: valuationNamed(valuationFormName, buildValuationForm)})
	siteOpeners[siteValuationKey] = openValuationReader
}

// openValuationReader opens the valuation reader with the pages line. Without an opener, the line or in the demo
// profile it gives no reader and goes on: each page is then the gap store_unavailable.
func openValuationReader(_ context.Context, a *app, track func(io.Closer, error) bool) (any, bool) {
	if a.deps.openValuation == nil || a.profile.Name == "demo" {
		return nil, true
	}
	env, err := a.envFile()
	if err != nil {
		return nil, false
	}
	if env[pagesDSNName] == "" {
		return nil, true
	}
	r, _, c, err := a.deps.openValuation(env[pagesDSNName])
	if !track(c, err) {
		return nil, false
	}
	return r, true
}

// valuationReaderOf is the reader of the site, nil when none was opened.
func valuationReaderOf(rd siteReaders) valuation.Reader {
	r, _ := siteReaderOf[valuation.Reader](rd, siteValuationKey)
	return r
}

// valuationCloses fills valuation_page.CloseReader with the stock id reader and the bar reader (Decision 18).
type valuationCloses struct {
	ids  market_state_page.StockIDReader
	bars bar.PriceBarReader
}

func (c valuationCloses) StockIDs(ctx context.Context, names []string) ([]store.InstrumentName, error) {
	return c.ids.StockIDs(ctx, names)
}

func (c valuationCloses) PriceBars(ctx context.Context, id int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	return c.bars.PriceBars(ctx, id, basis, from, to)
}

// valuationClosesOf is the close reader of the site, nil (no close price) when the id or the bar reader is missing.
func valuationClosesOf(rd siteReaders) valuation_page.CloseReader {
	ids, _ := siteReaderOf[market_state_page.StockIDReader](rd, siteMarketStateIDsKey)
	if ids == nil || rd.bars == nil {
		return nil
	}
	return valuationCloses{ids: ids, bars: rd.bars}
}

// valuationPageInput is what the three views of valuation_page read, over reader (nil: the gap store_unavailable).
// A failed read is the same gap and never a 500.
func valuationPageInput(ctx context.Context, a *app, clk clock.Clock, rd siteReaders, reader valuation.Reader) (valuation_page.Input, error) {
	day, err := dayOrDefault(clk, "")
	if err != nil {
		return valuation_page.Input{}, err
	}
	in := valuation_page.Input{
		RunID: "ot-serve", Now: clk.Now().UTC(), Day: day, Calc: valuelines.New(), Closes: valuationClosesOf(rd),
	}
	if reader != nil {
		st, err := reader.Current(ctx)
		if err != nil {
			a.log.Warn("valuation read failed", "component", "ot", "status", "failed")
		} else {
			in.State, in.StateOK = st, true
		}
	}
	if rd.underlying != nil {
		if rows, err := rd.underlying.Current(ctx); err == nil {
			in.Underlying = rows
		}
	}
	if rd.edit != nil {
		tok := ownerDummyToken()
		in.Form = &valuation_page.Form{Expires: tok.Expires, MAC: tok.MAC}
	}
	return in, nil
}

// valuationSaved is the result line of ?saved=1, shown only to the owner.
func valuationSaved(ctx context.Context, rd siteReaders) string {
	if rd.edit != nil && queryOf(ctx).Get("saved") == "1" {
		return "Saved."
	}
	return ""
}

func buildValuationTable(ctx context.Context, a *app, clk clock.Clock, _ string, rd siteReaders) ([]byte, error) {
	in, err := valuationPageInput(ctx, a, clk, rd, valuationReaderOf(rd))
	if err != nil {
		return nil, err
	}
	in.Saved = valuationSaved(ctx, rd)
	return valuation_page.BuildTable(ctx, in)
}

// buildValuationCompany builds the company view of ?company=<key>: an unknown key is an empty page with a gap.
func buildValuationCompany(ctx context.Context, a *app, clk clock.Clock, _ string, rd siteReaders) ([]byte, error) {
	reader := valuationReaderOf(rd)
	in, err := valuationPageInput(ctx, a, clk, rd, reader)
	if err != nil {
		return nil, err
	}
	in.Company = queryOf(ctx).Get("company")
	in.Saved = valuationSaved(ctx, rd)
	if reader != nil && in.StateOK && in.Company != "" {
		if in.Reviews, err = reader.Reviews(ctx, in.Company); err != nil {
			a.log.Warn("valuation reviews read failed", "component", "ot", "status", "failed")
			in.Reviews = nil
		}
	}
	return valuation_page.BuildCompany(ctx, in)
}

func buildValuationForm(ctx context.Context, a *app, clk clock.Clock, _ string, rd siteReaders) ([]byte, error) {
	in, err := valuationPageInput(ctx, a, clk, rd, valuationReaderOf(rd))
	if err != nil {
		return nil, err
	}
	in.Company = queryOf(ctx).Get("company")
	in.Saved = valuationSaved(ctx, rd)
	return valuation_page.BuildForm(ctx, in)
}

func buildValuationParams(ctx context.Context, a *app, clk clock.Clock, _ string, rd siteReaders) ([]byte, error) {
	return valuationParamsDoc(ctx, a, clk, rd, valuationReaderOf(rd), "")
}

// valuationParamsDoc builds the market parameters page over reader; refusal is the text of a refused post.
func valuationParamsDoc(ctx context.Context, a *app, clk clock.Clock, rd siteReaders, reader valuation.Reader, refusal string) ([]byte, error) {
	day, err := dayOrDefault(clk, "")
	if err != nil {
		return nil, err
	}
	in := valuation_params_page.Input{RunID: "ot-serve", Now: clk.Now().UTC(), Calc: valuelines.New(), Error: refusal}
	if reader != nil {
		st, err1 := reader.Current(ctx)
		vs, err2 := reader.RVersions(ctx)
		if err1 != nil || err2 != nil {
			a.log.Warn("valuation read failed", "component", "ot", "status", "failed")
		} else {
			in.StoreOK, in.Versions, in.RTable, in.Margin = true, vs, st.RTable, st.Margin
		}
	}
	in.Yield = latestYield(ctx, rd.series, day)
	if rd.edit != nil {
		tok := ownerDummyToken()
		in.Form = &valuation_params_page.Form{Expires: tok.Expires, MAC: tok.MAC}
	}
	return valuation_params_page.Build(in)
}

// latestYield is the newest stored DGS10 point with a value, nil when there is none or the read failed.
func latestYield(ctx context.Context, ser store.SeriesReader, day contracts.Day) *valuation_params_page.Yield {
	if ser == nil {
		return nil
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil
	}
	from := contracts.Day(d.AddDate(0, 0, -valuationYieldWindowDays).Format(dayLayout))
	pts, err := ser.SeriesPoints(ctx, "DGS10", from, day)
	if err != nil {
		return nil
	}
	var best *valuation_params_page.Yield
	for _, p := range pts {
		if p.Value == nil {
			continue
		}
		od := contracts.Day(p.ObservationDate.Format(dayLayout))
		if best == nil || od >= best.Day {
			best = &valuation_params_page.Yield{Value: strconv.FormatFloat(*p.Value, 'f', -1, 64), Day: od}
		}
	}
	return best
}

// pageBuilder is the build of a pageFamily.
type pageBuilder = func(context.Context, *app, clock.Clock, string, siteReaders) ([]byte, error)

// valuationNamed is the named builder of ot page --name for one family: the readers of the site, the page built
// with the clock at the time asked for. The schema of the params page is its own.
func valuationNamed(name string, build pageBuilder) func(ctx context.Context, a *app) (pages.Page, func(), error) {
	return func(ctx context.Context, a *app) (pages.Page, func(), error) {
		rd, closeAll, err := openSiteReaders(ctx, a)
		if err != nil {
			return pages.Page{}, nil, err
		}
		schema := "page.valuation_page.v1"
		if name == valuationParamsName {
			schema = "page.valuation_params_page.v1"
		}
		b := func(ctx context.Context, _ store.SeriesReader, day contracts.Day, now time.Time) ([]byte, error) {
			return build(ctx, a, nowClock(now), string(day), rd)
		}
		return pages.Page{Meta: pages.Meta{Name: name, Schema: schema}, Build: b}, closeAll, nil
	}
}

// nowClock is a clock that stands at one instant.
type nowClock time.Time

func (c nowClock) Now() time.Time { return time.Time(c) }

// valuationPostOf is the handler of an owner post path of the valuation forms, nil for any other path.
func valuationPostOf(path string) func(*app, clock.Clock, siteReaders, pages.NewHandlerFunc, http.ResponseWriter, *http.Request) {
	switch path {
	case valuationPostRoute:
		return postOwnerValuation
	case valuationParamsPostRoute:
		return postOwnerValuationParams
	}
	return nil
}
