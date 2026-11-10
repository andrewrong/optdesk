package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/option_boards"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/option-page-builders.md#5-option-page-route.

// siteOptionRoute is the address of the page option_page under the site root.
const siteOptionRoute = "options"

// optionPageName is the name of the page option_page, which pageNamed selects for ot page.
const optionPageName = "option_page"

// optionPageScope is the scope the page carries (docs/design/option-page-builders.md, Decision 9).
const optionPageScope = "options"

// newOptionPage builds the page option_page; a test replaces it. The page adds no field to deps.
var newOptionPage = option_boards.NewOptionPage

func init() {
	registerPage(pageFamily{name: optionPageName, route: siteOptionRoute, order: 40,
		build: buildOptionPage, named: optionPageNamed})
}

// buildOptionPage builds option_page per request for the day of dayFlag or today in the zone of the profile,
// from the candidate and option_eod readers of the site. A nil zone is an error.
func buildOptionPage(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New(optionPageName + ": no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	return optionPageOf(a, "ot-serve", rd.cands, rd.eod, rd.names).Build(ctx, nil, day, clk.Now().UTC())
}

// optionPageOf calls newOptionPage with the plain arguments (scope "options").
func optionPageOf(a *app, runID string, cands store.CandidateReader, eod store.OptionEodReader, names store.InstrumentNameReader) pages.Page {
	return newOptionPage(option_boards.OptionConfig{
		RunID: runID, BuildID: buildIDOf(version), Scope: optionPageScope, Zone: a.profile.Zone,
		Candidates: cands, Eod: eod, Names: names,
	})
}

// optionPageNamed returns option_page for ot page and a close function: the candidate and option_eod stores
// opened once with the pages line of .env, nil readers with a missing line (the page shows its gap no_reader),
// or the demo readers with the demo profile (no store). A nil zone is an error raised before any store is opened.
func optionPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New(optionPageName + " needs the owner's zone: " + zoneEnvName + " is not set")
	}
	if a.profile.Name == "demo" {
		return optionPageOf(a, "ot-page", demoCandidates{}, demoOption{}, demoOption{}), func() {}, nil
	}
	env, err := a.envFile()
	if err != nil {
		return pages.Page{}, nil, err
	}
	dsn := env[pagesDSNName]
	if dsn == "" {
		return optionPageOf(a, "ot-page", nil, nil, nil), func() {}, nil
	}
	cands, err := a.deps.openCandidate(ctx, dsn)
	if err != nil {
		return pages.Page{}, nil, fmt.Errorf("open candidate store with %s failed", pagesDSNName)
	}
	eod, err := a.deps.openOptionEod(ctx, dsn)
	if err != nil {
		_ = cands.Close()
		return pages.Page{}, nil, fmt.Errorf("open option_eod store with %s failed", pagesDSNName)
	}
	names, err := a.deps.openInstrumentName(ctx, dsn)
	if err != nil {
		_ = eod.Close()
		_ = cands.Close()
		return pages.Page{}, nil, fmt.Errorf("open instrument name store with %s failed", pagesDSNName)
	}
	return optionPageOf(a, "ot-page", cands, eod, names), func() { _ = names.Close(); _ = eod.Close(); _ = cands.Close() }, nil
}
