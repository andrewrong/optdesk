package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/hoursnotice"
	"github.com/andrewrong/optdesk/internal/store/hoursnoticepg"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
	"github.com/andrewrong/optdesk/internal/store/mark"
	"github.com/andrewrong/optdesk/internal/store/openquote"
	"github.com/andrewrong/optdesk/internal/store/optionopenpg"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
	"github.com/andrewrong/optdesk/internal/store/runview"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// pagesDSNName is the only .env line the site reads.
const pagesDSNName = store.DSNEnvPages

const (
	siteRoot       = "options/data"
	siteCatalogDay = 7  // days; the window option_data_page reads before the build day (its lookbackDays)
	siteNameIDs    = 50 // ids of one name read

	emptyStoreNote    = "No option data is stored yet; collection has not run."
	noUnderlyingLabel = "No underlying yet"
)

// siteChoice is one underlying the page links to.
type siteChoice struct {
	ID          int64
	Label, Href string
}

// sitePageInput is what the site hands deps.optionDataPage: the underlying, the readers and the choices.
type sitePageInput struct {
	UnderlyingID int64
	Label        string
	Symbol       string   // the name of the underlying; empty when unknown
	Notes        []string // texts the page shows as notes
	Eod          store.OptionEodReader
	Quotes       store.QuoteSnapshotReader
	Terms        store.OptionTermsReader
	Choices      []siteChoice
	Reasons      runview.Reader // the stored reasons of the day (nil: none)
}

// openOptionOpenStores opens the option open stores with a DSN (a test replaces it).
var openOptionOpenStores openquote.OpenFunc = optionopenpg.Open

