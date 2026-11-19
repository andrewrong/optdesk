package main

import (
	"context"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionevidence"
	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/compute/optionfig"
	"github.com/andrewrong/optdesk/internal/compute/optionshortlistapi"
	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/option_candidates_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/optionday"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
	"github.com/andrewrong/optdesk/internal/store/valuation"
	"github.com/andrewrong/optdesk/internal/strategy/risk/bsm"
)

// candidateSource is the Source of the quote of a stored candidate in obs.option_day: the candidates are made
// from the end-of-day capture, so the at-the-money row must come from the same capture (the other value of the
// store is "option_open"). It is the Source argument of the ATM picker.
const candidateSource = "option_eod"

// termsChunk is the most instrument ids one OptionTerms call accepts.
const termsChunk = 500

// candidateReaders are the readers option_candidates_page gets: the candidate and name readers, the readings
// (View), the option terms (Terms) and the option day reader (Days). The last three are nil on the named path
// (ot page) and in the demo profile, where the evidence cells are gaps.
type candidateReaders struct {
	Cands store.CandidateReader
	Names store.InstrumentNameReader
	View  store.ReadingViewReader
	Terms store.OptionTermsReader
	Days  optionday.DayReader
	// The five fields of the tier view; all nil on the named path (ot page) and in the demo profile, where
	// short_put_tiers is null.
	Tiers      optiontierapi.Tierer
	Account    store.AccountReader
	Aliases    []string
	Owner      ownerset.Reader
	RuleParams settingsapi.RuleParams
	// Valuation is the valuation reader of the site, for the valuation line of the company strips; nil on the
	// named path (ot page), in the demo profile and on a site with no pages line.
	Valuation valuation.Reader
	// Shortlist is the short-put shortlist rule; nil on the named path (ot page), where short_put_list is omitted.
	Shortlist optionshortlistapi.Shortlister
}

// bsmValuer adapts the Black-Scholes pricer to optionevidenceapi.Valuer, so cmd/ot is the place that joins the
// two modules.
type bsmValuer struct{ p bsm.Pricer }

var _ optionevidenceapi.Valuer = bsmValuer{}

// newBSMValuer returns the adapter as the interface, so the argument of optionevidence.New has the static type
// optionevidenceapi.Valuer.
func newBSMValuer() optionevidenceapi.Valuer { return bsmValuer{p: bsm.New()} }

// Value delegates to the held pricer.
func (b bsmValuer) Value(right, spot, strike, years, rate, vol string) (string, error) {
	return b.p.Value(right, spot, strike, years, rate, vol)
}

// newOptionCandidatesPage builds the page option_candidates_page in zone from the plain arguments; the one
// place that names option_candidates_page.Config.
func newOptionCandidatesPage(zone *time.Location, runID, buildID, scope string, r candidateReaders) pages.Page {
	return option_candidates_page.NewPage(candidatesConfig(zone, runID, buildID, scope, r))
}

// candidatesConfig is the Config newOptionCandidatesPage builds.
func candidatesConfig(zone *time.Location, runID, buildID, scope string, r candidateReaders) option_candidates_page.Config {
	return option_candidates_page.Config{
		RunID: runID, BuildID: buildID, Scope: scope, Zone: zone, Candidates: r.Cands, Names: r.Names,
		Figures:  optionfig.New(),
		Evidence: newEvaluator(),
		ATMFor:   newATMFor(r.Days, r.Terms, optionevidence.NewATMPicker()),
		Readings: r.View,
		Tiers:    r.Tiers, Account: r.Account, Aliases: r.Aliases, Owner: r.Owner, RuleParams: r.RuleParams,
		Valuation: r.Valuation, ValueLines: newValueLines(), Buy: newBuyPriceResolver(),
		Shortlist: r.Shortlist,
	}
}

type chainKey struct {
	underlying int
	day        contracts.Day
}

// newATMFor returns the ATMFunc that joins the chain of an underlying on a day (ChainDay) with the option terms
// (OptionTerms, at most 500 ids per call) into ChainQuote values and lets the picker choose. The joined chain is
// kept for the life of the closure. A nil days, terms or picker gives nil.
func newATMFor(days optionday.DayReader, terms store.OptionTermsReader, picker optionevidenceapi.ATMPicker) option_candidates_page.ATMFunc {
	if days == nil || terms == nil || picker == nil {
		return nil
	}
	var mu sync.Mutex
	chains := map[chainKey][]optionevidenceapi.ChainQuote{}
	return func(ctx context.Context, underlyingID int, day, expiry contracts.Day, spot string) (string, string, error) {
		mu.Lock()
		defer mu.Unlock()
		key := chainKey{underlyingID, day}
		chain, ok := chains[key]
		if !ok {
			var err error
			if chain, err = joinChain(ctx, days, terms, int64(underlyingID), day); err != nil {
				return "", "", err
			}
			chains[key] = chain
		}
		iv, strike := picker.ATM(chain, expiry, spot, candidateSource)
		return iv, strike, nil
	}
}

// joinChain copies the chain rows that have current terms into ChainQuote values.
func joinChain(ctx context.Context, days optionday.DayReader, terms store.OptionTermsReader,
	underlying int64, day contracts.Day) ([]optionevidenceapi.ChainQuote, error) {
	rows, err := days.ChainDay(ctx, underlying, day)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.InstrumentID
	}
	byID := make(map[int64]store.OptionTerms, len(ids))
	for from := 0; from < len(ids); from += termsChunk {
		to := min(from+termsChunk, len(ids))
		ts, err := terms.OptionTerms(ctx, ids[from:to])
		if err != nil {
			return nil, err
		}
		for _, t := range ts {
			byID[t.InstrumentID] = t
		}
	}
	chain := make([]optionevidenceapi.ChainQuote, 0, len(rows))
	for _, r := range rows {
		t, ok := byID[r.InstrumentID]
		if !ok {
			continue
		}
		iv := ""
		if r.Iv != nil {
			iv = *r.Iv
		}
		chain = append(chain, optionevidenceapi.ChainQuote{
			Expiry: t.ExpirationDate, Right: t.Right, Strike: t.Strike, IV: iv, Source: r.Source,
		})
	}
	return chain, nil
}
