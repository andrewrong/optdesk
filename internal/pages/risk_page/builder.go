package risk_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	timeLayout     = "2006-01-02T15:04:05-07:00"
	staleAfterDays = 4
	currency       = "USD" // the one currency of the risk figures
)

var (
	scopeKey      = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	numberPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
	decimalParts  = regexp.MustCompile(`^(-?)([0-9]+)(?:\.([0-9]*))?$`)
)

// build writes the household_risk page of alias for day; at is generated_at. alias "" is the main account.
func build(ctx context.Context, c Config, alias string, day contracts.Day, at time.Time) ([]byte, error) {
	if c.Zone == nil {
		return nil, errors.New("risk_page: no zone")
	}
	if _, err := time.Parse(dayLayout, string(day)); err != nil {
		return nil, fmt.Errorf("risk_page: bad day %q: %w", day, err)
	}
	at = at.In(c.Zone)
	if alias == "" && len(c.Aliases) > 0 {
		alias = c.Aliases[0]
	}
	if !slices.Contains(c.Aliases, alias) {
		return json.Marshal(gapPage(c, alias, at, "no_data", textUnknown))
	}
	if c.Account == nil || c.Ledger == nil || c.Sessions == nil || c.Ledgers == nil || c.Structures == nil || c.Holdings == nil {
		return json.Marshal(gapPage(c, alias, at, "no_reader", textNoReader))
	}
	l, err := c.load(ctx, alias, day)
	if err != nil {
		return nil, err
	}
	if l.in.HeldDay == "" {
		return json.Marshal(gapPage(c, alias, at, "no_data", textNoCapture))
	}
	if lag := len(l.in.SessionDays) - 1 - slices.Index(l.in.SessionDays, l.in.HeldDay); lag > staleSessions {
		return json.Marshal(gapPage(c, alias, at, "stale_input", textStale))
	}
	res := c.Structures().Evaluate(l.in, compute.StructureParams{MarkMaxAgeSessions: markMaxAgeSessions,
		CaptureTarget: captureTarget, ExitDTE: exitDTE, CommissionDays: commissionDays})
	in, foreign, skipped := riskInput(l, res)
	risk := c.Holdings().Risk(in)
	p := assemble(c, alias, at, risk, foreign, len(res.Other) > 0)
	if skipped > 0 {
		p.Notes = append(p.Notes, textNoExpiry)
	}
	captured := l.at
	if captured.IsZero() {
		captured, err = time.ParseInLocation(dayLayout, string(l.in.HeldDay), c.Zone)
		if err != nil {
			return nil, fmt.Errorf("risk_page: bad capture day %q: %w", l.in.HeldDay, err)
		}
	}
	captured = captured.In(c.Zone)
	p.DataTimes = []sourceTime{{Source: "account_capture", DataTime: captured.Format(timeLayout)}}
	if !l.ledger.IsZero() {
		p.DataTimes = append(p.DataTimes, sourceTime{Source: "option_ledger", DataTime: l.ledger.In(c.Zone).Format(timeLayout)})
	}
	p.Stale = staleLabel{
		DataTime: captured.Format(timeLayout), StaleAfter: captured.AddDate(0, 0, staleAfterDays).Format(timeLayout),
		Text: textStaleLabel + string(l.in.HeldDay),
	}
	return json.Marshal(p)
}

// assemble writes the page from the figures; the three blocks without a source are the gap not_built.
func assemble(c Config, alias string, at time.Time, risk blockapi.RiskResult, foreign, other bool) pageJSON {
	p := base(c, alias, at)
	p.RunStatus = "partial"
	p.Title, p.Sentence, p.VegaTitle = pageTitle, pageSentence, vegaTitle
	p.Delta = deltaBlock(risk)
	p.Vega = vegaBlock(risk)
	p.BetaDelta = betaGap(gapJSON{Reason: "not_built", Text: textNotBuiltBeta})
	p.Sigma = sigmaGap(gapJSON{Reason: "not_built", Text: textNotBuiltSigma})
	p.StandardMetrics = metricsGap(gapJSON{Reason: "not_built", Text: textNotBuiltMetrics})
	if foreign {
		p.Gaps = append(p.Gaps, gapJSON{Reason: "unsupported_currency", Text: textForeign})
	}
	if other {
		p.Notes = append(p.Notes, textOtherKinds)
	}
	for _, b := range []struct {
		title string
		gap   *gapJSON
	}{{titleDelta, p.Delta.Gap}, {vegaTitle, p.Vega.Gap}, {titleBeta, p.BetaDelta.Gap}, {titleSigma, p.Sigma.Gap},
		{titleMetrics, &gapJSON{Reason: "not_built", Text: textNotBuiltMetrics}}} {
		if b.gap != nil {
			p.Gaps = append(p.Gaps, gapJSON{Reason: b.gap.Reason, Text: b.title + ": " + b.gap.Text})
		}
	}
	return p
}

// riskInput is the input of Risk: the stocks of the capture and the net greeks of each recognised structure,
// filed under its nearest expiry. A structure not in USD is left out and foreign is true; one with no leg
// expiry is left out and counted in skipped.
func riskInput(l loaded, res compute.StructureResult) (in blockapi.RiskIn, foreign bool, skipped int) {
	in.Currency = currency
	for _, s := range l.stocks {
		in.Stocks = append(in.Stocks, blockapi.HoldingIn{Underlying: s.symbol, SecType: "STK", Currency: s.currency,
			Quantity: s.quantity, Multiplier: "1"})
	}
	for _, s := range res.Structures {
		if s.Currency != currency {
			foreign = true
			continue
		}
		expiry := ""
		for _, leg := range s.Legs {
			if leg.Expiry != "" && (expiry == "" || leg.Expiry < expiry) {
				expiry = leg.Expiry
			}
		}
		if expiry == "" {
			skipped++
			continue
		}
		in.Greeks = append(in.Greeks, blockapi.GreekIn{Underlying: s.Underlying, Expiry: contracts.Day(expiry),
			Delta: s.Greeks.Delta.Value, Vega: s.Greeks.Vega.Value})
	}
	return in, foreign, skipped
}