// openFamilyReaders runs the openers of siteOpeners in the order of their keys; ok is false when one failed.
func openFamilyReaders(ctx context.Context, a *app, track func(io.Closer, error) bool) (map[string]any, bool) {
	if len(siteOpeners) == 0 {
		return nil, true
	}
	keys := make([]string, 0, len(siteOpeners))
	for k := range siteOpeners {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ext := make(map[string]any, len(keys))
	for _, k := range keys {
		v, ok := siteOpeners[k](ctx, a, track)
		if !ok {
			return nil, false
		}
		ext[k] = v
	}
	return ext, true
}

// openOpenStoresOrNil opens the option open stores with the pages line; a failed open is logged and gives nil (the
// pages then show every quote as its close archive value and the refresh route cannot start).
func openOpenStoresOrNil(ctx context.Context, a *app, dsn string, track func(io.Closer, error) bool) openquote.Stores {
	oos, err := openOptionOpenStores(ctx, dsn)
	if err != nil {
		a.log.Warn("option open store not opened", "component", "ot", "status", "failed")
		return nil
	}
	track(oos, nil)
	return oos
}

// openQuotes returns the reader the option pages read open and refresh rows from: the overlay of the flag live-quotes
// over the stored rows, else the stored reader, else nil.
func (rd siteReaders) openQuotes() openquote.OptionOpenReader {
	switch {
	case rd.live != nil:
		return rd.live
	case rd.open != nil:
		return rd.open
	}
	return nil
}

// siteReaders are the readers the site builds its live pages from (the eight stores first, then the optional ones).
type siteReaders struct {
	series  store.SeriesReader
	eod     store.OptionEodReader
	quotes  store.QuoteSnapshotReader
	catalog store.OptionEodCatalog
	terms   store.OptionTermsReader
	names   store.InstrumentNameReader
	view    store.ReadingViewReader
	cands   store.CandidateReader

	// marks is the position mark reader of household_page, opened with the pages line (nil: no opener or a
	// failed open, the blocks that need a mark stay gaps).
	marks mark.Reader

	// hours is the reader of the hours notices of the home overview, opened with the pages line (nil: the demo
	// profile or a failed open; the overview then shows no notice).
	hours hoursnotice.Reader

	// bars is the daily bar reader of the home Companies card, opened with the pages line (nil: no opener, the demo
	// profile or a site with no bar line; the day change stays the gap no_reader).
	bars bar.PriceBarReader

	// fx is the stored FX rate reader of household_page, from the same bar store as bars (nil when bars is nil).
	fx bar.FxRateReader

	// the settings page: the owner reader (nil with no opener in deps, which the page shows as the gap no_reader).
	owner ownerset.Reader

	// underlying is the reader of the underlyings page (nil: no opener or a failed open, the gap no_reader).
	underlying underlying.Reader

	// jobReq is the reader of the job console page (nil: no opener or a failed open, the gap job_requests_unavailable).
	jobReq jobrequest.Reader

	// open, plan and optContracts are the stores of the option open pull opened with the pages line (nil in the demo
	// profile or when the open failed: the pages show every quote as its close archive value). live is the refresh
	// route and the overlay of `ot serve --live-quotes` (nil: the route options/refresh is 404).
	open         openquote.OptionOpenReader
	plan         openquote.OptionOpenPlanStore
	optContracts openquote.OptionContractStore
	live         *liveQuotes

	// edit is the POST route /owner-settings (nil: the route is absent, 404).
	edit *ownerEdit

	// the notices page: the account and earnings readers (nil with no opener in deps, which the page shows as
	// the gap no_reader). The positions pages keep their own account field below.
	noticeAccount store.AccountReader
	earnings      store.EarningsDateReader

	// the option positions pages: the ledger and account readers (the ones above, nil with no opener in deps
	// and in the demo profile, which the builders turn into the gap no_reader), the session days and the compute constructors; eod and terms
	// above are read by these pages too.
	ledger     store.OptionLedgerReader
	account    store.AccountReader
	sessions   store.SessionDayReader
	ledgers    compute.NewOptionLedgerFunc
	structures compute.NewOptionStructuresFunc
	results    compute.NewOptionResultsFunc

	// aliases is the alias list of the account pages, read once at serve start from the gateways file (empty: ["main"]).
	aliases []string

	// the tracking page: the option ledger reader of the pages line (nil with no opener in deps).
	optionLedger store.OptionLedgerReader

	// ext holds the readers a page family opens itself through siteOpeners, by key; nil in the demo profile.
	ext map[string]any
}

// siteOpener opens the reader of one page family on the shared closers (track registers a closer when its error is
// nil and answers whether to go on). It answers the reader and false when the open failed.
type siteOpener func(ctx context.Context, a *app, track func(io.Closer, error) bool) (any, bool)

// siteOpeners is the registry of those openers by key; a page family adds its own in an init().
var siteOpeners = map[string]siteOpener{}

// siteReaderOf is the reader of type T that the opener of key put in rd.ext: the zero value and false when ext is
// nil or holds no such reader.
func siteReaderOf[T any](rd siteReaders, key string) (T, bool) {
	v, ok := rd.ext[key].(T)
	return v, ok
}

// openSiteReaders returns the demo readers with the demo profile, else the stores (series, the
// five option stores, the reading view, the candidate store, and the account and earnings stores) opened once with the one .env line of the
// pages role (the session-day store included), and a function that closes them. A failed open closes the stores already opened.
//
//nolint:funlen // one flat sequence of opens, each with its fail line; splitting it would hide the close order
func openSiteReaders(ctx context.Context, a *app) (siteReaders, func(), error) {
	if a.profile.Name == "demo" {
		d := demoOption{}
		rd := siteReaders{series: demoReader{}, eod: d, quotes: d, catalog: d, terms: d, names: d, view: demoView{}, cands: demoCandidates{},
			noticeAccount: demoNoticeAccount{}, optionLedger: demoTrackingLedger{}}
		return withPositionsRules(a, rd), func() {}, nil
	}
	env, err := a.envFile()
	if err != nil {
		return siteReaders{}, nil, err
	}
	dsn := env[pagesDSNName]
	if dsn == "" {
		return siteReaders{}, nil, fmt.Errorf("%s is missing in .env", pagesDSNName)
	}
	var closers []func() error
	closeAll := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			_ = closers[i]()
		}
	}
	fail := func() (siteReaders, func(), error) {
		closeAll()
		return siteReaders{}, nil, fmt.Errorf("open stores with %s failed", pagesDSNName)
	}
	track := func(c io.Closer, err error) bool {
		if err == nil {
			closers = append(closers, c.Close)
		}
		return err == nil
	}
	ser, err := a.deps.openSeries(ctx, dsn)
	if !track(ser, err) {
		return fail()
	}
	eod, err := a.deps.openOptionEod(ctx, dsn)
	if !track(eod, err) {
		return fail()
	}
	quotes, err := a.deps.openQuoteSnapshot(ctx, dsn)
	if !track(quotes, err) {
		return fail()
	}
	catalog, err := a.deps.openOptionCatalog(ctx, dsn)
	if !track(catalog, err) {
		return fail()
	}
	terms, err := a.deps.openOptionTerms(ctx, dsn)
	if !track(terms, err) {
		return fail()
	}
	names, err := a.deps.openInstrumentName(ctx, dsn)
	if !track(names, err) {
		return fail()
	}
	view, err := a.deps.openReadingView(ctx, dsn)
	if !track(view, err) {
		return fail()
	}
	oos := openOpenStoresOrNil(ctx, a, dsn, track)
	opt, ledger, ok := openOptionalStores(ctx, a, dsn, track)
	if !ok {
		return fail()
	}
	owner, ok := openOwnerReader(ctx, a, dsn, track)
	if !ok {
		return fail()
	}
	under, ok := openUnderlyingReader(ctx, a, dsn, track)
	if !ok {
		return fail()
	}
	jobReq := openJobRequestReader(ctx, a, dsn, track)
	sessions, ok := openSessionReader(ctx, a, dsn, track)
	if !ok {
		return fail()
	}
	bars, fx, ok := openBarReader(ctx, a, dsn, track)
	if !ok {
		return fail()
	}
	rd := siteReaders{
		hours: openHoursReader(ctx, dsn, track),
		bars:  bars, fx: fx, marks: openMarkReader(ctx, a, dsn, track),
		series: ser, eod: eod, quotes: quotes, catalog: catalog, terms: terms, names: names, view: view,
		cands: opt.cands, noticeAccount: opt.account, earnings: opt.earn, owner: owner, underlying: under, jobReq: jobReq,
		optionLedger: ledger, ledger: ledger, account: opt.account, sessions: sessions, aliases: siteAliasList(a)}
	if oos != nil {
		rd.open, rd.plan, rd.optContracts = oos, oos, oos
	}
	var extOK bool
	if rd.ext, extOK = openFamilyReaders(ctx, a, track); !extOK {
		return fail()
	}
	return withPositionsRules(a, rd), closeAll, nil
}

