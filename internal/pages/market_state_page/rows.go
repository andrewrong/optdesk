package market_state_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Stored readings of the market rows (kind market).
const (
	figSp500Vs200d = "market.sp500_vs_200d"
	figVixPctile   = "market.vix_pctile_3y"
	figVixTerm     = "market.vix_term_ratio"
	kindMarket     = "market"
)

// Reasons of the Gap of common.v2.json.
const (
	reasonNoData       = "no_data"
	reasonNoReader     = "no_reader"
	reasonMissingInput = "missing_input"
	reasonStaleInput   = "stale_input"
)

const (
	stateOk         = "ok"
	codeStaleInput  = 3
	unitPercent     = "percent"
	unitIndexPoints = "index_points"
	unitRatio       = "ratio"
	unitPctPoints   = "percentage_points"
)

func mkGap(reason, text string) gap { return gap{Kind: "gap", Reason: reason, Text: text} }

// reasonOf maps a compute reason to the reason of a Gap (one table).
func reasonOf(r compute.Reason) string {
	switch r { //nolint:exhaustive // every other reason is missing_input
	case compute.NoInput:
		return reasonNoData
	case compute.StaleInput:
		return reasonStaleInput
	}
	return reasonMissingInput
}

// reasonText is the text of a Gap of a figure called what.
func reasonText(reason, what string) string {
	switch reason {
	case reasonNoReader:
		return what + ": " + gapNoReader
	case reasonNoData:
		return what + ": " + gapNoData
	case reasonStaleInput:
		return what + ": " + gapStale
	}
	return what + ": " + gapMissing
}

func numGap(r compute.Reason, what string) gap {
	reason := reasonOf(r)
	return mkGap(reason, reasonText(reason, what))
}

var decimalText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

var errNotDecimal = errors.New("market_state_page: not a decimal text")

// shiftRound shifts the decimal text by shift places to the right, rounds it half to even to
// decimals places and writes it with exactly decimals places; the arithmetic is exact (math/big)
// and a result of zero has no sign.
func shiftRound(text string, shift, decimals int) (string, error) {
	if !decimalText.MatchString(text) {
		return "", errNotDecimal
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok {
		return "", errNotDecimal
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(shift+decimals)), nil)
	r.Mul(r, new(big.Rat).SetInt(scale))
	neg := r.Sign() < 0
	r.Abs(r)
	q, rem := new(big.Int).QuoRem(r.Num(), r.Denom(), new(big.Int))
	switch rem.Lsh(rem, 1).Cmp(r.Denom()) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	digits := q.String()
	if decimals > 0 {
		if len(digits) <= decimals {
			digits = strings.Repeat("0", decimals-len(digits)+1) + digits
		}
		digits = digits[:len(digits)-decimals] + "." + digits[len(digits)-decimals:]
	}
	if neg && q.Sign() != 0 {
		digits = "-" + digits
	}
	return digits, nil
}

// signed puts a plus in front of a positive decimal text.
func signed(text string) string {
	if strings.HasPrefix(text, "-") || strings.Trim(text, "0.") == "" {
		return text
	}
	return "+" + text
}

// mkFigure is a figure of exact decimal text, or a missing_input Gap when the text is no number.
func mkFigure(value, text, unit, what string) any {
	if !decimalText.MatchString(value) {
		return mkGap(reasonMissingInput, reasonText(reasonMissingInput, what))
	}
	return figure{Kind: "value", Value: json.Number(value), Text: text, Unit: unit}
}

// dayText writes an ISO day as "Nov 13".
func dayText(day string) string {
	if t, err := time.Parse(dayLayout, day); err == nil {
		return t.Format("Jan 2")
	}
	return day
}

// ordinal writes a whole percent as "38th".
func ordinal(n string) string {
	suffix := "th"
	if len(n) >= 2 && n[len(n)-2] == '1' {
		return n + suffix
	}
	switch n[len(n)-1] {
	case '1':
		suffix = "st"
	case '2':
		suffix = "nd"
	case '3':
		suffix = "rd"
	}
	return n + suffix
}

