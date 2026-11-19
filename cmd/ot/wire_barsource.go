package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/sources/yfinance"
)

// Design: docs/design/base-data-page.md#23.

// barSources is the registry of bar sources by the id of the manifest's [source] table; a new source adds one line.
var barSources = map[string]sources.NewBarSourceFunc{
	"yfinance": yfinance.New,
}

// refbarsCommand is the command of the bar source up to the arguments the source adds:
// `uv run --locked --project <project> python -m optdesk.refbars`.
func refbarsCommand(uv, project string) []string {
	return []string{uv, "run", "--locked", "--project", project, "python", "-m", "optdesk.refbars"}
}

// barSource builds the bar source of the registry entry id over the uv program and the pyside project directory of
// calendar_refresh. An id the registry lacks is an error that names it and lists the known ids.
func (f jobFactory) barSource(id string) (sources.BarSource, error) {
	return f.barSourceIn(barSources, id)
}

// barSourceIn is barSource over the registry reg; a test passes its own.
func (f jobFactory) barSourceIn(reg map[string]sources.NewBarSourceFunc, id string) (sources.BarSource, error) {
	build, ok := reg[id]
	if !ok {
		known := make([]string, 0, len(reg))
		for k := range reg {
			known = append(known, k)
		}
		sort.Strings(known)
		return nil, fmt.Errorf("recon_bars: unknown bar source id %q, known ids: %s", id, strings.Join(known, ", "))
	}
	uv := f.uv
	if uv == "" {
		uv = "uv"
	}
	dir, err := f.calendarProjectDir()
	if err != nil {
		return nil, err
	}
	return build(sources.BarConfig{Command: refbarsCommand(uv, dir)})
}
