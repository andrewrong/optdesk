package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/optionopen"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/option_boards"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// Design: docs/design/option-page-builders.md#5-option-page-route.

// siteOptionRoute is the address of the page option_page under the site root.
const siteOptionRoute = "options"

// optionPageName is the name of the page option_page, which pageNamed selects for ot page.
const optionPageName = "option_page"

// optionPageScope is the scope the page carries (docs/design/option-page-builders.md, Decision 9).
const optionPageScope = "options"

// newOptionPage builds the page option_page; a test replaces it. The page adds no field to deps.
var newOptionPage = option_boards.NewOptionPage

func init() {
	registerPage(pageFamily{name: optionPageName, route: siteOptionRoute, order: 40,
		build: buildOptionPage, named: optionPageNamed})
}

// buildOptionPage builds option_page per request for the day of dayFlag or today in the zone of the profile,
// from the candidate and option_eod readers of the site. A nil zone is an error.
func buildOptionPage(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New(optionPageName + ": no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	symbols := cardSymbols(ctx, rd, day, nil)
	return optionPageWith(a, "ot-serve", rd.cands, rd.eod, rd.names, rd.openQuotes(), rd.live.state(ctx, symbols)).
		Build(ctx, nil, day, clk.Now().UTC())
}

// optionPageOf calls newOptionPage with the plain arguments (scope "options").
func optionPageOf(a *app, runID string, cands store.CandidateReader, eod store.OptionEodReader, names store.InstrumentNameReader) pages.Page {
	return optionPageWith(a, runID, cands, eod, names, nil, option_boards.RefreshState{})
}

// optionPageWith is optionPageOf with the overlay reader of the open and refresh rows and the state of the button.
func optionPageWith(a *app, runID string, cands store.CandidateReader, eod store.OptionEodReader, names store.InstrumentNameReader,
	quotes openquote.OptionOpenReader, refresh option_boards.RefreshState) pages.Page {
	return newOptionPage(option_boards.OptionConfig{
		RunID: runID, BuildID: buildIDOf(version), Scope: optionPageScope, Zone: a.profile.Zone,
		Candidates: cands, Eod: eod, Names: names, Quotes: quotes, Refresh: refresh,
	})
}

// optionPageNamed returns option_page for ot page and a close function: the candidate and option_eod stores
// opened once with the pages line of .env, nil readers with a missing line (the page shows its gap no_reader),
// or the demo readers with the demo profile (no store). A nil zone is an error raised before any store is opened.
func optionPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New(optionPageName + " needs the owner's zone: " + zoneEnvName + " is not set")
	}
	if a.profile.Name == "demo" {
		return optionPageOf(a, "ot-page", demoCandidates{}, demoOption{}, demoOption{}), func() {}, nil
	}
	env, err := a.envFile()
	if err != nil {
		return pages.Page{}, nil, err
	}
	dsn := env[pagesDSNName]
	if dsn == "" {
		return optionPageOf(a, "ot-page", nil, nil, nil), func() {}, nil
	}
	cands, err := a.deps.openCandidate(ctx, dsn)
	if err != nil {
		return pages.Page{}, nil, fmt.Errorf("open candidate store with %s failed", pagesDSNName)
	}
	eod, err := a.deps.openOptionEod(ctx, dsn)
	if err != nil {
		_ = cands.Close()
		return pages.Page{}, nil, fmt.Errorf("open option_eod store with %s failed", pagesDSNName)
	}
	names, err := a.deps.openInstrumentName(ctx, dsn)
	if err != nil {
		_ = eod.Close()
		_ = cands.Close()
		return pages.Page{}, nil, fmt.Errorf("open instrument name store with %s failed", pagesDSNName)
	}
	return optionPageOf(a, "ot-page", cands, eod, names), func() { _ = names.Close(); _ = eod.Close(); _ = cands.Close() }, nil
}

// isCardRow reports whether a short put row is the card contract of its underlying: ok and rank 1.
func isCardRow(r store.Candidate) bool {
	return r.Status == "ok" && r.RankInUnderlying != nil && *r.RankInUnderlying == 1
}