// reading is the newest stored row of one figure; noReader is set when there is no reader.
type reading struct {
	noReader bool
	rec      *contracts.ReadingRecord
	dataDay  string // ISO day of the parameter data_day, empty when absent
}

type readings struct{ sp, pctile, term reading }

// readReadings asks for the six days before day and takes the newest row of each reading.
func readReadings(ctx context.Context, c Config, d time.Time) (readings, error) {
	if c.Readings == nil {
		nr := reading{noReader: true}
		return readings{nr, nr, nr}, nil
	}
	var out readings
	var err error
	if out.sp, err = readNewest(ctx, c, figSp500Vs200d, seriesSP500, d); err != nil {
		return out, err
	}
	if out.pctile, err = readNewest(ctx, c, figVixPctile, seriesVIX, d); err != nil {
		return out, err
	}
	out.term, err = readNewest(ctx, c, figVixTerm, seriesVIX, d)
	return out, err
}

// readNewest asks for the six days day-5 to day and takes the newest row.
func readNewest(ctx context.Context, c Config, name, subject string, d time.Time) (reading, error) {
	from := contracts.Day(d.AddDate(0, 0, -5).Format(dayLayout))
	rows, err := c.Readings(ctx, name, kindMarket, subject, from, contracts.Day(d.Format(dayLayout)))
	if err != nil {
		return reading{}, fmt.Errorf("market_state_page: read %s: %w", name, err)
	}
	var best *contracts.ReadingRecord
	for i := range rows {
		r := &rows[i]
		if best == nil || r.Day.After(best.Day.Time) || (r.Day.Equal(best.Day.Time) && r.Rev > best.Rev) {
			best = r
		}
	}
	out := reading{rec: best}
	if best == nil {
		return out, nil
	}
	for _, p := range best.Parameters {
		if p.Param != "data_day" || p.ValueTag == nil {
			continue
		}
		if t, err := time.Parse(dayLayout, *p.ValueTag); err == nil {
			out.dataDay = t.Format(dayLayout)
		}
	}
	return out, nil
}

// usable returns the value text of an ok row that has one.
func (r reading) usable() (string, bool) {
	if r.rec == nil || string(r.rec.State) != stateOk || r.rec.Value == nil {
		return "", false
	}
	return *r.rec.Value, true
}

// gapOf is the Gap of a reading that has no figure.
func (r reading) gapOf(what string) gap {
	switch {
	case r.noReader:
		return mkGap(reasonNoReader, reasonText(reasonNoReader, what))
	case r.rec == nil:
		return mkGap(reasonNoData, reasonText(reasonNoData, what))
	case r.rec.ReasonCode != nil && *r.rec.ReasonCode == codeStaleInput:
		return mkGap(reasonStaleInput, reasonText(reasonStaleInput, what))
	}
	return mkGap(reasonMissingInput, reasonText(reasonMissingInput, what))
}

// dayOrGap is the day cell: the day, or the Gap when the figure is a gap or has no day.
func dayOrGap(day string, fig any, what string) any {
	if g, isGap := fig.(gap); isGap {
		return g
	}
	if day == "" {
		return mkGap(reasonMissingInput, what+": "+gapNoDay)
	}
	return day
}

