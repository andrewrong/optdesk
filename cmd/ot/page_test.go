package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/report_page"
	"github.com/andrewrong/optdesk/internal/store"
)

type pageCall struct {
	r   store.SeriesReader
	day contracts.Day
	at  time.Time
}

func pageApp(a *app, call *pageCall) {
	a.deps.pageList = []pages.Page{{
		Meta: pages.Meta{Name: "macro_rates"},
		Build: func(_ context.Context, r store.SeriesReader, d contracts.Day, at time.Time) ([]byte, error) {
			*call = pageCall{r, d, at}
			return []byte(`{"page":"x"}`), nil
		},
	}}
}

func fakePage(name, body string) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: name},
		Build: func(context.Context, store.SeriesReader, contracts.Day, time.Time) ([]byte, error) {
			return []byte(body), nil
		},
	}
}

func TestFindPage(t *testing.T) {
	t.Parallel()
	list := []pages.Page{fakePage("alpha", "a"), fakePage("beta", "b")}
	for _, n := range []string{"alpha", "beta"} {
		p, err := findPage(list, n)
		if err != nil || p.Meta.Name != n {
			t.Fatalf("%s: got %q err %v", n, p.Meta.Name, err)
		}
	}
	_, err := findPage(list, "nosuch")
	if err == nil || !strings.Contains(err.Error(), "nosuch") || !strings.Contains(err.Error(), "alpha, beta") {
		t.Fatalf("err %v", err)
	}
}

func TestPageUnknownNameOpensNoStore(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, store.DSNEnvPages+"=pages-dsn\n")
	var out bytes.Buffer
	a.stdout = &out
	var call pageCall
	pageApp(a, &call)
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		t.Error("a store was opened")
		return nil, nil
	}
	_, _, want := pageNamed(a, "nosuch")
	err := runPage(a, []string{"--name", "nosuch"})
	if err == nil || err.Error() != want.Error() || out.Len() != 0 {
		t.Fatalf("err %v want %v out %q", err, want, out.String())
	}
}

func TestPageNameSelectsEntry(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	var out bytes.Buffer
	a.stdout = &out
	var call pageCall
	pageApp(a, &call)
	a.deps.pageList = append(a.deps.pageList, fakePage("second", "SECOND"))
	if err := runPage(a, []string{"--name", "second"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "SECOND" || call.day != "" {
		t.Fatalf("out %q call %+v", out.String(), call)
	}
}

func TestPageBytesAreBuilderBytes(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, store.DSNEnvPages+"=pages-dsn\n")
	var out bytes.Buffer
	a.stdout = &out
	var call pageCall
	pageApp(a, &call)
	var dsn string
	var closed atomic.Int32
	a.deps.openSeries = func(_ context.Context, d string) (store.SeriesStore, error) {
		dsn = d
		return fakeSeries{n: &closed}, nil
	}
	if err := runPage(a, []string{"--day", "2026-02-03"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != `{"page":"x"}` || dsn != "pages-dsn" || call.day != "2026-02-03" || closed.Load() != 1 {
		t.Fatalf("out %q dsn %q call %+v closed %d", out.String(), dsn, call, closed.Load())
	}
	if want := time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC); !call.at.Equal(want) || call.at.Location() != time.UTC {
		t.Fatalf("generatedAt %v", call.at)
	}
	p := filepath.Join(t.TempDir(), "p.json")
	if err := runPage(a, []string{"--out", p}); err != nil {
		t.Fatal(err)
	}
	b, err := readFile(p)
	if err != nil || string(b) != `{"page":"x"}` {
		t.Fatalf("file %q err %v", b, err)
	}
}

func TestPageDemoOpensNoStore(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	var out bytes.Buffer
	a.stdout = &out
	var call pageCall
	pageApp(a, &call)
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		t.Error("a store was opened")
		return nil, nil
	}
	if err := runPage(a, nil); err != nil {
		t.Fatal(err)
	}
	pts, err := call.r.SeriesPoints(t.Context(), "DGS10", "2026-01-01", "2026-12-31")
	if err != nil || len(pts) != 3 || pts[1].Value != nil || pts[0].Value == nil {
		t.Fatalf("demo points %v err %v", pts, err)
	}
}

func TestPageMacroPage(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	var out bytes.Buffer
	a.stdout = &out
	var gotZone *time.Location
	var gotBuild string
	a.deps.pageList = []pages.Page{fakePage("macro_rates", "RATES")}
	a.deps.macroPage = func(z *time.Location, b string, _ compute.ReadingFunc) pages.Page {
		gotZone, gotBuild = z, b
		return fakePage("macro_page", "MACRO")
	}
	if err := runPage(a, []string{"--name", "macro_page"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "MACRO" || gotZone != a.profile.Zone || gotZone == nil || gotBuild != buildIDOf(version) {
		t.Fatalf("out %q zone %v build %q", out.String(), gotZone, gotBuild)
	}
}

func TestPageMacroRatesIgnoresZone(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	a.profile.Zone = nil
	var out bytes.Buffer
	a.stdout = &out
	a.deps.pageList = []pages.Page{fakePage("macro_rates", "RATES")}
	a.deps.macroPage = func(*time.Location, string, compute.ReadingFunc) pages.Page {
		t.Error("macroPage was called")
		return pages.Page{}
	}
	if err := runPage(a, []string{"--name", "macro_rates"}); err != nil || out.String() != "RATES" {
		t.Fatalf("out %q err %v", out.String(), err)
	}
}

func TestPageMacroPageNilZoneOpensNoStore(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, store.DSNEnvPages+"=pages-dsn\n")
	a.profile.Zone = nil
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		t.Error("a store was opened")
		return nil, nil
	}
	err := runPage(a, []string{"--name", "macro_page"})
	if err == nil || !strings.Contains(err.Error(), "OT_ZONE") {
		t.Fatalf("err %v", err)
	}
}

var update = flag.Bool("update", false, "rewrite the golden files")

type goldenClock struct{ now time.Time }

func (c goldenClock) Now() time.Time { return c.now }

func TestMacroPageDemoGolden(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	a, err := newApp(func(string) string { return "" }, &out, &errb)
	if err != nil {
		t.Fatal(err)
	}
	if a.profile.Zone != time.UTC {
		t.Fatalf("zone %v", a.profile.Zone)
	}
	a.deps = deps{
		newClock:  func() clock.Clock { return goldenClock{time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)} },
		macroPage: newDeps().macroPage,
	}
	if err := runPage(a, []string{"--name", "macro_page", "--day", "2026-11-05"}); err != nil {
		t.Fatal(err)
	}
	const path = "testdata/macro_page_demo.golden.json"
	if *update {
		if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("output differs from %s:\n%s", path, out.String())
	}
}

