package option_candidates_page

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionfigapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// maxRateAgeDays is the max_rate_age_days of job.toml: the oldest cash rate, in days before the
// session day, a figure may use (Decision 4).
const maxRateAgeDays = 7

const (
	rateSeries     = "DGS3MO"
	rateWindowDays = 14
	itmID          = "itm"
	excessID       = "excess"
)

// rateRead is the cash rate of one group: percent text and the day of its point, both empty when none.
type rateRead struct {
	text string
	day  contracts.Day
}

// readRate reads the newest DGS3MO point with a value in the 14 days up to day. A nil reader or no
// point gives the empty rate; a reader error is returned.
func readRate(ctx context.Context, r store.SeriesReader, day contracts.Day) (rateRead, error) {
	if r == nil {
		return rateRead{}, nil
	}
	from, err := daysBack(day, rateWindowDays)
	if err != nil {
		return rateRead{}, fmt.Errorf("option_candidates_page: bad day %q: %w", day, err)
	}
	pts, err := r.SeriesPoints(ctx, rateSeries, from, day)
	if err != nil {
		return rateRead{}, fmt.Errorf("option_candidates_page: read %s: %w", rateSeries, err)
	}
	var out rateRead
	for _, p := range pts {
		d := contracts.Day(p.ObservationDate.Format(time.DateOnly))
		if p.Value == nil || d < out.day {
			continue
		}
		out = rateRead{text: strconv.FormatFloat(*p.Value, 'f', -1, 64), day: d}
	}
	return out, nil
}

// putFigures is the figures of the ok put rows of one group, in row order.
func putFigures(figs optionfigapi.OptionFigures, rate rateRead, chainDay contracts.Day, rows []store.Candidate) []optionfigapi.PutFigures {
	if figs == nil {
		return nil
	}
	out := make([]optionfigapi.PutFigures, len(rows))
	for i, c := range rows {
		in := optionfigapi.PutFigureInput{
			ChainDay: chainDay, DTE: c.DTE, Strike: c.Strike, Spot: deref(c.Spot), IV: deref(c.Iv),
			AnnualYield: deref(c.AnnualizedYield), Rate: rate.text, RateDay: rate.day, MaxRateAgeDays: maxRateAgeDays,
		}
		out[i] = figs.Put(in)
	}
	return out
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// figureCell is a figure column that reads the result of its row by the row's position; with no
// results (a nil Figures) every cell is the gap no_reader.
func figureCell(figs []optionfigapi.PutFigures, f func(optionfigapi.PutFigures) any) func(int, store.Candidate) any {
	return func(i int, _ store.Candidate) any {
		if i < 0 || i >= len(figs) {
			return mkGap("no_reader", gapNoReader)
		}
		return f(figs[i])
	}
}

func figValueCell(text, key string) any {
	v, ok := numberLiteral(key)
	if !ok {
		return unreadableCell()
	}
	c := textCell(text)
	c.Value = &v
	return c
}

func itmFigure(f optionfigapi.PutFigures) any {
	if f.ItmGap != "" {
		return figGap(f.ItmGap, f.ItmWhy)
	}
	return figValueCell(f.ItmPercent, f.ItmKey)
}

func excessFigure(f optionfigapi.PutFigures) any {
	if f.ExcessGap != "" {
		return figGap(f.ExcessGap, f.ExcessWhy)
	}
	word := wordBelow
	if f.ExcessAbove {
		word = wordAbove
	}
	return figValueCell(word+fitSeparator+f.ExcessPercent, f.ExcessKey)
}

func figGap(reason, why string) any {
	switch reason {
	case "missing_input", "stale_input":
		return mkGap(reason, why)
	}
	return unreadableCell()
}

// earningsGap is the gap of a row with no stored earnings date (Decision 7).
func earningsGap(state string) gap {
	switch state {
	case "none", "absent", "listed":
		return mkGap("no_data", reasonText("no_earnings"))
	case "unsupported":
		return mkGap("no_data", reasonText("earnings_unsupported"))
	case "error":
		return mkGap("missing_input", reasonText("earnings_error"))
	}
	return unreadableCell()
}

// earningsDateCell shows the stored day as mm-dd with the full day, digits only, as the sort value.
func earningsDateCell(d contracts.Day) any {
	s := string(d)
	v, ok := numberLiteral(strings.ReplaceAll(s, "-", ""))
	if len(s) != len(dayLayout) || !ok {
		return unreadableCell()
	}
	c := textCell(s[5:])
	c.Value = &v
	return c
}
