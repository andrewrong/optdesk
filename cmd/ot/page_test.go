package main

import (
	"bytes"
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type pageCall struct {
	r   store.SeriesReader
	day contracts.Day
	at  time.Time
}

func pageApp(a *app, call *pageCall) {
	a.deps.buildMacroRates = func(_ context.Context, r store.SeriesReader, d contracts.Day, at time.Time) ([]byte, error) {
		*call = pageCall{r, d, at}
		return []byte(`{"page":"x"}`), nil
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
