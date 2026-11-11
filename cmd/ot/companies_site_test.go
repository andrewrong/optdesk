package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var companiesClock = fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}

const companiesWatch = `
[[underlying]]
issuer = "cik:0000320193"
market = "US"
symbol = "AAA"
sec_type = "STK"
exchange = "SMART"
currency = "USD"
buy_price = "85.5"

[[underlying]]
issuer = "cik:0000789019"
market = "US"
symbol = "BBB"
sec_type = "STK"
exchange = "SMART"
currency = "USD"
`

type noView struct{ store.ReadingViewReader }

func companiesApp(t *testing.T, watch string, readErr error) *app {
	t.Helper()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	a.deps.readFile = func(string) ([]byte, error) {
		if readErr != nil {
			return nil, readErr
		}
		return []byte(watch), nil
	}
	return a
}

func companiesGet(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, target, nil))
	return rec
}

func companiesRows(t *testing.T, body []byte) []struct {
	Cells []struct {
		Text      string `json:"text"`
		HandPrice string `json:"hand_price"`
	} `json:"cells"`
} {
	t.Helper()
	var vm struct {
		WatchList struct {
			Rows []struct {
				Cells []struct {
					Text      string `json:"text"`
					HandPrice string `json:"hand_price"`
				} `json:"cells"`
			} `json:"rows"`
		} `json:"watch_list"`
	}
	if err := json.Unmarshal(body, &vm); err != nil {
		t.Fatal(err)
	}
	return vm.WatchList.Rows
}

