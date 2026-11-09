package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/alertrules"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeView is a ReadingViewReader that answers rows (or err) for every read and counts Close calls and the
// DSN-free reads.
type fakeView struct {
	mu     sync.Mutex
	rows   []contracts.ReadingRecord
	err    error
	reads  int
	closed atomic.Int32
}

func (f *fakeView) set(rows ...contracts.ReadingRecord) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = rows
}

func (f *fakeView) WinnerReadings(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	return f.rows, f.err
}

func (f *fakeView) Close() error { f.closed.Add(1); return nil }

// viewRow is an ok decimal reading of day; dd "" leaves the parameter data_day out.
func viewRow(day, value, dd string) contracts.ReadingRecord {
	d, _ := time.Parse(dayLayout, day)
	r := contracts.ReadingRecord{Day: types.SerializableDate{Time: d}, State: "ok", Value: &value, Unit: "decimal", Rev: 1}
	if dd != "" {
		b, _ := json.Marshal([]map[string]any{{"param": "data_day", "value_num": nil, "value_tag": dd}})
		_ = json.Unmarshal(b, &r.Parameters)
	}
	return r
}

// oneRule is a rule set of one rule that fires on a value of at least 0.5.
func oneRule() (compute.AlertRules, error) {
	return compute.AlertRules{Rules: []compute.AlertRule{{
		ID: "r1", Title: "T", Figure: "market.vix_pctile_3y", FigureVersion: 1, SubjectKind: "market",
		Subject: "VIXCLS", Unit: "decimal", Op: "ge", Threshold: "0.5", Level: "w", MaxDataAgeDays: 6,
	}}}, nil
}

// reportApp is an app with the real report page, risks block and evaluation over oneRule.
func reportApp(t *testing.T, env string) *app {
	t.Helper()
	a, _ := testApp(t, env)
	a.profile.Zone = time.FixedZone("CST", 8*3600)
	a.deps.reportPage = newReportPage
	a.deps.riskBlock = newRiskBlock
	a.deps.alertRules = oneRule
	a.deps.evaluateAlerts = alertrules.Evaluate
	return a
}

