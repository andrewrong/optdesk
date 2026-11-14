package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/ease"
	"github.com/andrewrong/optdesk/internal/compute/liquidity"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/macro_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

// Design: docs/design/macro-events.md#Decisions 11.

// newMacroPageEvents builds the macro page as newMacroPage does, with events as the reader of the stored dates;
// a test passes a fake and opens no store.
func newMacroPageEvents(zone *time.Location, buildID string, read compute.ReadingFunc, events macroevent.Reader) pages.Page {
	cfg := macroPageConfig(zone, buildID, read)
	cfg.Events = events
	return macro_page.NewPage(cfg)
}

// macroPageConfig is the config of the macro page without events: newMacroPage and newMacroPageEvents both take it.
func macroPageConfig(zone *time.Location, buildID string, read compute.ReadingFunc) macro_page.Config {
	return macro_page.Config{
		RunID:           "ot-page",
		BuildID:         buildID,
		Scope:           "macro",
		Zone:            zone,
		Read:            read,
		Liquidity:       liquidity.Compute,
		LiquidityParams: liquidity.TrialParams(),
		Net:             ease.NetLiquidity,
		Indicators: []macro_page.Indicator{
			{SeriesID: "DGS10", Name: "10-year Treasury yield", SourceText: "FRED"},
		},
	}
}

// openMacroEvents opens the reader-only store on store.DSNEnvPages. The reader is nil, with a no-op close, for the
// demo profile, an empty line, or deps without the opener.
func openMacroEvents(ctx context.Context, a *app) (macroevent.Reader, func(), error) {
	if a.profile.Name == "demo" {
		return nil, func() {}, nil
	}
	md, ok := a.deps.ext[extMacroEvents].(macroEventsDeps)
	if !ok || md.openReader == nil {
		return nil, func() {}, nil
	}
	env, err := a.envFile()
	if err != nil {
		return nil, nil, err
	}
	dsn := env[store.DSNEnvPages]
	if dsn == "" {
		return nil, func() {}, nil
	}
	r, c, err := md.openReader(ctx, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open macro event reader with %s failed", store.DSNEnvPages)
	}
	return r, closeQuiet(c), nil
}

func closeQuiet(c io.Closer) func() { return func() { _ = c.Close() } }
