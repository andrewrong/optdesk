package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// siteCandidatesRoute is the address of the page option_candidates_page under the site root. The ninth item
// of the bar that points to it belongs to web (docs/design/option-candidates-page.md, Decision 2).
const siteCandidatesRoute = "options/candidates"

// candidatesName is the name of the page option_candidates_page, which pageNamed selects for ot page.
const candidatesName = "option_candidates_page"

func init() {
	registerPage(pageFamily{name: candidatesName, route: siteCandidatesRoute, order: 10,
		build: buildCandidates, named: candidatesPageNamed})
}

// candidateStore is the candidate store as the page reads it: the reader and the close of the connections.
type candidateStore interface {
	store.CandidateReader
	Close() error
}

// openCandidateFunc opens the candidate store with a DSN (internal/store/candidate.Open).
type openCandidateFunc func(ctx context.Context, dsn string) (candidateStore, error)

// buildCandidates builds option_candidates_page per request for the day of dayFlag or today, from the
// candidate reader of the site (rd.cands, opened once per process); the demo profile reads fixed demo rows
// and opens nothing. The lookback over earlier days is the builder's. A nil zone is an error.
func buildCandidates(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New("option_candidates_page: no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	return newCandidatesPage(a, "ot-serve", rd.cands).Build(ctx, nil, day, clk.Now().UTC())
}

// newCandidatesPage calls deps.optionCandidatesPage with the plain arguments (scope "candidates").
func newCandidatesPage(a *app, runID string, cands store.CandidateReader) pages.Page {
	return a.deps.optionCandidatesPage(a.profile.Zone, runID, buildIDOf(version), "candidates", cands)
}

// candidatesPageNamed returns option_candidates_page for ot page and a close function: the candidate store
// opened once with the pages line of .env, nil with a missing line (the page shows its gap no_reader), or the
// demo rows with the demo profile (no store). A nil zone is an error raised before any store is opened.
func candidatesPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New(candidatesName + " needs the owner's zone: " + zoneEnvName + " is not set")
	}
	if a.profile.Name == "demo" {
		return newCandidatesPage(a, "ot-page", demoCandidates{}), func() {}, nil
	}
	env, err := a.envFile()
	if err != nil {
		return pages.Page{}, nil, err
	}
	dsn := env[pagesDSNName]
	if dsn == "" {
		return newCandidatesPage(a, "ot-page", nil), func() {}, nil
	}
	s, err := a.deps.openCandidate(ctx, dsn)
	if err != nil {
		return pages.Page{}, nil, fmt.Errorf("open candidate store with %s failed", pagesDSNName)
	}
	return newCandidatesPage(a, "ot-page", s), func() { _ = s.Close() }, nil
}

// demoCandidates is the candidate reader of the demo profile: for every day it holds two ok rows and one
// blocked row of each of the three kinds, fixed text, ranked as stored.
type demoCandidates struct{}

// Candidates returns the demo rows of kind written on day; an unknown kind or a zero day is an error.
func (demoCandidates) Candidates(_ context.Context, day contracts.Day, kind string) ([]store.Candidate, error) {
	if day == "" {
		return nil, errors.New("demo candidates: no day")
	}
	right := "call"
	switch kind {
	case "short_put":
		right = "put"
	case "covered_call", "held_call_guard":
	default:
		return nil, fmt.Errorf("demo candidates: unknown kind %q", kind)
	}
	at := time.Date(2026, 11, 6, 21, 30, 0, 0, time.UTC)
	mk := func(id int, status, strike, yield string, rank *int, gates string) store.Candidate {
		y := yield
		d, iv := "-0.25", "0.31"
		return store.Candidate{
			SessionDay: day, RunID: "demo-run", Kind: kind, InstrumentID: id, RuleVersion: "demo",
			Issuer: "DEMO", UnderlyingID: 1, Expiry: "2026-12-18", DTE: 39, Right: right, Strike: strike,
			Multiplier: 100, BidEod: "1.20", BreakEven: "93.80", AnnualizedYield: &y, Delta: &d, Iv: &iv,
			Status: status, Gates: gates, Marks: "[]", Fits: "{}", EarningsState: "listed",
			Rank: rank, ComputedAt: at,
		}
	}
	one, two := 1, 2
	return []store.Candidate{
		mk(101, "ok", "95.00", "0.123456", &one, "[]"),
		mk(102, "ok", "90.00", "0.081200", &two, "[]"),
		mk(103, "blocked", "85.00", "0.040000", nil,
			`[{"gate":"events","status":"block","reason":"earnings_before_expiry"}]`),
	}, nil
}