// openOptionalStores opens the candidate, account, option ledger (the reader returned apart) and earnings stores that deps has an opener for; a missing
// opener is a nil reader, which the pages show as the gap no_reader. ok is false when an open failed.
func openOptionalStores(ctx context.Context, a *app, dsn string,
	track func(io.Closer, error) bool) (r noticesReaders, ledger store.OptionLedgerReader, ok bool) {
	if a.deps.openCandidate != nil {
		cs, err := a.deps.openCandidate(ctx, dsn)
		if !track(cs, err) {
			return r, nil, false
		}
		r.cands = cs
	}
	if a.deps.openAccount != nil {
		as, err := a.deps.openAccount(ctx, dsn)
		if !track(as, err) {
			return r, nil, false
		}
		r.account = as
	}
	if a.deps.openOptionLedgerRO != nil {
		ls, err := a.deps.openOptionLedgerRO(ctx, dsn)
		if !track(ls, err) {
			return r, nil, false
		}
		ledger = ls
	}
	if a.deps.openEarningsDates != nil {
		es, err := a.deps.openEarningsDates(ctx, dsn)
		if !track(es, err) {
			return r, nil, false
		}
		r.earn = es
	}
	return r, ledger, true
}

// openOwnerReader opens the owner settings reader when deps has an opener, else it is nil (the page shows the
// gap no_reader). ok is false when the open failed.
func openOwnerReader(ctx context.Context, a *app, dsn string, track func(io.Closer, error) bool) (ownerset.Reader, bool) {
	if a.deps.openOwnerSet == nil {
		return nil, true
	}
	oset, err := a.deps.openOwnerSet(ctx, dsn)
	if !track(oset, err) {
		return nil, false
	}
	return oset, true
}

