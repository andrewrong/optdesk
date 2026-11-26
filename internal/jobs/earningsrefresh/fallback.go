package earningsrefresh

import (
	"context"
	"errors"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
)

// fallbackSource is the row source the fallback writes (docs/design/earnings-fallback.md Decision 2).
const fallbackSource = "yfinance"

// FallbackMaker builds the fallback source of one market for the symbols of the entries the primary left without
// a date.
type FallbackMaker func(market string, symbols []string) (sources.EarningsSource, error)

// WithFallback returns j with a fallback: after the primary fetch, the entries without a date are asked of the
// source mk builds, once per market (Decisions 1, 2 and 4). A job that is not from this package, or a nil mk,
// is returned unchanged.
func WithFallback(j contracts.Job, mk FallbackMaker) contracts.Job {
	jb, ok := j.(*job)
	if !ok || mk == nil {
		return j
	}
	jb.fallback = mk
	return jb
}

// applyFallback turns the rows with no date (not listed, not owner) into listed yfinance rows where the fallback
// has a listing on or after day. Its failures are logged once per market and change nothing else.
func (j *job) applyFallback(ctx context.Context, day contracts.Day, rows []contracts.EarningsDate) {
	if j.fallback == nil {
		return
	}
	byIssuer := map[string]jobs.WatchEntry{}
	for _, e := range j.entries {
		byIssuer[e.Issuer] = e
	}
	for _, market := range []string{"US", "JP"} {
		var idx []int
		var symbols []string
		for i, r := range rows {
			if string(r.Market) == market && r.Status != "listed" && r.Source != "owner" {
				idx = append(idx, i)
				symbols = append(symbols, byIssuer[r.Issuer].Contract.Symbol)
			}
		}
		if len(idx) == 0 {
			continue
		}
		src, err := j.fallback(market, symbols)
		var listings []sources.EarningsListing
		if err == nil {
			listings, err = src.Fetch(ctx)
		}
		if err != nil {
			code := "unknown"
			var ce sources.CodedError
			if errors.As(err, &ce) && ce.ErrorCode() != "" {
				code = ce.ErrorCode()
			}
			j.log.Warn("fallback failed", "market", market, "code", code)
			continue
		}
		for n, i := range idx {
			if d := earliestAsked(listings, symbols[n], string(day)); d != "" {
				if pd, err := date(d); err == nil {
					rows[i].Status, rows[i].Source, rows[i].ReportDate = "listed", fallbackSource, pd
					rows[i].ErrorCode, rows[i].TimeOfDay = "", ""
					rows[i].FiscalPeriodEnd, rows[i].Estimate = nil, nil
				}
			}
		}
	}
}

// earliestAsked returns the earliest report date on or after day of the listings of the symbol asked ("" for none).
func earliestAsked(listings []sources.EarningsListing, asked, day string) string {
	best := ""
	for _, l := range listings {
		d := string(l.ReportDate)
		if d == "" || d < day || !sameSymbol(l.Symbol, asked) {
			continue
		}
		if best == "" || d < best {
			best = d
		}
	}
	return best
}
