package main

import (
	"context"
	"io"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/basecheck"
	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/base_data_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/basefact"
	"github.com/andrewrong/optdesk/internal/store/basefactpg"
	"github.com/andrewrong/optdesk/internal/store/coverage"
	"github.com/andrewrong/optdesk/internal/store/ops"
)

// Design: docs/design/base-data-page.md#10.

// siteBaseDataRoute is the address of the page base_data_page under the site root.
const siteBaseDataRoute = "base-data"

const (
	// baseDataName is the name of the page for ot page.
	baseDataName = "base_data_page"
	// siteBaseDataFactsKey and siteBaseDataCoverageKey are the keys of the readers the openers put in siteReaders.ext.
	siteBaseDataFactsKey    = "base_data_facts"
	siteBaseDataCoverageKey = "base_data_coverage"
	// extBaseData is the name of the openers in the ext deps.
	extBaseData = "base_data"
)

// baseDataDeps are the openers of the page and the injected verdict function.
type baseDataDeps struct {
	openFacts    basefact.OpenFunc
	openCoverage coverage.OpenFunc
	eval         basecheckapi.Func
}

func init() {
	extDeps[extBaseData] = func() any {
		return baseDataDeps{openFacts: basefactpg.Open, openCoverage: ops.OpenCoverage, eval: basecheck.Evaluate}
	}
	registerPage(pageFamily{name: baseDataName, route: siteBaseDataRoute, order: 135, build: buildBaseData, named: baseDataPageNamed})
	siteOpeners[siteBaseDataFactsKey] = func(ctx context.Context, a *app, track func(io.Closer, error) bool) (any, bool) {
		return openBaseData(ctx, a, track, func(ctx context.Context, d baseDataDeps, dsn string) (io.Closer, error) {
			if d.openFacts == nil {
				return nil, nil //nolint:nilnil // no opener: no reader, the page shows the gap
			}
			return d.openFacts(ctx, dsn)
		})
	}
	siteOpeners[siteBaseDataCoverageKey] = func(ctx context.Context, a *app, track func(io.Closer, error) bool) (any, bool) {
		return openBaseData(ctx, a, track, func(ctx context.Context, d baseDataDeps, dsn string) (io.Closer, error) {
			if d.openCoverage == nil {
				return nil, nil //nolint:nilnil // no opener: no reader, the page shows the gap
			}
			return d.openCoverage(ctx, dsn)
		})
	}
}

// openBaseData opens one reader of the page with the pages line. Without the deps, an opener or the line it gives no
// reader and goes on: the page is then a gap.
func openBaseData(ctx context.Context, a *app, track func(io.Closer, error) bool,
	open func(context.Context, baseDataDeps, string) (io.Closer, error),
) (any, bool) {
	d, ok := a.deps.ext[extBaseData].(baseDataDeps)
	if !ok {
		return nil, true
	}
	env, err := a.envFile()
	if err != nil {
		return nil, false
	}
	if env[pagesDSNName] == "" {
		return nil, true
	}
	r, err := open(ctx, d, env[pagesDSNName])
	if r == nil && err == nil {
		return nil, true
	}
	if !track(r, err) {
		return nil, false
	}
	return r, true
}

// buildBaseData builds base_data_page per request.
func buildBaseData(ctx context.Context, a *app, clk clock.Clock, _ string, rd siteReaders) ([]byte, error) {
	facts, _ := siteReaderOf[basefact.Reader](rd, siteBaseDataFactsKey)
	cov, _ := siteReaderOf[coverage.Reader](rd, siteBaseDataCoverageKey)
	return base_data_page.Build(ctx, base_data_page.Input{Now: clk.Now().UTC(), Facts: facts, Cov: cov, Eval: baseDataEval(a)})
}

// baseDataEval is the injected verdict function: the one of the deps, basecheck.Evaluate when a test set none.
func baseDataEval(a *app) basecheckapi.Func {
	if d, ok := a.deps.ext[extBaseData].(baseDataDeps); ok && d.eval != nil {
		return d.eval
	}
	return basecheck.Evaluate
}

// baseDataPageNamed gives the page to ot page: the demo profile and a missing line give no reader.
func baseDataPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	var rd siteReaders
	closeFn := func() {}
	if a.profile.Name != "demo" {
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
	build := func(ctx context.Context, _ store.SeriesReader, _ contracts.Day, now time.Time) ([]byte, error) {
		facts, _ := siteReaderOf[basefact.Reader](rd, siteBaseDataFactsKey)
		cov, _ := siteReaderOf[coverage.Reader](rd, siteBaseDataCoverageKey)
		return base_data_page.Build(ctx, base_data_page.Input{Now: now.UTC(), Facts: facts, Cov: cov, Eval: baseDataEval(a)})
	}
	return pages.Page{Meta: pages.Meta{Name: baseDataName, Schema: "page.base_data_page.v1"}, Build: build}, closeFn, nil
}