// buildRows makes the five rows in fixed order and the newest data day of each source.
func buildRows(c Config, rd readings, ms marketstateapi.Result, lr liquidityapi.Result) ([]marketRow, dataDays) {
	days := dataDays{}
	hist := func(l marketstateapi.Line, label string, f pointFormat) any {
		if c.Market == nil {
			return mkGap(reasonNoReader, reasonText(reasonNoReader, label+" history"))
		}
		return historyCell(l, label, f)
	}
	sp := sp500Row(rd.sp)
	sp.History = hist(ms.History.Sp500Vs200d, labelSp500Vs200d, fmtPercentShift)
	dd := drawdownRow(c.Market != nil, ms.Drawdown)
	dd.History = hist(ms.History.Drawdown, labelDrawdown, fmtPercent)
	vx := vixRow(c.Market != nil, ms.History.VIX, rd.pctile)
	vx.History = hist(ms.History.VIX, labelVix, fmtOneDecimal)
	tm := termRow(rd.term)
	tm.History = hist(ms.History.VIXTerm, labelVixTerm, fmtTwoDecimals)
	hy := hySpreadRow(c.Liquidity != nil, lr)
	hy.History = hist(ms.History.HighYield, labelHySpread, fmtPercent)
	rows := []marketRow{sp, dd, vx, tm, hy}
	for _, r := range rows {
		if day, ok := r.Day.(string); ok {
			src := map[string]string{"sp500_vs_200d": seriesSP500, "sp500_drawdown": seriesSP500,
				"vix": seriesVIX, "vix_term": seriesVIX, "hy_spread": seriesHY}[r.ID]
			days.add(src, day)
		}
	}
	return rows, days
}

func sp500Row(r reading) marketRow {
	row := marketRow{ID: "sp500_vs_200d", Label: labelSp500Vs200d, Window: windowSp500Vs200d}
	v, ok := r.usable()
	pct, err := shiftRound(v, 2, 2)
	if !ok || err != nil {
		row.Figure = r.gapOf(labelSp500Vs200d)
		if ok {
			row.Figure = mkGap(reasonMissingInput, reasonText(reasonMissingInput, labelSp500Vs200d))
		}
	} else {
		text := "S&P 500 equals its 200-day average"
		switch {
		case strings.HasPrefix(pct, "-"):
			text = "S&P 500 is " + pct[1:] + "% below its 200-day average"
		case pct != "0.00":
			text = "S&P 500 is " + pct + "% above its 200-day average"
		}
		row.Figure = mkFigure(pct, text, unitPercent, labelSp500Vs200d)
	}
	row.Day = dayOrGap(r.dataDay, row.Figure, labelSp500Vs200d)
	return row
}

func drawdownRow(haveMarket bool, n liquidityapi.Num) marketRow {
	row := marketRow{ID: "sp500_drawdown", Label: labelDrawdown, Window: windowDrawdown}
	switch {
	case !haveMarket:
		row.Figure = mkGap(reasonNoReader, reasonText(reasonNoReader, labelDrawdown))
	case n.Reason != "":
		row.Figure = numGap(n.Reason, labelDrawdown)
	default:
		pct, err := shiftRound(n.Value, 0, 2)
		if err != nil {
			row.Figure = mkGap(reasonMissingInput, reasonText(reasonMissingInput, labelDrawdown))
			break
		}
		text := "S&P 500 is at its 60-session high of " + dayText(string(n.Aux))
		if strings.HasPrefix(pct, "-") {
			text = "S&P 500 is " + pct[1:] + "% below its 60-session high of " + dayText(string(n.Aux))
		}
		row.Figure = mkFigure(pct, text, unitPercent, labelDrawdown)
	}
	row.Day = dayOrGap(string(n.Day), row.Figure, labelDrawdown)
	return row
}