func TestPageReportFlags(t *testing.T) {
	t.Parallel()
	newApp := func(t *testing.T) (*app, *bytes.Buffer, *[]string) {
		t.Helper()
		a, _ := testApp(t, "")
		a.profile.Name = "demo"
		var out bytes.Buffer
		a.stdout = &out
		var got []string
		a.deps.pageList = []pages.Page{fakePage("macro_rates", "RATES")}
		a.deps.macroPage = func(*time.Location, string, compute.ReadingFunc) pages.Page { return fakePage("macro_page", "MACRO") }
		a.deps.reportPage = func(c report_page.Config) pages.Page {
			got = append(got, c.Market, c.Slot)
			return fakePage("report_page", "REPORT")
		}
		return a, &out, &got
	}
	a, out, got := newApp(t)
	if err := runPage(a, []string{"--name", "report_page", "--market", "us", "--slot", "open"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "REPORT" || strings.Join(*got, ",") != "US,open" {
		t.Fatalf("out %q got %v", out.String(), *got)
	}
	for _, c := range []struct {
		args []string
		flag string
	}{
		{[]string{"--slot", "open"}, "--market"},
		{[]string{"--market", "us"}, "--slot"},
		{[]string{"--market", "xx", "--slot", "open"}, "--market"},
		{[]string{"--market", "us", "--slot", "noon"}, "--slot"},
	} {
		a, out, _ := newApp(t)
		err := runPage(a, append([]string{"--name", "report_page"}, c.args...))
		if err == nil || !strings.Contains(err.Error(), c.flag) || out.Len() != 0 {
			t.Errorf("%v: err %v out %q", c.args, err, out)
		}
	}
	for name, want := range map[string]string{"macro_rates": "RATES", "macro_page": "MACRO"} {
		a, out, _ := newApp(t)
		if err := runPage(a, []string{"--name", name}); err != nil || out.String() != want {
			t.Errorf("%s: %q %v", name, out, err)
		}
	}
}

// macroViewApp is an app with the profile of testApp, a macroPage fake that hands its read to got, and an
// openReadingView fake that records the DSN it got and returns view.
func macroViewApp(t *testing.T, env string, view *fakeView, gotDSN *[]string, got *compute.ReadingFunc) *app {
	t.Helper()
	a, _ := testApp(t, env)
	a.deps.macroPage = func(_ *time.Location, _ string, read compute.ReadingFunc) pages.Page {
		*got = read
		return fakePage("macro_page", "MACRO")
	}
	var seriesClosed atomic.Int32
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		return fakeSeries{n: &seriesClosed}, nil
	}
	a.deps.openReadingView = func(_ context.Context, dsn string) (store.ReadingViewReader, error) {
		*gotDSN = append(*gotDSN, dsn)
		return view, nil
	}
	return a
}

func TestMacroPageGetsReadingFuncWhenDSNSet(t *testing.T) {
	t.Parallel()
	view := &fakeView{}
	view.set(viewRow("2026-11-05", "55.5", ""))
	var dsns []string
	var read compute.ReadingFunc
	a := macroViewApp(t, store.DSNEnvPages+"=pages-dsn\n", view, &dsns, &read)
	var out bytes.Buffer
	a.stdout = &out
	if err := runPage(a, []string{"--name", "macro_page"}); err != nil {
		t.Fatal(err)
	}
	if read == nil {
		t.Fatal("macroPage got a nil read")
	}
	rows, err := read(t.Context(), "macro.ease", "market", "WALCL", "2026-11-05", "2026-11-05")
	if err != nil || len(rows) != 1 || view.reads != 1 {
		t.Fatalf("rows %v err %v reads %d", rows, err, view.reads)
	}
	if view.closed.Load() != 1 || len(dsns) != 1 || dsns[0] != "pages-dsn" || out.String() != "MACRO" {
		t.Fatalf("closed %d dsns %v out %q", view.closed.Load(), dsns, out.String())
	}
}

func TestMacroPageNilReadWhenDSNUnset(t *testing.T) {
	t.Parallel()
	for name, env := range map[string]string{"absent": "OT_OTHER=x\n", "empty": store.DSNEnvPages + "=\n"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var dsns []string
			read := func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
				return nil, nil
			}
			var got compute.ReadingFunc = read
			a := macroViewApp(t, env, &fakeView{}, &dsns, &got)
			if _, closeFn, err := pageNamed(a, "macro_page"); err != nil {
				t.Fatal(err)
			} else {
				closeFn()
			}
			if got != nil || len(dsns) != 0 {
				t.Fatalf("read non-nil %t, opener calls %v", got != nil, dsns)
			}
		})
	}
}

