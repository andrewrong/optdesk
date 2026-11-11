package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// siteTrackingRoute is the address of the page option_tracking_page under the site root. The bar item that
// points to it belongs to web (docs/design/option-tracking.md, entry 9).
const siteTrackingRoute = "options/tracking"

// trackingName is the name of the page option_tracking_page, which pageNamed selects for ot page.
const trackingName = "option_tracking_page"

// trackingReaders are the readers of option_tracking_page.Config, each nil when its store did not open (the
// page then shows its own gap).
type trackingReaders struct {
	Ledger     store.OptionLedgerReader
	Account    store.AccountReader
	Candidates store.CandidateReader
	Eod        store.OptionEodReader
	Catalog    store.OptionEodCatalog
	Names      store.InstrumentNameReader
	Terms      store.OptionTermsReader
	Sessions   store.SessionDayReader
}

// trackingReadersOf picks the readers of the page from the readers of the site. Sessions is rd.sessions, opened
// with the pages line (view rd.session_day_r1); nil with the demo profile, which the page shows as the gap no_sessions.
func trackingReadersOf(rd siteReaders) trackingReaders {
	var ledger store.OptionLedgerReader
	if rd.optionLedger != nil {
		ledger = aliasedLedger{OptionLedgerReader: rd.optionLedger, configured: rd.aliasList()}
	}
	return trackingReaders{
		Ledger: ledger, Account: rd.noticeAccount, Candidates: rd.cands, Eod: rd.eod,
		Catalog: rd.catalog, Names: rd.names, Terms: rd.terms, Sessions: rd.sessions,
	}
}

// aliasedLedger is the ledger reader of the page: it answers StoredAliases with the configured aliases while the
// ledger stores no row, so the page names its empty account by the alias, not by "main".
type aliasedLedger struct {
	store.OptionLedgerReader
	configured []string
}

// StoredAliases returns the stored aliases, the inner error unchanged, or a copy of the configured aliases
// (never nil) when none is stored.
func (l aliasedLedger) StoredAliases(ctx context.Context) ([]string, error) {
	got, err := l.OptionLedgerReader.StoredAliases(ctx)
	if err != nil || len(got) > 0 {
		return got, err
	}
	return slices.Clone(l.configured), nil
}

func init() {
	registerPage(pageFamily{name: trackingName, route: siteTrackingRoute, order: 30, build: buildTracking, named: trackingPageNamed})
}

// buildTracking builds option_tracking_page per request for the day of dayFlag or today in the zone of the
// profile, from the readers of the site. A nil zone is an error.
func buildTracking(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New(trackingName + ": no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	return newTrackingPage(a, "ot-serve", trackingReadersOf(rd)).Build(ctx, nil, day, clk.Now().UTC())
}

// newTrackingPage calls deps.optionTrackingPage with the plain arguments (scope "tracking").
func newTrackingPage(a *app, runID string, r trackingReaders) pages.Page {
	return a.deps.optionTrackingPage(a.profile.Zone, runID, buildIDOf(version), "tracking", r)
}

// trackingPageNamed returns option_tracking_page for ot page and a close function: the stores opened once with
// the pages line of .env, no reader at all with a missing line (the page shows its gaps), or the demo readers
// with the demo profile (no store). A nil zone is an error raised before any store is opened.
func trackingPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New(trackingName + " needs the owner's zone: " + zoneEnvName + " is not set")
	}
	if a.profile.Name != "demo" {
		env, err := a.envFile()
		if err != nil {
			return pages.Page{}, nil, err
		}
		if env[pagesDSNName] == "" {
			return newTrackingPage(a, "ot-page", trackingReaders{}), func() {}, nil
		}
	}
	rd, closeFn, err := openSiteReaders(ctx, a)
	if err != nil {
		return pages.Page{}, nil, err
	}
	return newTrackingPage(a, "ot-page", trackingReadersOf(rd)), closeFn, nil
}

// demoTrackingLedger is the ledger reader of the demo profile: for the main alias, rows that replay to two
// open short options (a put and a call) and one recent assignment of a put with its shares; no other alias.
type demoTrackingLedger struct{}

// demoLedgerRow builds one stored row of the main alias from its attributes.
func demoLedgerRow(tx string, attrs string) store.FlexTradeRow {
	return store.FlexTradeRow{
		Alias: demoNoticesMain, LevelOfDetail: "EXECUTION", TransactionID: tx, Revision: 1,
		Attributes: attrs, Source: "demo", RecordedAt: time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC),
	}
}

func demoOptAttrs(expiry, strike, right, qty, proceeds, oc, notes, date string) string {
	return fmt.Sprintf(`{"assetCategory":"OPT","symbol":"DEMO","underlyingSymbol":"DEMO","conid":"%s%s","strike":%q,`+
		`"expiry":%q,"putCall":%q,"multiplier":"100","quantity":%q,"proceeds":%q,"ibCommission":"-0.65",`+
		`"currency":"USD","openCloseIndicator":%q,"notes":%q,"tradeDate":%q,"orderTime":"%s;100000"}`,
		right, strike, strike, expiry, right, qty, proceeds, oc, notes, date, date)
}

// FlexTradeRows returns the demo rows of the main alias; any other alias has none.
func (demoTrackingLedger) FlexTradeRows(_ context.Context, alias string) ([]store.FlexTradeRow, error) {
	if alias != demoNoticesMain {
		return []store.FlexTradeRow{}, nil
	}
	return []store.FlexTradeRow{
		demoLedgerRow("d1", demoOptAttrs("20261120", "95", "P", "-1", "120", "O", "", "20261020")),
		demoLedgerRow("d2", demoOptAttrs("20261218", "105", "C", "-1", "90", "O", "", "20261023")),
		demoLedgerRow("d3", demoOptAttrs("20261030", "80", "P", "-1", "70", "O", "", "20261001")),
		demoLedgerRow("d4", demoOptAttrs("20261030", "80", "P", "1", "0", "C", "A", "20261030")),
		demoLedgerRow("d5", `{"assetCategory":"STK","symbol":"DEMO","quantity":"100","tradePrice":"80","proceeds":"-8000",`+
			`"ibCommission":"0","currency":"USD","notes":"A","tradeDate":"20261030","orderTime":"20261030;160000"}`),
	}, nil
}

// LatestOkFetch returns no fetch record.
func (demoTrackingLedger) LatestOkFetch(context.Context, string, string) (store.FlexFetchRecord, bool, error) {
	return store.FlexFetchRecord{}, false, nil
}

// StoredAliases returns the one demo alias.
func (demoTrackingLedger) StoredAliases(context.Context) ([]string, error) {
	return []string{demoNoticesMain}, nil
}