func TestCompaniesRouteServes(t *testing.T) {
	t.Parallel()
	a := companiesApp(t, companiesWatch, nil)
	h := siteHandler(a, companiesClock, "2026-11-05", siteReaders{view: noView{}}, echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteCompaniesRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"hand_price":"85.5"`) || !strings.Contains(rec.Body.String(), `"no_reader"`) {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	if rec := companiesGet(t, h, http.MethodHead, "/"+siteCompaniesRoute); rec.Code != 200 {
		t.Errorf("HEAD: %d", rec.Code)
	}
	if rec := companiesGet(t, h, http.MethodGet, "/"+siteCompaniesRoute+"/x"); rec.Code != 404 {
		t.Errorf("sub path: %d", rec.Code)
	}
	if rec := companiesGet(t, h, http.MethodPost, "/"+siteCompaniesRoute); rec.Code != 405 {
		t.Errorf("POST: %d", rec.Code)
	}
}

func TestHandPriceBesideGap(t *testing.T) {
	t.Parallel()
	a := companiesApp(t, companiesWatch, nil)
	body, err := buildCompanies(t.Context(), a, companiesClock, "2026-11-05", siteReaders{view: noView{}})
	if err != nil {
		t.Fatal(err)
	}
	rows := companiesRows(t, body)
	if len(rows) != 2 {
		t.Fatalf("rows %d", len(rows))
	}
	for i, want := range []string{"85.5", ""} {
		c := rows[i].Cells[1]
		if c.HandPrice != want || !strings.HasPrefix(c.Text, "no band") {
			t.Errorf("row %d: %+v", i, c)
		}
	}
	// a missing file gives no rows; the demo profile gives its two rows with a hand price each.
	a = companiesApp(t, "", fs.ErrNotExist)
	body, err = buildCompanies(t.Context(), a, companiesClock, "", siteReaders{})
	if err != nil || len(companiesRows(t, body)) != 0 {
		t.Fatalf("missing file: %v %s", err, body)
	}
	a.profile.Name = "demo"
	body, err = buildCompanies(t.Context(), a, companiesClock, "", siteReaders{})
	rows = companiesRows(t, body)
	if err != nil || len(rows) != 2 || rows[0].Cells[1].HandPrice == "" || rows[1].Cells[1].HandPrice == "" {
		t.Fatalf("demo: %v %s", err, body)
	}
}

func TestPageNameBuilds(t *testing.T) {
	t.Parallel()
	a := companiesApp(t, companiesWatch, nil)
	want, err := buildCompanies(t.Context(), a, companiesClock, "2026-11-05", siteReaders{})
	if err != nil {
		t.Fatal(err)
	}
	page, closeFn, err := pageNamed(a, companiesName)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	got, err := page.Build(t.Context(), nil, contracts.Day("2026-11-05"), companiesClock.Now().UTC())
	if err != nil || string(got) != string(want) {
		t.Fatalf("%v\n got %s\nwant %s", err, got, want)
	}
}

func TestBrokenWatchRefuses(t *testing.T) {
	t.Parallel()
	a := companiesApp(t, "[[underlying]]\nissuer = \"bad\"\n", nil)
	if _, err := buildCompanies(t.Context(), a, companiesClock, "", siteReaders{}); err == nil {
		t.Fatal("broken watch.toml built")
	}
	h := siteHandler(a, companiesClock, "", siteReaders{}, echoHandler, http.NotFoundHandler())
	if rec := companiesGet(t, h, http.MethodGet, "/"+siteCompaniesRoute); rec.Code != 500 {
		t.Errorf("code %d", rec.Code)
	}
	a = companiesApp(t, "", errors.New("denied"))
	if _, err := buildCompanies(t.Context(), a, companiesClock, "", siteReaders{}); err == nil {
		t.Fatal("unreadable file built")
	}
}

// heldAccount is an account reader whose position rows are keyed by day.
type heldAccount struct {
	store.AccountReader
	byDay map[string][]store.Position
	err   error
}

func (h heldAccount) Positions(_ context.Context, _, day string) ([]store.Position, error) {
	return h.byDay[day], h.err
}

type heldPage struct {
	Header struct {
		HeldCount *int `json:"held_count"`
	} `json:"header"`
	Holdings *struct {
		Rows []struct {
			Cells []struct {
				Text string `json:"text"`
			} `json:"cells"`
		} `json:"rows"`
	} `json:"holdings"`
	WatchList struct {
		Rows []json.RawMessage `json:"rows"`
	} `json:"watch_list"`
}

func buildHeld(t *testing.T, acc store.AccountReader) (heldPage, string) {
	t.Helper()
	a := companiesApp(t, companiesWatch, nil)
	body, err := buildCompanies(t.Context(), a, companiesClock, "2026-11-05", siteReaders{noticeAccount: acc})
	if err != nil {
		t.Fatal(err)
	}
	var p heldPage
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatal(err)
	}
	return p, string(body)
}

func stk(sym, sec, qty string) store.Position {
	return store.Position{Symbol: sym, SecType: sec, Quantity: qty}
}

func TestCompaniesHeldFromPositions(t *testing.T) {
	t.Parallel()
	for name, day := range map[string]string{"page day": "2026-11-05", "day before": "2026-11-04", "six days before": "2026-10-30"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p, body := buildHeld(t, heldAccount{byDay: map[string][]store.Position{day: {stk("AAA", "STK", "100")}}})
			if p.Header.HeldCount == nil || *p.Header.HeldCount != 1 || p.Holdings == nil || len(p.Holdings.Rows) != 1 {
				t.Fatalf("held: %s", body)
			}
			if !strings.Contains(body, `"AAA"`) || len(p.WatchList.Rows) != 1 || strings.Contains(string(p.WatchList.Rows[0]), `"AAA"`) {
				t.Fatalf("watch list: %s", body)
			}
		})
	}
}

func TestCompaniesHeldIgnoresOptionsAndZero(t *testing.T) {
	t.Parallel()
	p, body := buildHeld(t, heldAccount{byDay: map[string][]store.Position{"2026-11-05": {stk("AAA", "OPT", "-1"), stk("BBB", "STK", "0")}}})
	if p.Header.HeldCount == nil || *p.Header.HeldCount != 0 || len(p.WatchList.Rows) != 2 {
		t.Fatalf("%s", body)
	}
}

func TestCompaniesHeldNilWithoutReader(t *testing.T) {
	t.Parallel()
	a := companiesApp(t, companiesWatch, nil)
	body, err := buildCompanies(t.Context(), a, companiesClock, "2026-11-05", siteReaders{})
	if err != nil {
		t.Fatal(err)
	}
	var p heldPage
	if err := json.Unmarshal(body, &p); err != nil || p.Header.HeldCount != nil || p.Holdings != nil {
		t.Fatalf("%v %s", err, body)
	}
}

func TestCompaniesHeldReaderErrorIsReturned(t *testing.T) {
	t.Parallel()
	a := companiesApp(t, companiesWatch, nil)
	_, err := buildCompanies(t.Context(), a, companiesClock, "2026-11-05", siteReaders{noticeAccount: heldAccount{err: errors.New("boom")}})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("%v", err)
	}
}
