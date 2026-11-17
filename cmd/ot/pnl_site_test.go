package main

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/market_state_page"
	"github.com/andrewrong/optdesk/internal/pages/option_pnl_page"
	"github.com/andrewrong/optdesk/internal/store"
)

type pnlFixture struct {
	a      *app
	builds atomic.Int32
	cfg    option_pnl_page.Config
	built  int
	series store.SeriesReader
}

func newPnlFixture(t *testing.T, env string) *pnlFixture {
	t.Helper()
	f := &pnlFixture{}
	f.a, _ = testApp(t, env)
	f.a.deps.optionPnlPage = func(c option_pnl_page.Config) pages.Page {
		f.cfg = c
		f.built++
		build := func(_ context.Context, r store.SeriesReader, _ contracts.Day, _ time.Time) ([]byte, error) {
			f.builds.Add(1)
			f.series = r
			return []byte(`{"schema":"option_pnl_page"}`), nil
		}
		return pages.Page{Meta: pages.Meta{Name: pnlName}, Build: build}
	}
	return f
}

type pnlIDs struct{}

func (pnlIDs) StockIDs(context.Context, []string) ([]store.InstrumentName, error) { return nil, nil }

func TestPnlRouteServes(t *testing.T) {
	t.Parallel()
	f := newPnlFixture(t, "")
	h := siteHandler(f.a, trackingClock, "2026-11-05", siteReaders{}, echoHandler, http.NotFoundHandler())
	rec := trackingGet(t, h, http.MethodGet, "/"+siteOptionPnlRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"schema":"option_pnl_page"`) {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
}

type pnlEmptyLedger struct{ store.OptionLedgerReader }

func (pnlEmptyLedger) StoredAliases(context.Context) ([]string, error) { return nil, nil }

func TestPnlReadersFromSite(t *testing.T) {
	t.Parallel()
	rd := siteReaders{optionLedger: pnlEmptyLedger{}, aliases: []string{"acct1"}}
	got, err := pnlReadersOf(rd).Ledger.StoredAliases(t.Context())
	if err != nil || len(got) != 1 || got[0] != "acct1" {
		t.Fatalf("aliases %v %v", got, err)
	}
	if pnlReadersOf(siteReaders{}).Ledger != nil {
		t.Fatal("nil ledger not kept nil")
	}
}

func TestPnlNilZoneIsError(t *testing.T) {
	t.Parallel()
	f := newPnlFixture(t, "")
	f.a.profile.Zone = nil
	if _, err := buildPnl(t.Context(), f.a, trackingClock, "", siteReaders{}); err == nil {
		t.Fatal("buildPnl: nil zone built")
	}
	if _, _, err := pnlPageNamed(t.Context(), f.a); err == nil {
		t.Fatal("pnlPageNamed: nil zone")
	}
	if f.built != 0 {
		t.Fatal("page constructed")
	}
}

func TestPnlNamedWithoutLineHasGaps(t *testing.T) {
	t.Parallel()
	f := newPnlFixture(t, "OT_X=1\n")
	f.a.profile.Name = "real"
	f.a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		t.Error("store opened")
		return nil, nil
	}
	_, closeFn, err := pnlPageNamed(t.Context(), f.a)
	if err != nil {
		t.Fatal(err)
	}
	closeFn()
	c := f.cfg
	if f.built != 1 || c.Ledger != nil || c.Sessions != nil || c.Days != nil || c.StockIDs != nil || c.Bars != nil {
		t.Fatalf("config %+v", c)
	}
}

func TestPnlPassesAttrAndBooker(t *testing.T) {
	t.Parallel()
	f := newPnlFixture(t, "")
	if _, err := buildPnl(t.Context(), f.a, trackingClock, "2026-11-05", siteReaders{}); err != nil {
		t.Fatal(err)
	}
	c := f.cfg
	if c.Attributor == nil || c.Booker == nil || c.Replay == nil {
		t.Fatalf("config %+v", c)
	}
	if c.MarkMaxAgeSessions != 3 || c.RecentDays != 30 || c.CalendarID != "XNYS" {
		t.Fatalf("config %+v", c)
	}
}

func TestPnlMaxRateAgeIsSeven(t *testing.T) {
	t.Parallel()
	f := newPnlFixture(t, "")
	if _, err := buildPnl(t.Context(), f.a, trackingClock, "2026-11-05", siteReaders{}); err != nil {
		t.Fatal(err)
	}
	if f.cfg.MaxRateAgeDays != 7 || pnlMaxRateAgeDays != 7 {
		t.Fatalf("MaxRateAgeDays %d", f.cfg.MaxRateAgeDays)
	}
}

func TestPnlStockIDsFromExt(t *testing.T) {
	t.Parallel()
	f := newPnlFixture(t, "")
	ids := pnlIDs{}
	rd := siteReaders{ext: map[string]any{siteMarketStateIDsKey: market_state_page.StockIDReader(ids)}}
	if _, err := buildPnl(t.Context(), f.a, trackingClock, "2026-11-05", rd); err != nil {
		t.Fatal(err)
	}
	if f.cfg.StockIDs != option_pnl_page.StockIDReader(ids) {
		t.Fatalf("StockIDs %v", f.cfg.StockIDs)
	}
}

type pnlSeries struct{ store.SeriesReader }

func TestPnlBuildGetsSeriesReader(t *testing.T) {
	t.Parallel()
	f := newPnlFixture(t, "")
	s := pnlSeries{}
	if _, err := buildPnl(t.Context(), f.a, trackingClock, "2026-11-05", siteReaders{series: s}); err != nil {
		t.Fatal(err)
	}
	if f.series != store.SeriesReader(s) {
		t.Fatalf("series %v", f.series)
	}
	f.series = nil
	f.a.profile.Name = "demo"
	p, closeFn, err := pnlPageNamed(t.Context(), f.a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	if _, err := p.Build(t.Context(), nil, "2026-11-05", time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}
	if f.series == nil {
		t.Fatal("pnlPageNamed passed nil series")
	}
}