func reportRisksState(t *testing.T, b []byte) (string, int) {
	t.Helper()
	var p struct {
		Blocks struct {
			Risks struct {
				State string `json:"state"`
				Items any    `json:"items"`
			} `json:"risks"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("%v: %q", err, b)
	}
	items, _ := p.Blocks.Risks.Items.([]any)
	return p.Blocks.Risks.State, len(items)
}

func TestReportOpensPagesRoleOnly(t *testing.T) {
	t.Parallel()
	const env = "OT_PG_DSN_PAGES=pages-dsn\nOT_PG_DSN_READINGS=readings-dsn\nOT_PG_DSN_RUNNER=runner-dsn\n"
	t.Run("ot page", func(t *testing.T) {
		t.Parallel()
		a := reportApp(t, env)
		a.profile.Name = "real"
		var out bytes.Buffer
		a.stdout = &out
		v := &fakeView{}
		var dsns []string
		a.deps.openReadingView = func(_ context.Context, dsn string) (store.ReadingViewReader, error) {
			dsns = append(dsns, dsn)
			return v, nil
		}
		if err := runPage(a, []string{"--name", "report_page", "--market", "us", "--slot", "open", "--day", "2026-11-06"}); err != nil {
			t.Fatal(err)
		}
		if len(dsns) != 1 || dsns[0] != "pages-dsn" || v.closed.Load() != 1 {
			t.Fatalf("dsns %v closed %d", dsns, v.closed.Load())
		}
	})
	t.Run("site", func(t *testing.T) {
		t.Parallel()
		a := reportApp(t, env)
		a.profile.Name = "real"
		v := &fakeView{}
		var dsns []string
		var other atomic.Int32
		a.deps.openReadingView = func(_ context.Context, dsn string) (store.ReadingViewReader, error) {
			dsns = append(dsns, dsn)
			return v, nil
		}
		a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
			return cSeries{closeCounter: closeCounter{&other}}, nil
		}
		a.deps.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) {
			return cEod{closeCounter: closeCounter{&other}}, nil
		}
		a.deps.openQuoteSnapshot = func(context.Context, string) (store.QuoteSnapshotStore, error) {
			return cQuotes{closeCounter: closeCounter{&other}}, nil
		}
		a.deps.openOptionCatalog = func(context.Context, string) (store.OptionEodCatalogStore, error) {
			return cCatalog{closeCounter: closeCounter{&other}}, nil
		}
		a.deps.openOptionTerms = func(context.Context, string) (store.OptionTermsStore, error) {
			return cTerms{closeCounter: closeCounter{&other}}, nil
		}
		a.deps.openInstrumentName = func(context.Context, string) (store.InstrumentNameStore, error) {
			return cNames{closeCounter: closeCounter{&other}}, nil
		}
		rd, closeFn, err := openSiteReaders(t.Context(), a)
		if err != nil {
			t.Fatal(err)
		}
		clk := fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
		h := siteHandler(a, clk, "", rd, echoHandler, http.NotFoundHandler())
		for range 3 {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/report?market=jp&slot=close", nil))
			if rec.Code != 200 {
				t.Fatalf("code %d", rec.Code)
			}
		}
		closeFn()
		if len(dsns) != 1 || dsns[0] != "pages-dsn" || v.closed.Load() != 1 {
			t.Fatalf("dsns %v closed %d", dsns, v.closed.Load())
		}
	})
	t.Run("missing line", func(t *testing.T) {
		t.Parallel()
		a := reportApp(t, "OT_PG_DSN_READINGS=sentinel-secret\n")
		a.profile.Name = "real"
		a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) {
			t.Error("a reader was opened")
			return nil, errors.New("opened")
		}
		err := runPage(a, []string{"--name", "report_page", "--market", "us", "--slot", "open"})
		if err == nil || !strings.Contains(err.Error(), store.DSNEnvPages) || strings.Contains(err.Error(), "sentinel") {
			t.Fatalf("err %v", err)
		}
	})
}

func TestReportRisksAreFilled(t *testing.T) {
	t.Parallel()
	run := func(t *testing.T, dd string) []byte {
		t.Helper()
		a := reportApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
		a.profile.Name = "real"
		var out bytes.Buffer
		a.stdout = &out
		v := &fakeView{}
		v.set(viewRow("2026-11-06", "0.9", dd))
		a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) { return v, nil }
		args := []string{"--name", "report_page", "--market", "us", "--slot", "close", "--day", "2026-11-06"}
		if err := runPage(a, args); err != nil {
			t.Fatal(err)
		}
		if v.reads != 1 {
			t.Fatalf("reads %d", v.reads)
		}
		return out.Bytes()
	}
	filled := run(t, "2026-11-06")
	if st, n := reportRisksState(t, filled); st != "ok" || n != 1 {
		t.Fatalf("state %s items %d", st, n)
	}
	if st, _ := reportRisksState(t, run(t, "")); st != "gap" {
		t.Fatalf("without data_day: state %s", st)
	}
	validateReport(t, filled)
}

func validateReport(t *testing.T, b []byte) {
	t.Helper()
	f, err := os.Open("../../contracts/schemas/page.report_page.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("page.json", doc); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile("page.json")
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Fatalf("schema: %v", err)
	}
}

func TestReportDemoOpensNoStore(t *testing.T) {
	t.Parallel()
	a := reportApp(t, "")
	a.profile.Name = "demo"
	a.deps.alertRules = alertrules.DefaultRules
	a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) {
		t.Error("a store was opened")
		return nil, errors.New("opened")
	}
	var out bytes.Buffer
	a.stdout = &out
	if err := runPage(a, []string{"--name", "report_page", "--market", "us", "--slot", "open", "--day", "2026-11-06"}); err != nil {
		t.Fatal(err)
	}
	if st, _ := reportRisksState(t, out.Bytes()); st != "ok" && st != "partial" {
		t.Fatalf("page state %s", st)
	}
	validateReport(t, out.Bytes())
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	clk := fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
	h := siteHandler(a, clk, "", rd, echoHandler, http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/report?market=us&slot=open", nil))
	if st, _ := reportRisksState(t, rec.Body.Bytes()); rec.Code != 200 || (st != "ok" && st != "partial") {
		t.Fatalf("site: %d state %s", rec.Code, st)
	}
}
