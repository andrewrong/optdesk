package companies_page

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	dayLayout = "2006-01-02"

	bandName     = "valuation.buy_band"
	approvalName = "valuation.band_approval"

	// approvalValidDays is how long an approval holds, counted from its day.
	approvalValidDays = 90
	// approvalLookbackDays is how far before today the approval is looked for.
	approvalLookbackDays = 400

	// The price_state values of page.companies_page.v2.
	stateBand     = "band"
	stateOverride = "owner override"
	stateAwaiting = "awaiting approval"
	stateExpired  = "expired"
)

// bandReady reports whether every reader the band needs is set.
func (c Config) bandReady() bool {
	return c.Read != nil && c.Facts != nil && c.StandardOf != nil && c.BuildFilings != nil && c.Band != nil
}

// priceCell builds the price line cell of one subject. The override wins; then the band as the
// compute functions give it, qualified by the owner's approval. The cell holds no formula: every
// figure in it is text the compute functions or the owner wrote.
func (c Config) priceCell(ctx context.Context, s Subject, today contracts.Day) (out cell, unvalued bool, err error) {
	out = cell{HandPrice: s.HandPrice}
	if c.Override != nil {
		line, ok, oerr := c.Override.Line(ctx, s.Symbol)
		if oerr != nil {
			return cell{}, false, fmt.Errorf("companies_page: override of %s: %w", s.Symbol, oerr)
		}
		if ok && line != "" {
			out.Text, out.PriceState = line, stateOverride
			return out, false, nil
		}
	}
	if !c.bandReady() {
		out.Text = noReaderText
		return out, false, nil
	}
	source, ok := factSource(s.Issuer)
	if !ok {
		out.Text = unvaluedText("no issuer id")
		return out, true, nil
	}
	facts, ferr := c.Facts.FactsFiledBy(ctx, source, s.Issuer, today)
	if ferr != nil {
		return cell{}, false, fmt.Errorf("companies_page: facts of %s: %w", s.Symbol, ferr)
	}
	fr := c.BuildFilings(facts, today, c.StandardOf(facts))
	if fr.State != valuationapi.StateOK {
		out.Text = unvaluedText(fr.Reasons...)
		return out, true, nil
	}
	band := c.Band(bandName, fr.Filings, today)
	if band.State != valuationapi.StateOK {
		out.Text = unvaluedText(band.Reasons...)
		return out, true, nil
	}
	state, aerr := c.approvalState(ctx, s.Symbol, band.Value, today)
	if aerr != nil {
		return cell{}, false, aerr
	}
	out.PriceState = state
	figure := band.Low + " to " + band.Value
	switch state {
	case stateBand:
		out.Text = figure
	case stateAwaiting:
		// The owner sees the figure he is asked to approve.
		out.Text = stateAwaiting + ": " + figure
	default:
		out.Text = state
	}
	return out, false, nil
}

// approvalState is the price state of a band whose 8% value is value: the band while the latest
// approval is for that value and today is within 90 days of its day, expired after that,
// awaiting approval without such an approval.
func (c Config) approvalState(ctx context.Context, symbol, value string, today contracts.Day) (string, error) {
	t, err := time.Parse(dayLayout, string(today))
	if err != nil {
		return "", fmt.Errorf("companies_page: bad day %q: %w", today, err)
	}
	from := contracts.Day(t.AddDate(0, 0, -approvalLookbackDays).Format(dayLayout))
	rows, err := c.Read.WinnerReadings(ctx, approvalName, "underlying", symbol, from, today)
	if err != nil {
		return "", fmt.Errorf("companies_page: approval of %s: %w", symbol, err)
	}
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		if r.State != "ok" || r.Value == nil {
			continue
		}
		if *r.Value != value {
			return stateAwaiting, nil
		}
		if t.After(r.Day.AddDate(0, 0, approvalValidDays)) {
			return stateExpired, nil
		}
		return stateBand, nil
	}
	return stateAwaiting, nil
}

// factSource names the fact source of an issuer text: cik:<digits> is read from sec, edinet:<code>
// from edinet.
func factSource(issuer string) (string, bool) {
	switch {
	case strings.HasPrefix(issuer, "cik:"):
		return "sec", true
	case strings.HasPrefix(issuer, "edinet:"):
		return "edinet", true
	}
	return "", false
}

func unvaluedText(reasons ...string) string {
	if len(reasons) == 0 {
		return "unvalued"
	}
	return "unvalued: " + strings.Join(reasons, ", ")
}
