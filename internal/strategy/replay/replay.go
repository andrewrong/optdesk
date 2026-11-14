// Package replay is the pure engine behind replayapi.Replayer (docs/design/strategy-replay.md, Decisions 2 to 6, 8,
// 9 and 12): it prices every short lot held to expiry, walks the stored quotes of each lot under each early-close
// variant, and builds the report lines. Standard library only; no clock, file or network.
package replay

import (
	"sort"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy/replayapi"
)

// HoldVariant is the name of the variant that never closes; the engine prices it on every lot without a quote.
const HoldVariant = "hold"

type engine struct{}

// New returns the replayer.
func New() replayapi.Replayer { return engine{} }

// Replay prices each lot under hold and under each variant. Lots are taken in id order and outcomes sort by lot id
// then variant name, so the result is a function of the arguments.
func (engine) Replay(lots []replayapi.Lot, mkt replayapi.Market, variants []replayapi.Variant,
	sessionDays []contracts.Day, asOf contracts.Day, minLots int) replayapi.Report {
	ls := append([]replayapi.Lot(nil), lots...)
	sort.SliceStable(ls, func(i, j int) bool { return ls[i].ID < ls[j].ID })
	days := append([]contracts.Day(nil), sessionDays...)
	sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })

	rep := replayapi.Report{Lots: len(ls), Excluded: map[string]int{}}
	quoteDays := map[contracts.Day]bool{}
	var outs []replayapi.Outcome
	for _, l := range ls {
		if l.Fate == "open" {
			rep.StillOpen++
		}
		window := lotDays(l, days)
		if n := noteQuotes(l, mkt, window, quoteDays); n > 0 {
			rep.LotsWithQuote++
			rep.LotDays += n
		}
		hold := holdOutcome(l, mkt, asOf)
		outs = append(outs, hold)
		count(rep.Excluded, hold)
		for _, v := range variants {
			o := walkVariant(l, v, mkt, window, hold)
			outs = append(outs, o)
			count(rep.Excluded, o)
		}
	}
	rep.QuoteDays = len(quoteDays)
	for d := range quoteDays {
		if rep.FirstQuoteDay == "" || d < rep.FirstQuoteDay {
			rep.FirstQuoteDay = d
		}
		if d > rep.LastQuoteDay {
			rep.LastQuoteDay = d
		}
	}
	sort.SliceStable(outs, func(i, j int) bool {
		if outs[i].LotID != outs[j].LotID {
			return outs[i].LotID < outs[j].LotID
		}
		return outs[i].Variant < outs[j].Variant
	})
	rep.Outcomes = outs
	rep.Lines = buildLines(ls, outs, variants, minLots)
	return rep
}

func count(excluded map[string]int, o replayapi.Outcome) {
	if !o.Priced {
		excluded[o.Variant+":"+o.Reason]++
	}
}

// noteQuotes records the days after entry up to expiry on which the lot's contract has a stored quote and returns
// how many there are.
func noteQuotes(l replayapi.Lot, mkt replayapi.Market, window []contracts.Day, seen map[contracts.Day]bool) int {
	n := 0
	for _, d := range window {
		if _, ok := mkt.Quotes[l.InstrumentID][d]; ok {
			seen[d] = true
			n++
		}
	}
	return n
}
