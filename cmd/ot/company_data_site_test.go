package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/market_state_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

var companySiteNow = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

// fakeCompanyUnderlying answers fixed rows.
type fakeCompanyUnderlying struct{ rows []underlying.Row }

func (f fakeCompanyUnderlying) Current(context.Context) ([]underlying.Row, error) { return f.rows, nil }
func (f fakeCompanyUnderlying) History(context.Context, int) ([]underlying.Row, error) {
	return nil, nil
}

// fakeCompanyFacts answers no fact and records the subjects it was asked for.
type fakeCompanyFacts struct {
	mu       *sync.Mutex
	subjects *[]string
}

func (f fakeCompanyFacts) FactsFiledBy(ctx context.Context, s, subj string, c contracts.Day) ([]contracts.FundamentalFact, error) {
	return f.FactsWithPeriodFiledBy(ctx, s, subj, c)
}

func (f fakeCompanyFacts) FactsWithPeriodFiledBy(_ context.Context, _, subj string, _ contracts.Day) ([]contracts.FundamentalFact, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	*f.subjects = append(*f.subjects, subj)
	return nil, nil
}

// companySiteReaders is a fake reader set with one stock row; subjects collects the fact reads.
func companySiteReaders(subjects *[]string) siteReaders {
	rows := []underlying.Row{{Key: "US:AAPL", Symbol: "AAPL", Market: "US", Kind: underlying.KindStock,
		Status: underlying.StatusActive, Issuer: "cik:0000320193", Name: "Apple"}}
	facts := fakeCompanyFacts{mu: &sync.Mutex{}, subjects: subjects}
	return siteReaders{
		underlying: fakeCompanyUnderlying{rows: rows}, bars: emptyCompanyBars{}, earnings: emptyCompanyEarnings{},
		ext: map[string]any{siteFactsKey: store.FactReader(facts)},
	}
}

func companySiteGet(t *testing.T, rd siteReaders, method, target string) (int, string) {
	t.Helper()
	a, _ := testApp(t, "")
	h := siteHandler(a, fakeClock{companySiteNow}, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := settingsGet(t, h, method, target)
	return rec.Code, rec.Body.String()
}

func TestCompanyDataRouteStatus(t *testing.T) {
	t.Parallel()
	var subjects []string
	rd := companySiteReaders(&subjects)
	code, body := companySiteGet(t, rd, http.MethodGet, "/company-data")
	if code != http.StatusOK || !strings.Contains(body, `"schema":"company_data_page"`) || !strings.Contains(body, `"key":"US:AAPL"`) {
		t.Fatalf("GET: %d %s", code, body)
	}
	if code, _ := companySiteGet(t, rd, http.MethodHead, "/company-data"); code != http.StatusOK {
		t.Errorf("HEAD: %d", code)
	}
	if code, _ := companySiteGet(t, rd, http.MethodPost, "/company-data"); code != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d", code)
	}
	if code, _ := companySiteGet(t, rd, http.MethodGet, "/company-data/x"); code != http.StatusNotFound {
		t.Errorf("below the route: %d", code)
	}
	// A missing reader is an empty one: a page of gaps, never a 500.
	code, body = companySiteGet(t, siteReaders{}, http.MethodGet, "/company-data")
	if code != http.StatusOK || strings.TrimSpace(body) == "" {
		t.Errorf("build error: %d %q", code, body)
	}
}

func TestCompanyDataSitesOpenNoWriter(t *testing.T) {
	t.Parallel()
	const pages = "postgres://ot_ro_pages@h/db"
	a, _ := testApp(t, "OT_PG_DSN_PAGES="+pages+"\nOT_PG_DSN_FUNDAMENTAL=postgres://ot_w_fundamental@h/db\n")
	var dsns []string
	a.deps.ext = map[string]any{extFundamentals: fundamentalsDeps{openFacts: func(_ context.Context, dsn string) (store.FactStore, error) {
		dsns = append(dsns, dsn)
		return nil, errors.New("fake: not opened")
	}}}
	_, ok := siteOpeners[siteFactsKey](t.Context(), a, func(io.Closer, error) bool { return false })
	if ok {
		t.Fatal("a failed open went on")
	}
	if len(dsns) != 1 || dsns[0] != pages {
		t.Fatalf("fact store opened with %v, want the pages line only", dsns)
	}
}

func TestCompanyPagesDemoProfileOpensNoStore(t *testing.T) {
	t.Parallel()
	for _, name := range []string{companyQuartersName, companyDataName} {
		a, _ := testApp(t, "")
		a.profile.Name = "demo"
		a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
			t.Error("a store was opened")
			return nil, errors.New("opened")
		}
		a.deps.ext = map[string]any{extFundamentals: fundamentalsDeps{openFacts: func(context.Context, string) (store.FactStore, error) {
			t.Error("the fact store was opened")
			return nil, errors.New("opened")
		}}}
		p, closeFn, err := pageNamed(a, name)
		if err != nil {
			t.Fatal(err)
		}
		b, err := p.Build(t.Context(), nil, "2026-11-05", companySiteNow)
		closeFn()
		if err != nil || !strings.Contains(string(b), `"schema":"`+name+`"`) {
			t.Fatalf("%s: %v %s", name, err, b)
		}
	}
}

// idsOfSymbols answers StockIDs with one fixed instrument per symbol.
type idsOfSymbols map[string]int64

func (m idsOfSymbols) StockIDs(_ context.Context, names []string) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, n := range names {
		if id, ok := m[n]; ok {
			out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: n})
		}
	}
	return out, nil
}

// barsUnderID has bars only under one instrument id.
type barsUnderID struct {
	id   int64
	bars []contracts.PriceBar
}

func (b barsUnderID) PriceBars(_ context.Context, id int64, _ string, _, _ contracts.Day) ([]contracts.PriceBar, error) {
	if id != b.id {
		return nil, nil
	}
	return b.bars, nil
}

func TestCompanyDataPassesStockIDs(t *testing.T) {
	t.Parallel()
	var subjects []string
	rd := companySiteReaders(&subjects)
	d := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	rd.bars = barsUnderID{id: 77, bars: []contracts.PriceBar{{SessionDay: types.SerializableDate{Time: d}}}}
	rd.ext[siteMarketStateIDsKey] = market_state_page.StockIDReader(idsOfSymbols{"AAPL": 77})
	a, _ := testApp(t, "")
	site, err := buildCompanyData(context.Background(), a, fakeClock{companySiteNow}, "2026-11-05", rd)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(site), `"bar_count":"1"`) {
		t.Fatalf("site page: %s", site)
	}
	// Without the reader the row has no bars.
	delete(rd.ext, siteMarketStateIDsKey)
	site, err = buildCompanyData(context.Background(), a, fakeClock{companySiteNow}, "2026-11-05", rd)
	if err != nil || strings.Contains(string(site), `"bar_count":"1"`) {
		t.Fatalf("no reader: %v %s", err, site)
	}
}
