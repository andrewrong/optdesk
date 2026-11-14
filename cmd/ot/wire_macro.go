package main

import (
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/macro_page"
)

// Design: docs/design/liquidity-view.md#7 liquidity-view-wire.

// newMacroPage builds the page macro_page in zone; the run id is a label inside the page JSON only. Its config
// (macroPageConfig) carries the liquidity function, so the page is version 2.
func newMacroPage(zone *time.Location, buildID string, read compute.ReadingFunc) pages.Page {
	return macro_page.NewPage(macroPageConfig(zone, buildID, read))
}
