package option_tracking_page

import (
	"context"
	"fmt"
	"strconv"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

// The quote coverage of an open row: of the trading days from the later of its opening day and
// the first stored quote of the contract (the as-of day included), on how many a two-sided quote (bid and ask both stored and above
// zero) is stored. The day comes from obs.option_day through Config.Days; a day with no row there
// is looked up in option_eod. Counts only: nothing is estimated and no gap day is filled.
const (
	coverageID    = "coverage"
	coverageLabel = "Quote coverage"
	coverageHint  = "Trading days from the later of the opening day and the first stored quote of the " +
		"contract, on which a two-sided quote (bid and ask both above zero) is stored"

	reasonOneSide = "one side only"
	reasonNoQuote = "no quote stored"
)

type dayKey struct {
	id  int64
	day contracts.Day
}

// lotCoverage is the coverage of one open row: days counted and the gap days, oldest first.
type lotCoverage struct {
	days    int
	gapDays []contracts.Day
}

// coverageOf returns the coverage cell of each open row by position id and the note of the
// account ("" when no row has a count). With a nil Config.Days every cell is the gap no_reader.
func coverageOf(ctx context.Context, c Config, in trackapi.TrackInput, open []opt) (map[string]any, string, error) {
	cells := make(map[string]any, len(open))
	if len(open) == 0 {
		return cells, "", nil
	}
	if c.Days == nil {
		for _, o := range open {
			cells[o.PositionID] = readerGap()
		}
		return cells, "", nil
	}
	contractOf := make(map[string]string, len(in.Positions))
	for _, p := range in.Positions {
		contractOf[p.PositionID] = p.Contract
	}
	type lot struct {
		o    opt
		id   int64
		span []contracts.Day
	}
	var lots []lot
	var ids []int64
	symbols := map[string]bool{}
	var from contracts.Day
	for _, o := range open {
		id, err := strconv.ParseInt(contractOf[o.PositionID], 10, 64)
		span := sessionsFrom(in.SessionDays, o.OpenedOn)
		if err != nil || len(span) == 0 {
			cells[o.PositionID] = codeGap(codeNoValue)
			continue
		}
		lots = append(lots, lot{o, id, span})
		ids = append(ids, id)
		symbols[o.Underlying] = true
		if from == "" || span[0] < from {
			from = span[0]
		}
	}
	if len(lots) == 0 {
		return cells, "", nil
	}
	stored, err := dayRows(ctx, c, ids, from, in.AsOf)
	if err != nil {
		return nil, "", err
	}
	eod := &eodLoader{ctx: ctx, c: c, symbols: symbols, from: from, to: in.AsOf}
	var gapTotal, dayTotal int
	for _, l := range lots {
		lc, err := coverFromFirstQuote(l.id, l.span, stored, eod)
		if err != nil {
			return nil, "", err
		}
		cells[l.o.PositionID] = lc.cell()
		gapTotal += len(lc.gapDays)
		dayTotal += lc.days
	}
	note := in.Alias + noteSep + " " + strconv.Itoa(gapTotal) + " of " + strconv.Itoa(dayTotal) +
		" lot-days have no two-sided quote since the first stored quote"
	return cells, note, nil
}

// eodLoader reads the option_eod sides once, and only when a day has no option_day row.
type eodLoader struct {
	ctx      context.Context
	c        Config
	symbols  map[string]bool
	from, to contracts.Day
	sides    map[dayKey]string
}

func (e *eodLoader) get() (map[dayKey]string, error) {
	if e.sides != nil {
		return e.sides, nil
	}
	var err error
	e.sides, err = eodSides(e.ctx, e.c, e.symbols, e.from, e.to)
	return e.sides, err
}

// coverFromFirstQuote counts the days of one lot from the later of its opening day and the first day on
// which a quote with at least one side is stored (the lot's span starts at its opening day), and
// names its gap days. A lot with no stored quote at all is counted over its whole span.
func coverFromFirstQuote(id int64, span []contracts.Day, stored map[dayKey]optionday.Day, eod *eodLoader) (lotCoverage, error) {
	reasons := make([]string, len(span))
	first := -1
	for i, d := range span {
		k := dayKey{id, d}
		if row, ok := stored[k]; ok {
			reasons[i] = sideReason(deref(row.Bid), deref(row.Ask))
		} else {
			m, err := eod.get()
			if err != nil {
				return lotCoverage{}, err
			}
			r, ok := m[k]
			if !ok {
				r = reasonNoQuote
			}
			reasons[i] = r
		}
		if first < 0 && reasons[i] != reasonNoQuote {
			first = i
		}
	}
	if first < 0 {
		first = 0
	}
	lc := lotCoverage{days: len(span) - first}
	for i := first; i < len(span); i++ {
		if reasons[i] != "" {
			lc.gapDays = append(lc.gapDays, span[i])
		}
	}
	return lc, nil
}

// sessionsFrom is the session days not before opened, in the order given.
func sessionsFrom(days []contracts.Day, opened contracts.Day) []contracts.Day {
	if opened == "" {
		return nil
	}
	for i, d := range days {
		if d >= opened {
			return days[i:]
		}
	}
	return nil
}

// sideReason is "" for a two-sided quote, else why the day is a gap. A quote with neither side
// above zero is no quote.
func sideReason(bid, ask string) string {
	b, a := positive(bid), positive(ask)
	switch {
	case b && a:
		return ""
	case b || a:
		return reasonOneSide
	}
	return reasonNoQuote
}

func positive(text string) bool {
	if text == "" {
		return false
	}
	d, err := decimal.NewFromString(text)
	return err == nil && d.IsPositive()
}

// dayRows reads the option_day rows of ids in chunks.
func dayRows(ctx context.Context, c Config, ids []int64, from, to contracts.Day) (map[dayKey]optionday.Day, error) {
	out := map[dayKey]optionday.Day{}
	for _, ch := range chunks(ids) {
		rows, err := c.Days.Days(ctx, ch, from, to)
		if err != nil {
			return nil, fmt.Errorf("option_tracking_page: read option_day: %w", err)
		}
		for _, r := range rows {
			out[dayKey{r.InstrumentID, r.SessionDay}] = r
		}
	}
	return out, nil
}

// eodSides maps (instrument, day) of the option_eod rows of the symbols to the reason of the day
// ("" for two sides above zero). A missing reader gives an empty map, so every such day is a gap.
func eodSides(ctx context.Context, c Config, symbols map[string]bool, from, to contracts.Day) (map[dayKey]string, error) {
	out := map[dayKey]string{}
	if c.Eod == nil || c.Catalog == nil || c.Names == nil {
		return out, nil
	}
	ids, _, err := underlyingIDs(ctx, c, from, symbols)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		rows, err := c.Eod.OptionEod(ctx, id, from, to)
		if err != nil {
			return nil, fmt.Errorf("option_tracking_page: read option_eod of %d: %w", id, err)
		}
		for _, r := range rows {
			d := contracts.Day(r.SessionDay.Format(dayLayout))
			if d < from || d > to {
				continue
			}
			out[dayKey{int64(r.InstrumentID), d}] = sideReason(decimalText(r.Bid), decimalText(r.Ask))
		}
	}
	return out, nil
}

// cell is "n of m days", toned down when there is any gap; the gap days are not listed.
func (lc lotCoverage) cell() valueCell {
	v := textCell(strconv.Itoa(lc.days-len(lc.gapDays)) + " of " + strconv.Itoa(lc.days) + " days")
	if len(lc.gapDays) > 0 {
		v.Tone = "down"
	}
	return v
}