func deltaBlock(r blockapi.RiskResult) deltaJSON {
	b := deltaJSON{Title: titleDelta, Meaning: meaningDelta, Heads: []string{headName, headDelta}, Rows: []deltaRow{}}
	for _, ln := range r.Delta {
		cell, ok := deltaCell(ln.Value)
		if !ok {
			continue
		}
		b.Rows = append(b.Rows, deltaRow{Name: ln.Key, Delta: cell})
	}
	if cell, ok := deltaCell(r.DeltaTotal); ok {
		b.Total = &cell
	} else {
		b.Gap = riskGap(r.DeltaTotal)
	}
	return b
}

func vegaBlock(r blockapi.RiskResult) vegaJSON {
	b := vegaJSON{Meaning: meaningVega, Heads: []string{headExpiry, headVega}, Rows: []vegaRow{}}
	for _, ln := range r.Vega {
		v, ok := vegaCell(ln.Value)
		if !ok {
			continue
		}
		b.Rows = append(b.Rows, vegaRow{ExpiryText: ln.Key, Vega: v})
	}
	if v, ok := vegaCell(r.VegaTotal); ok {
		b.Total = &v
	} else {
		b.Gap = riskGap(r.VegaTotal)
	}
	return b
}

// riskGap is the gap of a block whose total is not a number; a total with no reason of its own is missing_input.
func riskGap(total compute.FigureValue) *gapJSON {
	reason := total.Gap
	if reason == "" {
		reason = "missing_input"
	}
	return &gapJSON{Reason: reason, Text: gapText(reason)}
}

func gapText(reason string) string {
	switch reason {
	case "unsupported_currency":
		return textForeign
	case "no_data":
		return textNoCapture
	}
	return textDeltaGap
}

// deltaCell is a delta in dollars as a money cell, rounded half up to cents; ok is false for a gap or an
// unreadable number.
func deltaCell(v compute.FigureValue) (moneyCell, bool) {
	if v.Gap != "" {
		return moneyCell{}, false
	}
	cents, ok := roundCents(v.Value)
	if !ok {
		return moneyCell{}, false
	}
	text := cents
	if !strings.HasPrefix(cents, "-") && strings.Trim(cents, "0.") != "" {
		text = "+" + cents
	}
	return moneyCell{Value: json.Number(cents), Text: text + unitDelta, Unit: "usd_signed", Currency: currency}, true
}

func vegaCell(v compute.FigureValue) (vegaFigure, bool) {
	if v.Gap != "" || !numberPattern.MatchString(v.Value) {
		return vegaFigure{}, false
	}
	return vegaFigure{Value: json.Number(v.Value), Text: v.Value + unitVega, Unit: "usd_per_vol_point"}, true
}

// roundCents writes decimal text with two places, the third place rounding half away from zero.
func roundCents(text string) (string, bool) {
	m := decimalParts.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	sign, whole, frac := m[1], m[2], m[3]
	frac += "000"
	digits := []byte(whole + frac[:2])
	if frac[2] >= '5' {
		i := len(digits) - 1
		for ; i >= 0 && digits[i] == '9'; i-- {
			digits[i] = '0'
		}
		if i < 0 {
			digits = append([]byte{'1'}, digits...)
		} else {
			digits[i]++
		}
	}
	n := len(digits) - 2
	out := strings.TrimLeft(string(digits[:n]), "0")
	if out == "" {
		out = "0"
	}
	out += "." + string(digits[n:])
	if sign == "-" && strings.Trim(out, "0.") != "" {
		out = "-" + out
	}
	return out, true
}

// base is the envelope with every block empty.
func base(c Config, alias string, at time.Time) pageJSON {
	scope := alias
	if !scopeKey.MatchString(scope) {
		scope = "unknown"
	}
	return pageJSON{
		Schema: "household_risk", SchemaVersion: 1, RunID: c.RunID, BuildID: c.BuildID, Scope: scope,
		GeneratedAt: at.Format(timeLayout), DataTimes: []sourceTime{}, Notes: []string{}, Gaps: []gapJSON{},
		Stale: staleLabel{DataTime: at.Format(timeLayout), StaleAfter: at.Format(timeLayout), Text: textStaleLabel + "none"},
	}
}

// gapPage is the page of an alias that has nothing to show: every block is the one gap.
func gapPage(c Config, alias string, at time.Time, reason, text string) pageJSON {
	p := base(c, alias, at)
	g := gapJSON{Reason: reason, Text: text}
	p.RunStatus = "failed"
	p.Title, p.Sentence, p.VegaTitle = pageTitle, text, vegaTitle
	p.Delta = deltaJSON{Title: titleDelta, Meaning: meaningDelta, Heads: []string{headName, headDelta}, Rows: []deltaRow{}, Gap: &g}
	p.Vega = vegaJSON{Meaning: meaningVega, Heads: []string{headExpiry, headVega}, Rows: []vegaRow{}, Gap: &g}
	p.BetaDelta, p.Sigma, p.StandardMetrics = betaGap(g), sigmaGap(g), metricsGap(g)
	p.Gaps = []gapJSON{g}
	return p
}