// openSessionReader opens the session-day store with the pages line when deps has an opener, else the reader is
// nil (the pages show the gap no_sessions). ok is false when the open failed.
func openSessionReader(ctx context.Context, a *app, dsn string, track func(io.Closer, error) bool) (store.SessionDayReader, bool) {
	if a.deps.openSessionDay != nil {
		sd, err := a.deps.openSessionDay(ctx, dsn)
		if !track(sd, err) {
			return nil, false
		}
		return sd, true
	}
	return nil, true
}

// withPositionsRules sets the compute constructors of the positions pages; the ledger, account and session
// readers are set by openSiteReaders.
func withPositionsRules(a *app, rd siteReaders) siteReaders {
	rd.ledgers, rd.structures, rd.results = a.deps.newOptionLedger, a.deps.newOptionStructures, a.deps.newOptionResults
	return rd
}

// siteHandler serves the routes of the site: health (before any page route), "" (302 to home), the live routes macro, options/data and
// options/data/<id>, the routes report, market and options/candidates and options/notices, the registered page routes and
// assets/*, which assets answers. A live page is built per request;
// an error answers 500 with a fixed text. Any other path is 404 and a method other than GET and HEAD is 405.
func siteHandler(a *app, clk clock.Clock, dayFlag string, rd siteReaders, newHandler pages.NewHandlerFunc, assets http.Handler) http.Handler {
	pagesH := sitePages(a, clk, dayFlag, rd, newHandler, assets)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimPrefix(r.URL.Path, "/") == siteHealthRoute {
			serveHealth(a, clk, rd, w, r)
			return
		}
		if strings.TrimPrefix(r.URL.Path, "/") == liveRefreshRoute {
			if rd.live == nil {
				http.NotFound(w, r)
				return
			}
			rd.live.ServeHTTP(w, r)
			return
		}
		if rd.edit != nil && ownerLogin(rd, w, r) {
			return
		}
		if strings.TrimPrefix(r.URL.Path, "/") == ownerPostRoute && rd.edit != nil {
			postOwnerSettings(a, clk, dayFlag, rd, newHandler, w, r)
			return
		}
		if strings.TrimPrefix(r.URL.Path, "/") == underlyingsPostRoute && rd.edit != nil {
			postOwnerUnderlyings(a, clk, dayFlag, rd, newHandler, w, r)
			return
		}
		if strings.TrimPrefix(r.URL.Path, "/") == priceLinesRoute && r.Method == http.MethodPost && rd.edit != nil {
			postOwnerPriceLines(a, clk, rd, newHandler, w, r)
			return
		}
		if strings.TrimPrefix(r.URL.Path, "/") == jobConsolePostRoute && rd.edit != nil {
			postOwnerJobConsole(a, clk, rd, w, r)
			return
		}
		if strings.TrimPrefix(r.URL.Path, "/") == flexImportPostRoute && rd.edit != nil {
			postOwnerFlexImport(a, clk, rd, newHandler, w, r)
			return
		}
		pagesH.ServeHTTP(w, r)
	})
}