func TestMacroPageDemoOpensNoView(t *testing.T) {
	t.Parallel()
	var dsns []string
	var got compute.ReadingFunc = func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
		return nil, nil
	}
	a := macroViewApp(t, store.DSNEnvPages+"=pages-dsn\n", &fakeView{}, &dsns, &got)
	a.profile.Name = "demo"
	if err := runPage(a, []string{"--name", "macro_page"}); err != nil {
		t.Fatal(err)
	}
	if got != nil || len(dsns) != 0 {
		t.Fatalf("read non-nil %t, opener calls %v", got != nil, dsns)
	}
}

func TestMacroPageReaderOpenErrorIsReported(t *testing.T) {
	t.Parallel()
	const secret = "secret-pages-dsn"
	a, _ := testApp(t, store.DSNEnvPages+"="+secret+"\n")
	a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) {
		return nil, errors.New("connect " + secret + " refused")
	}
	a.deps.macroPage = func(*time.Location, string, compute.ReadingFunc) pages.Page {
		t.Error("macroPage was called")
		return pages.Page{}
	}
	err := runPage(a, []string{"--name", "macro_page"})
	if err == nil || !strings.Contains(err.Error(), store.DSNEnvPages) || strings.Contains(err.Error(), secret) {
		t.Fatalf("err %v", err)
	}
}

func TestPageMarketFlags(t *testing.T) {
	t.Parallel()
	newApp := func(t *testing.T) (*app, *bytes.Buffer) {
		t.Helper()
		a, _ := testApp(t, "")
		a.profile.Name = "demo"
		var out bytes.Buffer
		a.stdout = &out
		a.deps.pageList = []pages.Page{fakePage("macro_rates", "RATES")}
		a.deps.macroPage = func(*time.Location, string, compute.ReadingFunc) pages.Page { return fakePage("macro_page", "MACRO") }
		a.deps.reportPage = func(report_page.Config) pages.Page {
			return fakePage("report_page", "REPORT")
		}
		a.deps.marketPage = func(*time.Location, string, string, string, compute.ReadingFunc) pages.Page {
			return fakePage("market_page", "MARKET")
		}
		return a, &out
	}
	a, out := newApp(t)
	if err := runPage(a, []string{"--name", "market_page"}); err != nil || out.String() != "MARKET" {
		t.Fatalf("out %q err %v", out, err)
	}
	a, out = newApp(t)
	if err := runPage(a, []string{"--name", "market_page", "--day", "x"}); err == nil || out.Len() != 0 {
		t.Fatalf("bad day: err %v out %q", err, out)
	}
	for name, want := range map[string]string{"macro_rates": "RATES", "macro_page": "MACRO"} {
		a, out := newApp(t)
		if err := runPage(a, []string{"--name", name}); err != nil || out.String() != want {
			t.Errorf("%s: %q %v", name, out, err)
		}
	}
	a, out = newApp(t)
	if err := runPage(a, []string{"--name", "report_page", "--market", "us", "--slot", "open"}); err != nil || out.String() != "REPORT" {
		t.Errorf("report_page: %q %v", out, err)
	}
}
