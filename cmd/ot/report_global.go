package main

import (
	"context"

	"github.com/andrewrong/optdesk/internal/jobs/barsrefresh"
	"github.com/andrewrong/optdesk/internal/pages/report_blocks"
	"github.com/andrewrong/optdesk/internal/store/instrument"
)

// Design: docs/design/report-global-assets.md#Issues entry 5 report-global-assets-wire.

// extReportGlobal is the key of the report's global assets deps in extDeps.
const extReportGlobal = "report_global"

// openStockIDsFunc opens the stock id reader with the pages line.
type openStockIDsFunc func(ctx context.Context, dsn string) (stockIDStore, error)

// stockIDStore is the stock id reader with its Close.
type stockIDStore interface {
	report_blocks.StockIDReader
	Close() error
}

// reportGlobalDeps is the ext deps of the report page: the opener of the stock id reader. A nil opener leaves
// the reader nil, which the block shows as the gap no_reader.
type reportGlobalDeps struct{ openStockIDs openStockIDsFunc }

// reportReaders are the readers of the report blocks: the site readers and the stock id reader of the
// global assets block.
type reportReaders struct {
	siteReaders
	stockIDs report_blocks.StockIDReader
}

func init() {
	extDeps[extReportGlobal] = func() any { return reportGlobalDeps{openStockIDs: openStockIDsDefault} }
}

// openStockIDsDefault binds instrument.OpenStockIDs to openStockIDsFunc. On error it returns a nil interface, not
// a *instrument.StockIDs nil held in one.
func openStockIDsDefault(ctx context.Context, dsn string) (stockIDStore, error) {
	s, err := instrument.OpenStockIDs(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// reportStockIDOpener returns the opener of the stock id reader, or nil when the key is missing, holds another
// type or holds no opener. It never panics.
func reportStockIDOpener(a *app) openStockIDsFunc {
	d, ok := a.deps.ext[extReportGlobal].(reportGlobalDeps)
	if !ok {
		return nil
	}
	return d.openStockIDs
}

// reportAssets returns the listed assets of the global assets block from the manifest of bars_refresh, so the
// collector and the page share one list. An unreadable manifest gives none (the block is then no_reader).
func reportAssets() []report_blocks.Asset {
	m, err := barsrefresh.DefaultManifest()
	if err != nil {
		return nil
	}
	out := make([]report_blocks.Asset, 0, len(m.Stocks))
	for _, s := range m.Stocks {
		out = append(out, report_blocks.Asset{Symbol: s.Symbol, Label: s.Label, Group: s.Group})
	}
	return out
}