// sitePages serves every route of the site but /health.
func sitePages(a *app, clk clock.Clock, dayFlag string, rd siteReaders, newHandler pages.NewHandlerFunc, assets http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		var id int64
		var build func(context.Context) ([]byte, error)
		isAsset := false
		build = extraPageBuilder(path, r.URL.Query(), a, clk, dayFlag, rd)
		switch {
		case path == "":
		case path == "macro":
			build = func(ctx context.Context) ([]byte, error) { return buildMacro(ctx, a, clk, dayFlag, rd) }
		case path == siteMarketRoute:
			build = func(ctx context.Context) ([]byte, error) { return buildMarket(ctx, a, clk, dayFlag, rd) }
		case isPositionsPath(path):
			kind, sid := positionsRoute(path)
			q := r.URL.Query()
			build = func(ctx context.Context) ([]byte, error) {
				return buildPositions(ctx, a, clk, dayFlag, rd, kind, sid, q)
			}
		case build != nil:
		case path == "report":
			q := r.URL.Query()
			build = func(ctx context.Context) ([]byte, error) { return buildReport(ctx, a, clk, dayFlag, rd.view, q) }
		case path == siteRoot:
			build = func(ctx context.Context) ([]byte, error) { return buildSite(ctx, a, clk, dayFlag, rd, 0, false) }
		case strings.HasPrefix(path, siteRoot+"/"):
			n, ok := siteIDOf(strings.TrimPrefix(path, siteRoot+"/"))
			if !ok {
				http.NotFound(w, r)
				return
			}
			id = n
			build = func(ctx context.Context) ([]byte, error) { return buildSite(ctx, a, clk, dayFlag, rd, id, true) }
		case strings.HasPrefix(path, "assets/") && len(path) > len("assets/"):
			isAsset = true
		default:
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		switch {
		case path == "":
			http.Redirect(w, r, "/"+siteHomeRoute, http.StatusFound)
		case isAsset:
			assets.ServeHTTP(w, r)
		default:
			vm, err := build(r.Context())
			if errors.Is(err, errReportQuery) {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if err != nil {
				a.log.Error("serve build failed", "component", "ot", "status", "failed")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			newHandler(vm).ServeHTTP(w, r)
		}
	})
}

// extraPageBuilder is the builder of a registered page route (pagereg.go), nil for any other path. The query string
// reaches the build through its context.
func extraPageBuilder(path string, q url.Values, a *app, clk clock.Clock, dayFlag string, rd siteReaders) func(context.Context) ([]byte, error) {
	f, ok := pageFamilyOfRoute(path)
	if !ok || f.build == nil {
		kf, key, kok := pageFamilyOfKeyRoute(path)
		if !kok {
			return nil
		}
		return func(ctx context.Context) ([]byte, error) {
			return kf.buildKey(withQuery(ctx, q), a, clk, dayFlag, rd, key)
		}
	}
	return func(ctx context.Context) ([]byte, error) { return f.build(withQuery(ctx, q), a, clk, dayFlag, rd) }
}

// siteIDOf is the underlying id of the last element of an options/data path: a positive number written
// without a sign or a leading zero.
func siteIDOf(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil && n > 0 && strconv.FormatInt(n, 10) == s
}

// siteHealthRoute is the path of the health check, under the root of the site.
const siteHealthRoute = "health"

// siteHealthTimeout bounds the one series read of GET /health.
const siteHealthTimeout = 2 * time.Second

// serveHealth answers GET and HEAD /health: 200 {"status":"ok"} when the series reader answers one range
// of DGS10 on today's market day, else 503 {"status":"unavailable"}. The error text is never printed.
func serveHealth(a *app, clk clock.Clock, rd siteReaders, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), siteHealthTimeout)
	defer cancel()
	day, err := dayOrDefault(clk, "")
	if err == nil {
		_, err = rd.series.SeriesPoints(ctx, "DGS10", day, day)
	}
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		a.log.Error("serve health failed", "component", "ot", "status", "failed")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"status":"unavailable"}`)
		return
	}
	_, _ = io.WriteString(w, `{"status":"ok"}`)
}

// buildMacro builds the first page of deps.pageList (macro_rates) for the requested day from the series reader.
func buildMacro(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	return a.deps.pageList[0].Build(ctx, rd.series, day, clk.Now().UTC())
}

// buildSite builds the view model of the underlying id, or of the one with the greatest latest day
// when byID is false (id 0 when the catalog is empty).
func buildSite(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders, id int64, byID bool) ([]byte, error) {
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, err
	}
	since := contracts.Day(d.AddDate(0, 0, -siteCatalogDay).Format(dayLayout))
	entries, err := rd.catalog.OptionEodLatest(ctx, since)
	if err != nil {
		return nil, err
	}
	if !byID && len(entries) > 0 {
		id = bestOf(entries)
	}
	label, symbol := fmt.Sprintf("Underlying %d", id), ""
	var notes []string
	labels := map[int64]string{}
	if len(entries) == 0 {
		notes = []string{emptyStoreNote}
		if id == 0 {
			label = noUnderlyingLabel
		}
	} else {
		names, err := readSiteNames(ctx, rd.names, id, entries)
		if err != nil {
			return nil, err
		}
		for _, n := range names {
			if n.Type != "option" && n.Name != "" {
				labels[n.InstrumentID] = n.Name
			}
		}
		if n, ok := labels[id]; ok {
			label, symbol = n, n
		}
	}
	choices := make([]siteChoice, 0, len(entries))
	for _, e := range entries {
		l, ok := labels[e.UnderlyingID]
		if !ok {
			l = fmt.Sprintf("Underlying %d", e.UnderlyingID)
		}
		choices = append(choices, siteChoice{
			ID: e.UnderlyingID, Label: l,
			Href: "/" + siteRoot + "/" + strconv.FormatInt(e.UnderlyingID, 10),
		})
	}
	page := a.deps.optionDataPage(a.profile.Zone, buildIDOf(version), sitePageInput{
		UnderlyingID: id, Label: label, Symbol: symbol, Notes: notes,
		Eod: rd.eod, Quotes: rd.quotes, Terms: rd.terms, Choices: choices, Reasons: optionReasonReader(rd),
	})
	return page.Build(ctx, nil, day, clk.Now().UTC())
}

// bestOf is the underlying with the greatest latest day, the lowest id on a tie; entries is not empty.
func bestOf(entries []store.OptionEodLatest) int64 {
	best := entries[0]
	for _, e := range entries[1:] {
		if e.SessionDay > best.SessionDay || (e.SessionDay == best.SessionDay && e.UnderlyingID < best.UnderlyingID) {
			best = e
		}
	}
	return best.UnderlyingID
}

// readSiteNames reads, in one call, the names of the chosen id (when positive) and of the first entries,
// at most siteNameIDs ids in all.
func readSiteNames(ctx context.Context, r store.InstrumentNameReader, id int64, entries []store.OptionEodLatest) ([]store.InstrumentName, error) {
	ids := make([]int64, 0, siteNameIDs)
	if id > 0 {
		ids = append(ids, id)
	}
	for _, e := range entries {
		if len(ids) >= siteNameIDs {
			break
		}
		if !slices.Contains(ids, e.UnderlyingID) {
			ids = append(ids, e.UnderlyingID)
		}
	}
	return r.InstrumentNames(ctx, ids)
}

// openSiteHours opens the hours-notice reader with the pages line; a package variable so that the tests, which have
// no database, install a fake in an init().
var openSiteHours = func(ctx context.Context, dsn string) (hoursnotice.Reader, error) {
	st, err := hoursnoticepg.Open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return st, nil
}

// openHoursReader opens the hours-notice reader; a failed open is nil, which leaves the home overview without notices.
func openHoursReader(ctx context.Context, dsn string, track func(io.Closer, error) bool) hoursnotice.Reader {
	rd, err := openSiteHours(ctx, dsn)
	if !track(rd, err) {
		return nil
	}
	return rd
}

// openMarkReader opens the mark store with the pages line when deps has an opener. A nil opener or a failed
// open gives nil (the blocks that need a mark stay gaps); the site does not fail for it.
func openMarkReader(ctx context.Context, a *app, dsn string, track func(io.Closer, error) bool) mark.Reader {
	if a.deps.openMark == nil {
		return nil
	}
	ms, err := a.deps.openMark(ctx, dsn)
	if !track(ms, err) {
		return nil
	}
	return ms
}

// openBarReader opens the bar store when deps has an opener and returns it as the price bar reader and the FX
// rate reader (both nil without an opener); a failed open is false.
func openBarReader(ctx context.Context, a *app, dsn string, track func(io.Closer, error) bool) (bar.PriceBarReader, bar.FxRateReader, bool) {
	if a.deps.openBar == nil {
		return nil, nil, true
	}
	bs, err := a.deps.openBar(ctx, dsn)
	if !track(bs, err) {
		return nil, nil, false
	}
	return bs, bs, true
}