// vixRow takes the level from the newest point of the VIX history and the percentile from the
// stored reading.
func vixRow(haveMarket bool, l marketstateapi.Line, pctile reading) marketRow {
	row := marketRow{ID: "vix", Label: labelVix, Window: windowVix}
	day := ""
	switch {
	case !haveMarket:
		row.Figure = mkGap(reasonNoReader, reasonText(reasonNoReader, labelVix))
	case len(l.Points) == 0:
		row.Figure = mkGap(reasonNoData, reasonText(reasonNoData, labelVix))
	default:
		last := l.Points[len(l.Points)-1]
		day = string(last.Day)
		level, err := shiftRound(last.Value, 0, 1)
		if err != nil {
			row.Figure = mkGap(reasonMissingInput, reasonText(reasonMissingInput, labelVix))
			break
		}
		text := "VIX " + level + " on " + dayText(day)
		pt, ptText := vixPctText(pctile)
		if ptText {
			text += ", at the " + pt + " of the last 3 years"
		}
		row.Figure = mkFigure(level, text, unitIndexPoints, labelVix)
	}
	pt, _ := vixPctText(pctile)
	row.Percentile = &percentile{Text: pt, Window: pctWindowVix}
	row.Day = dayOrGap(day, row.Figure, labelVix)
	return row
}

// vixPctText is the percentile text of the stored reading and whether it is a number.
func vixPctText(r reading) (string, bool) {
	v, ok := r.usable()
	if !ok {
		return gapNoPct + r.gapOf("reading").Text, false
	}
	if x, ok := new(big.Rat).SetString(v); !ok || !decimalText.MatchString(v) || x.Sign() < 0 || x.Cmp(big.NewRat(1, 1)) > 0 {
		return gapNoPct + gapMissing, false
	}
	n, err := shiftRound(v, 2, 0)
	if err != nil {
		return gapNoPct + gapMissing, false
	}
	return ordinal(n) + " percentile", true
}

func termRow(r reading) marketRow {
	row := marketRow{ID: "vix_term", Label: labelVixTerm, Window: windowVixTerm}
	v, ok := r.usable()
	text, err := shiftRound(v, 0, 2)
	if !ok || err != nil {
		row.Figure = r.gapOf(labelVixTerm)
		if ok {
			row.Figure = mkGap(reasonMissingInput, reasonText(reasonMissingInput, labelVixTerm))
		}
	} else {
		row.Figure = mkFigure(text, "VIX is "+text+" of VIX3M on "+dayText(r.dataDay), unitRatio, labelVixTerm)
	}
	row.Day = dayOrGap(r.dataDay, row.Figure, labelVixTerm)
	return row
}

func hySpreadRow(haveLiquidity bool, lr liquidityapi.Result) marketRow {
	row := marketRow{ID: "hy_spread", Label: labelHySpread, Window: windowHySpread}
	n := lr.HighYield
	rank := lr.HighYieldRank
	pctText, havePct := gapNoPct+gapMissing, false
	switch {
	case !haveLiquidity:
		pctText = gapNoPct + gapNoReader
	case rank.Reason == compute.ShortHistory:
		pctText = gapNoPct + "short stored history, " + fmt.Sprint(rank.Days) + " days"
	case rank.Reason != "":
		pctText = gapNoPct + numGap(rank.Reason, "rank").Text
	default:
		if p, err := shiftRound(rank.Value, 0, 0); err == nil {
			pctText, havePct = ordinal(p)+" percentile of our stored history since "+string(rank.First)+
				" ("+fmt.Sprint(rank.Days)+" days)", true
		}
	}
	switch {
	case !haveLiquidity:
		row.Figure = mkGap(reasonNoReader, reasonText(reasonNoReader, labelHySpread))
	case n.Reason != "":
		row.Figure = numGap(n.Reason, labelHySpread)
	default:
		level, err := shiftRound(n.Value, 0, 2)
		if err != nil {
			row.Figure = mkGap(reasonMissingInput, reasonText(reasonMissingInput, labelHySpread))
			break
		}
		text := "High-yield spread " + level + "% on " + dayText(string(n.Day))
		if havePct {
			text += ", at the " + pctText
		}
		row.Figure = mkFigure(level, text, unitPercent, labelHySpread)
	}
	row.Percentile = &percentile{Text: pctText, Window: pctWindowHy}
	row.Day = dayOrGap(string(n.Day), row.Figure, labelHySpread)
	return row
}
