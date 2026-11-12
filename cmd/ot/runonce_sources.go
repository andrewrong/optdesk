package main

import (
	"fmt"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
)

// fredSource is the FRED source, or the stand-in that answers permanent when the key is empty.
func fredSource(d deps, cfg runOnceConfig, clk clock.Clock) (sources.Source, error) {
	if cfg.fredKey == "" {
		return noKeySource{}, nil
	}
	src, err := d.newFRED(sources.FREDConfig{BaseURL: fredBaseURL, APIKey: cfg.fredKey, Clock: clk})
	if err != nil {
		return nil, fmt.Errorf("build FRED source failed: %w", err)
	}
	return src, nil
}

// jpSymbols is the symbol of each JP entry of the watch list, in file order: the codes one J-Quants call each.
func jpSymbols(watch []jobs.WatchEntry) []string {
	var out []string
	for _, e := range watch {
		if e.Market == "JP" {
			out = append(out, e.Contract.Symbol)
		}
	}
	return out
}
