package main

import (
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy/replayapi"
)

// replayName is a variant name with the hash of its parameters.
type replayName struct{ Name, Hash string }

// replayHeader is what the header of the output names besides the report.
type replayHeader struct {
	Lots     string
	AsOf     contracts.Day
	Variants []replayName
	MinLots  int
}

// printReplay prints the first and last stored quote day, the header, the counts and one row per line, a lot
// count beside each figure. Order is the order of the report; no map is iterated.
func printReplay(w io.Writer, h replayHeader, rep replayapi.Report) {
	p := func(format string, args ...any) { _, _ = fmt.Fprintf(w, format+"\n", args...) }
	if rep.FirstQuoteDay == "" {
		p("stored quote days: none")
	} else {
		p("stored quote days: first=%s last=%s", rep.FirstQuoteDay, rep.LastQuoteDay)
	}
	p("lots set: %s", h.Lots)
	p("as-of day: %s", h.AsOf)
	for _, v := range h.Variants {
		p("variant %s params_hash=%s", v.Name, v.Hash)
	}
	p("early assignment is not modelled; a day without a stored quote is a gap, not a guess")
	p("the stored quote is the morning snapshot 90 minutes after the open; settlement is the close")
	p("a rolled lot's actual does not include the lot it was rolled into")
	p("coverage: lots=%d with_quote_after_entry=%d lot_days=%d quote_days=%d", rep.Lots, rep.LotsWithQuote, rep.LotDays, rep.QuoteDays)
	p("still open: %d", rep.StillOpen)
	for _, k := range slices.Sorted(maps.Keys(rep.Excluded)) {
		p("excluded %s: %d", k, rep.Excluded[k])
	}
	for _, l := range rep.Lines {
		line := fmt.Sprintf("set=%s variant=%s currency=%s lots=%d", l.Set, l.Variant, l.Currency, l.Lots)
		if l.ParamHash != "" {
			line += " params_hash=" + l.ParamHash
		}
		line += fmt.Sprintf(" cycles=%d closed_early=%d held=%d assigned=%d rolled=%d in_the_money=%d",
			l.Cycles, l.ClosedEarly, l.Held, l.Assigned, l.Rolled, l.InTheMoney)
		line += " kept_share=" + figure(l.KeptShare, l.Lots) + " mean_gross=" + figure(l.MeanGross, l.Lots) +
			" worst_gross=" + figure(l.WorstGross, l.Lots) + " hold_kept_share=" + figure(l.HoldKeptShare, l.Lots) +
			" actual_kept_share=" + figure(l.ActualKeptShare, l.Lots)
		if l.UnderMinimum {
			line += fmt.Sprintf(" under %d [T]: no conclusion", h.MinLots)
		}
		p("%s", line)
	}
}

// figure is value (n=lots), or n/a (n=lots) when the line has no such figure.
func figure(v string, lots int) string {
	if v == "" {
		v = "n/a"
	}
	return fmt.Sprintf("%s (n=%d)", v, lots)
}
