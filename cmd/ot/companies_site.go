package main

import (
	"context"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/companies_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// Design: docs/design/valuation.md#companies-route-band.

// siteCompaniesRoute is the address of the page companies_page under the site root.
const siteCompaniesRoute = "companies"

// companiesName is the name of the page companies_page, which pageNamed selects for ot page.
const companiesName = "companies_page"

func init() {
	registerPage(pageFamily{name: companiesName, route: siteCompaniesRoute, order: 50, build: buildCompanies, named: companiesPageNamed})
}

// companiesDemoSubjects are the two fixed rows of the demo profile, each with a hand price.
var companiesDemoSubjects = []companies_page.Subject{
	{Issuer: "cik:0000000001", Market: "US", Symbol: "DEMOA", HandPrice: "85.5"},
	{Issuer: "cik:0000000002", Market: "US", Symbol: "DEMOB", HandPrice: "42"},
}

// companiesSubjects are the rows of the page: the demo rows with the demo profile, else one per stored underlying
// with an issuer whose status is active, paused or retired (docs/design/underlyings.md Decision 6). No reader gives
// none; a read error is returned.
func companiesSubjects(ctx context.Context, a *app, rd siteReaders) ([]companies_page.Subject, error) {
	if a.profile.Name == "demo" {
		return companiesDemoSubjects, nil
	}
	if rd.underlying == nil {
		return nil, nil
	}
	rows, err := rd.underlying.Current(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]companies_page.Subject, 0, len(rows))
	for _, r := range rows {
		switch r.Status {
		case underlying.StatusActive, underlying.StatusPaused, underlying.StatusRetired:
		default:
			continue
		}
		if r.Issuer == "" {
			continue
		}
		out = append(out, companies_page.Subject{Issuer: r.Issuer, Market: r.Market, Symbol: r.Symbol, HandPrice: r.BuyPrice})
	}
	return out, nil
}

// siteFactsKey is the key of the fact reader that the site opener puts in siteReaders.ext.
const siteFactsKey = "companies_facts"

func init() { siteOpeners[siteFactsKey] = openCompaniesFacts }

// openCompaniesFacts opens the fact store on the pages line (role ot_ro_pages, reads the view only) and gives it as
// the reader of the page. Without the ext deps it gives no reader and goes on.
func openCompaniesFacts(ctx context.Context, a *app, track func(io.Closer, error) bool) (any, bool) {
	fd, ok := fundamentalsOf(a.deps)
	if !ok || fd.openFacts == nil {
		return nil, true
	}
	env, err := a.envFile()
	if err != nil {
		return nil, false
	}
	s, err := fd.openFacts(ctx, env[pagesDSNName])
	if !track(s, err) {
		return nil, false
	}
	return store.FactReader(s), true
}

// heldLookbackDays is how many days before the page day the positions are looked for when the day has none.
const heldLookbackDays = 6

// heldIssuers gives the Held function of the page: the issuers of the subjects whose symbol is the symbol of a
// stock position (SecType STK, non-zero Quantity) of alias, read for the newest of day and the six days before it
// that has any position row. A reader error is returned.
func heldIssuers(acc store.AccountReader, alias string, day contracts.Day, subjects []companies_page.Subject) func(context.Context) (map[string]bool, error) {
	return func(ctx context.Context) (map[string]bool, error) {
		t, err := time.Parse(dayLayout, string(day))
		if err != nil {
			return nil, err
		}
		var rows []store.Position
		for i := 0; i <= heldLookbackDays && len(rows) == 0; i++ {
			if rows, err = acc.Positions(ctx, alias, t.AddDate(0, 0, -i).Format(dayLayout)); err != nil {
				return nil, err
			}
		}
		symbols := map[string]bool{}
		for _, p := range rows {
			if p.SecType == "STK" && !isZeroQuantity(p.Quantity) {
				symbols[p.Symbol] = true
			}
		}
		held := map[string]bool{}
		for _, s := range subjects {
			if symbols[s.Symbol] {
				held[s.Issuer] = true
			}
		}
		return held, nil
	}
}

// isZeroQuantity reports whether decimal text q is zero ("0", "0.0", "-0"); text that is not a number is not zero.
func isZeroQuantity(q string) bool {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(q))
	return ok && r.Sign() == 0
}

// buildCompaniesAt builds companies_page for day from the readers rd (a nil reader gives a gap). With the compute
// functions in the deps the inputs that entry 24 left nil are set by newCompaniesPage; without them only the
// price line is filled, with the hand price, and no row has a band.
func buildCompaniesAt(ctx context.Context, a *app, rd siteReaders, day contracts.Day, now time.Time) ([]byte, error) {
	subjects, err := companiesSubjects(ctx, a, rd)
	if err != nil {
		return nil, err
	}
	cfg := companies_page.Config{
		Zone: a.profile.Zone, Today: func() contracts.Day { return day }, Subjects: subjects,
	}
	if rd.view != nil {
		cfg.Read = rd.view
	}
	if rd.noticeAccount != nil && a.profile.Name != "demo" {
		cfg.Held = heldIssuers(rd.noticeAccount, rd.aliasList()[0], day, subjects)
	}
	if fd, ok := fundamentalsOf(a.deps); ok {
		facts, _ := siteReaderOf[store.FactReader](rd, siteFactsKey)
		cfg = newCompaniesPage(cfg, companiesInputs{
			Facts: facts, Series: rd.series, Catalog: rd.catalog, Names: rd.names, Eod: rd.eod, Terms: rd.terms, Quotes: rd.quotes,
		}, fd.fn)
	}
	return companies_page.Build(ctx, cfg, now)
}

// buildCompanies builds companies_page per request for the day of dayFlag or today, from the view reader of the site.
func buildCompanies(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	return buildCompaniesAt(ctx, a, rd, day, clk.Now().UTC())
}

// companiesPageNamed returns companies_page for ot page and a close function: the site readers opened once with the
// pages line of .env, no reader with a missing line, or the demo view with the demo profile. The series reader that
// ot page opens, when given, replaces the one of the site readers.
func companiesPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	var rd siteReaders
	closeFn := func() {}
	switch a.profile.Name {
	case "demo":
		rd = siteReaders{view: demoView{}}
	default:
		env, err := a.envFile()
		if err != nil {
			return pages.Page{}, nil, err
		}
		if env[pagesDSNName] != "" {
			r, closeAll, err := openSiteReaders(ctx, a)
			if err != nil {
				return pages.Page{}, nil, err
			}
			rd, closeFn = r, closeAll
		}
	}
	build := func(ctx context.Context, ser store.SeriesReader, day contracts.Day, now time.Time) ([]byte, error) {
		r := rd
		if ser != nil {
			r.series = ser
		}
		return buildCompaniesAt(ctx, a, r, day, now)
	}
	return pages.Page{Meta: pages.Meta{Name: companiesName}, Build: build}, closeFn, nil
}
