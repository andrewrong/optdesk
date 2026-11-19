package main

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/valuation_import_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/valuation-import.md#5 (Decisions 1, 2, 14).

// The route of the page and the owner post path under the site root, and the name of the page family.
const (
	valuationImportRoute     = "valuation-import"
	valuationImportPostRoute = "valuation-import-post"
	valuationImportName      = "valuation_import_page"
)

func init() {
	registerPage(pageFamily{name: valuationImportName, route: valuationImportRoute, order: 175, build: buildValuationImport,
		named: valuationImportNamed})
}

// buildValuationImport is the empty view: the file form. With --owner-edit off the page says the import is off.
func buildValuationImport(ctx context.Context, _ *app, _ clock.Clock, _ string, rd siteReaders) ([]byte, error) {
	in := valuation_import_page.Input{View: valuation_import_page.ViewEmpty}
	if rd.edit == nil {
		in.Gaps = []string{"Import is off: start ot serve with --owner-edit to upload a file."}
	}
	return valuation_import_page.Build(ctx, in)
}

// valuationImportNamed is the named builder of ot page --name valuation_import_page.
func valuationImportNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	rd, closeAll, err := openSiteReaders(ctx, a)
	if err != nil {
		return pages.Page{}, nil, err
	}
	b := func(ctx context.Context, _ store.SeriesReader, day contracts.Day, now time.Time) ([]byte, error) {
		return buildValuationImport(ctx, a, nowClock(now), string(day), rd)
	}
	return pages.Page{Meta: pages.Meta{Name: valuationImportName, Schema: "page.valuation_import_page.v1"}, Build: b}, closeAll, nil
}
