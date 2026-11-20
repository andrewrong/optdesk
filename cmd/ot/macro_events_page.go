package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/ease"
	"github.com/andrewrong/optdesk/internal/compute/liquidity"
	"github.com/andrewrong/optdesk/internal/compute/ruleparam"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/macro_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

// Design: docs/design/macro-events.md#Decisions 11.

// newMacroPageEvents builds the macro page as newMacroPage does, with events as the reader of the stored dates;
// a test passes a fake and opens no store.
func newMacroPageEvents(zone *time.Location, buildID string, read compute.ReadingFunc, events macroevent.Reader) pages.Page {
	return newMacroPageOwner(zone, buildID, read, events, nil)
}

// newMacroPageOwner is newMacroPageEvents with the owner's settings reader, which gives the option window; a nil
// owner gives the registry default.
func newMacroPageOwner(zone *time.Location, buildID string, read compute.ReadingFunc, events macroevent.Reader, owner ownerset.Reader) pages.Page {
	cfg := macroPageConfig(zone, buildID, read)
	cfg.Events = events
	cfg.Window = ownerWindow(owner)
	return macro_page.NewPage(cfg)
}

// windowKey is the rule parameter that is the option window.
const windowKey = "max_dte"

// registryWindow is the default window of the rule registry and the text it is resolved from.
func registryWindow() int {
	n, _ := strconv.Atoi(ruleparam.New().Resolve(nil)[windowKey])
	return n
}

// ownerWindow returns the window function of the macro page: the owner's stored max_dte resolved by the rule
// registry and true, or the registry default and false when none is stored, the reader is nil or a read fails.
func ownerWindow(owner ownerset.Reader) macro_page.WindowFunc {
	return func(ctx context.Context, at time.Time) (int, bool) {
		if owner == nil {
			return registryWindow(), false
		}
		params, err := owner.Params(ctx, at)
		if err != nil {
			return registryWindow(), false
		}
		stored := make(map[string]string, len(params))
		for _, p := range params {
			stored[p.Key] = p.Value
		}
		if stored[windowKey] == "" {
			return registryWindow(), false
		}
		n, err := strconv.Atoi(ruleparam.New().Resolve(stored)[windowKey])
		if err != nil || n <= 0 {
			return registryWindow(), false
		}
		return n, true
	}
}

// openMacroOwner opens the owner settings reader on store.DSNEnvPages as openMacroEvents opens the event reader. The
// reader is nil, with a no-op close, for the demo profile, an empty line, deps without the opener, or a failed open:
// the window is then the registry default, which the page says.
func openMacroOwner(ctx context.Context, a *app) (ownerset.Reader, func()) {
	if a.profile.Name == "demo" || a.deps.openOwnerSet == nil {
		return nil, func() {}
	}
	env, err := a.envFile()
	if err != nil || env[store.DSNEnvPages] == "" {
		return nil, func() {}
	}
	o, err := a.deps.openOwnerSet(ctx, env[store.DSNEnvPages])
	if err != nil {
		return nil, func() {}
	}
	return o, closeQuiet(o)
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
		Groups:          macroGroups(),
		Window:          ownerWindow(nil),
	}
}

// macroGroups is the three groups of the Indicators card, in the order of Decision 5 of docs/design/macro-page-v2.md.
func macroGroups() []macro_page.Group {
	row := func(id, name string) macro_page.Indicator {
		return macro_page.Indicator{SeriesID: id, Name: name, SourceText: "FRED"}
	}
	return []macro_page.Group{
		{Key: "rates", Indicators: []macro_page.Indicator{
			row("DFF", "Effective federal funds rate"), row("DGS3MO", "3-month Treasury yield"),
			row("DGS2", "2-year Treasury yield"), row("DGS10", "10-year Treasury yield"),
			row("T10Y2Y", "10-year minus 2-year spread"), row("T10Y3M", "10-year minus 3-month spread"),
			row("T10YIE", "10-year breakeven inflation"),
		}},
		{Key: "credit", Indicators: []macro_page.Indicator{
			row("BAMLH0A0HYM2", "High-yield credit spread"), row("NFCI", "Chicago Fed financial conditions index"),
		}},
		{Key: "dollar_japan", Indicators: []macro_page.Indicator{
			row("DTWEXBGS", "Broad dollar index"), row("DEXJPUS", "USD/JPY"),
			row("IRLTLT01JPM156N", "Japan 10-year yield"),
		}},
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
