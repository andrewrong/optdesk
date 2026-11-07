package main

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
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
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
	var out bytes.Buffer
	a.stdout = &out
	var call pageCall
	pageApp(a, &call)
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		t.Error("a store was opened")
		return nil, nil
	}
	_, want := pageNamed(a, "nosuch")
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
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
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
	a.deps.macroPage = func(z *time.Location, b string) pages.Page {
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
	a.deps.macroPage = func(*time.Location, string) pages.Page {
		t.Error("macroPage was called")
		return pages.Page{}
	}
	if err := runPage(a, []string{"--name", "macro_rates"}); err != nil || out.String() != "RATES" {
		t.Fatalf("out %q err %v", out.String(), err)
	}
}

func TestPageMacroPageNilZoneOpensNoStore(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
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
