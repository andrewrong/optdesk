package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
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
}

// siteReaders are the seven readers the site builds its live pages from.
type siteReaders struct {
	series  store.SeriesReader
	eod     store.OptionEodReader
	quotes  store.QuoteSnapshotReader
	catalog store.OptionEodCatalog
	terms   store.OptionTermsReader
	names   store.InstrumentNameReader
	view    store.ReadingViewReader
}

// openSiteReaders returns the demo readers with the demo profile, else the six stores (series and the
// five option stores) opened once with the one .env line of the pages role, and a function that closes
// them. A failed open closes the stores already opened.
func openSiteReaders(ctx context.Context, a *app) (siteReaders, func(), error) {
	if a.profile.Name == "demo" {
		d := demoOption{}
		return siteReaders{demoReader{}, d, d, d, d, d, demoView{}}, func() {}, nil
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
	var rd siteReaders
	ser, err := a.deps.openSeries(ctx, dsn)
	if err != nil {
		return fail()
	}
	closers = append(closers, ser.Close)
	eod, err := a.deps.openOptionEod(ctx, dsn)
	if err != nil {
		return fail()
	}
	closers = append(closers, eod.Close)
	quotes, err := a.deps.openQuoteSnapshot(ctx, dsn)
	if err != nil {
		return fail()
	}
	closers = append(closers, quotes.Close)
	catalog, err := a.deps.openOptionCatalog(ctx, dsn)
	if err != nil {
		return fail()
	}
	closers = append(closers, catalog.Close)
	terms, err := a.deps.openOptionTerms(ctx, dsn)
	if err != nil {
		return fail()
	}
	closers = append(closers, terms.Close)
	names, err := a.deps.openInstrumentName(ctx, dsn)
	if err != nil {
		return fail()
	}
	closers = append(closers, names.Close)
	view, err := a.deps.openReadingView(ctx, dsn)
	if err != nil {
		return fail()
	}
	closers = append(closers, view.Close)
	rd = siteReaders{ser, eod, quotes, catalog, terms, names, view}
	return rd, closeAll, nil
}

// siteHandler serves the routes of the site: "" (302 to home), the live routes macro, options/data and
// options/data/<id>, the route report, the six sample routes (sample.go) and assets/*, which assets answers. A live page is built per request;
// an error answers 500 with a fixed text. Any other path is 404 and a method other than GET and HEAD is 405.
func siteHandler(a *app, clk clock.Clock, dayFlag string, rd siteReaders, newHandler pages.NewHandlerFunc, assets http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		var id int64
		var build func(context.Context) ([]byte, error)
		isAsset := false
		switch {
		case path == "":
		case path == "macro":
			build = func(ctx context.Context) ([]byte, error) { return buildMacro(ctx, a, clk, dayFlag, rd) }
		case path == "report":
			q := r.URL.Query()
			build = func(ctx context.Context) ([]byte, error) { return buildReport(ctx, a, clk, dayFlag, rd.view, q) }
		case path == siteRoot:
			build = func(ctx context.Context) ([]byte, error) { return buildSite(ctx, a, clk, dayFlag, rd, 0, false) }
		case strings.HasPrefix(path, siteRoot+"/"):
			n, err := strconv.ParseInt(strings.TrimPrefix(path, siteRoot+"/"), 10, 64)
			if err != nil || n <= 0 || strconv.FormatInt(n, 10) != strings.TrimPrefix(path, siteRoot+"/") {
				http.NotFound(w, r)
				return
			}
			id = n
			build = func(ctx context.Context) ([]byte, error) { return buildSite(ctx, a, clk, dayFlag, rd, id, true) }
		case strings.HasPrefix(path, "assets/") && len(path) > len("assets/"):
			isAsset = true
		default:
			vm, ok := sampleVM(path)
			if !ok {
				http.NotFound(w, r)
				return
			}
			build = func(context.Context) ([]byte, error) { return vm, nil }
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		switch {
		case path == "":
			http.Redirect(w, r, "/"+sampleHome, http.StatusFound)
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
		Eod: rd.eod, Quotes: rd.quotes, Terms: rd.terms, Choices: choices,
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
