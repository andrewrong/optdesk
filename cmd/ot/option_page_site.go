package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/option_boards"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
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
	symbols := cardSymbols(ctx, rd, day, nil)
	return optionPageWith(a, "ot-serve", rd.cands, rd.eod, rd.names, rd.openQuotes(), rd.live.state(ctx, symbols)).
		Build(ctx, nil, day, clk.Now().UTC())
}

// optionPageOf calls newOptionPage with the plain arguments (scope "options").
func optionPageOf(a *app, runID string, cands store.CandidateReader, eod store.OptionEodReader, names store.InstrumentNameReader) pages.Page {
	return optionPageWith(a, runID, cands, eod, names, nil, option_boards.RefreshState{})
}

// optionPageWith is optionPageOf with the overlay reader of the open and refresh rows and the state of the button.
func optionPageWith(a *app, runID string, cands store.CandidateReader, eod store.OptionEodReader, names store.InstrumentNameReader,
	quotes openquote.OptionOpenReader, refresh option_boards.RefreshState) pages.Page {
	return newOptionPage(option_boards.OptionConfig{
		RunID: runID, BuildID: buildIDOf(version), Scope: optionPageScope, Zone: a.profile.Zone,
		Candidates: cands, Eod: eod, Names: names, Quotes: quotes, Refresh: refresh,
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

// cardSymbols are the tickers of the cards of a page: the ok rank 1 short puts of the day, at most maxCards
// underlyings, each named by the name reader; match, when set, keeps the underlying a stock page shows.
func cardSymbols(ctx context.Context, rd siteReaders, day contracts.Day, match func(store.Candidate, string) bool) []string {
	if rd.cands == nil || rd.names == nil {
		return nil
	}
	rows, err := rd.cands.Candidates(ctx, day, liveCardKind)
	if err != nil {
		return nil
	}
	var picked []store.Candidate
	for _, r := range rows {
		if r.Status == "ok" {
			picked = append(picked, r)
		}
	}
	var ids []int64
	for _, r := range picked {
		if id := int64(r.UnderlyingID); !slices.Contains(ids, id) && len(ids) < 500 {
			ids = append(ids, id)
		}
	}
	ns, err := rd.names.InstrumentNames(ctx, ids)
	if err != nil {
		return nil
	}
	name := map[int64]string{}
	for _, n := range ns {
		name[n.InstrumentID] = n.Name
	}
	var out []string
	for _, r := range picked {
		s := strings.ToUpper(name[int64(r.UnderlyingID)])
		if s == "" || slices.Contains(out, s) {
			continue
		}
		if match == nil && r.RankInUnderlying != nil && *r.RankInUnderlying == 1 && len(out) < 6 {
			out = append(out, s)
		} else if match != nil && match(r, s) {
			out = append(out, s)
		}
	}
	return out
}