// cardSymbols are the tickers of the cards of a page: the ok rank 1 short puts of the day, at most maxCards
// underlyings, each named by the name reader; match, when set, keeps the underlying a stock page shows.
func cardSymbols(ctx context.Context, rd siteReaders, day contracts.Day, match func(store.Candidate, string) bool) []string {
	if rd.cands == nil || rd.names == nil {
		return nil
	}
	rows, err := rd.cands.Candidates(ctx, day, liveCardKind)
	if err != nil {
		return nil
	}
	var picked []store.Candidate
	for _, r := range rows {
		if r.Status == "ok" {
			picked = append(picked, r)
		}
	}
	var ids []int64
	for _, r := range picked {
		if id := int64(r.UnderlyingID); !slices.Contains(ids, id) && len(ids) < 500 {
			ids = append(ids, id)
		}
	}
	ns, err := rd.names.InstrumentNames(ctx, ids)
	if err != nil {
		return nil
	}
	name := map[int64]string{}
	for _, n := range ns {
		name[n.InstrumentID] = n.Name
	}
	var out []string
	for _, r := range picked {
		s := strings.ToUpper(name[int64(r.UnderlyingID)])
		if s == "" || slices.Contains(out, s) {
			continue
		}
		if match == nil && isCardRow(r) && len(out) < 6 {
			out = append(out, s)
		} else if match != nil && match(r, s) {
			out = append(out, s)
		}
	}
	return out
}

// marketTargets builds the asked-able targets of the planned items of m whose underlying is one of syms.
func (l *liveQuotes) marketTargets(ctx context.Context, m liveMarket, day contracts.Day, syms []string) ([]optionopen.Target, error) {
	items, err := l.d.Plan.Plan(ctx, day, m.slot)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	var unders []int64
	for _, it := range items {
		if id := int64(it.UnderlyingID); !slices.Contains(unders, id) {
			unders = append(unders, id)
		}
	}
	uis, err := l.d.Insts.Instruments(ctx, unders)
	if err != nil {
		return nil, err
	}
	want := map[int64]bool{}
	for _, in := range uis {
		if slices.Contains(syms, strings.ToUpper(in.Spec.Name)) {
			want[in.ID] = true
		}
	}
	var mine []contracts.OptionOpenItem
	ids := map[int64]bool{}
	for _, it := range items {
		if want[int64(it.UnderlyingID)] && !it.Deferred {
			mine = append(mine, it)
			ids[int64(it.InstrumentID)] = true
		}
	}
	for id := range want {
		ids[id] = true
	}
	conID, spec, err := l.conIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	refs := map[int64]map[int64]openquote.OptionRef{}
	var out []optionopen.Target
	for _, it := range mine {
		u, o := int64(it.UnderlyingID), int64(it.InstrumentID)
		if refs[u] == nil {
			list, err := l.d.Contracts.ListOptions(ctx, u)
			if err != nil {
				return nil, err
			}
			refs[u] = map[int64]openquote.OptionRef{}
			for _, r := range list {
				refs[u][r.InstrumentID] = r
			}
		}
		if t, ok := liveTarget(m, it, spec[u], refs[u][o], conID[u], conID[o]); ok {
			out = append(out, t)
		}
	}
	return out, nil
}

// conIDs reads the instruments of ids and returns their gateway contract ids and specs.
func (l *liveQuotes) conIDs(ctx context.Context, ids map[int64]bool) (map[int64]int64, map[int64]store.InstrumentSpec, error) {
	idList := make([]int64, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	insts, err := l.d.Insts.Instruments(ctx, idList)
	if err != nil {
		return nil, nil, err
	}
	conID, spec := map[int64]int64{}, map[int64]store.InstrumentSpec{}
	for _, in := range insts {
		spec[in.ID] = in.Spec
		for _, idf := range in.Identifiers {
			if n, err := strconv.ParseInt(idf.Value, 10, 64); err == nil && idf.Scheme == "ibkr_conid" {
				conID[in.ID] = n
			}
		}
	}
	return conID, spec, nil
}

// liveTarget builds the target of one item; false when a contract field is missing or malformed (nothing is asked).
func liveTarget(m liveMarket, it contracts.OptionOpenItem, us store.InstrumentSpec, ref openquote.OptionRef, uCon, oCon int64) (optionopen.Target, bool) {
	if uCon == 0 || oCon == 0 || ref.InstrumentID == 0 {
		return optionopen.Target{}, false
	}
	exp, err := time.Parse(time.DateOnly, string(ref.Expiry))
	if err != nil {
		return optionopen.Target{}, false
	}
	strike, err := strconv.ParseFloat(ref.Strike, 64)
	if err != nil {
		return optionopen.Target{}, false
	}
	ccy := m.currency
	if us.Currency != "" {
		ccy = us.Currency
	}
	return optionopen.Target{
		Item: it, Symbol: us.Name, Currency: ccy, Expiry: exp, Strike: strike, Right: optionopen.Right(ref.Right),
		Under: broker.Contract{SecType: "STK", Symbol: us.Name, Exchange: m.exchange, Currency: ccy, ConID: uCon},
		Option: broker.Contract{SecType: "OPT", Symbol: us.Name, Exchange: ref.Exchange, Currency: ref.Currency,
			Expiry: strings.ReplaceAll(string(ref.Expiry), "-", ""), Strike: ref.Strike, Right: ref.Right,
			Multiplier: ref.Multiplier, TradingClass: ref.TradingClass, ConID: oCon},
	}, true
}
