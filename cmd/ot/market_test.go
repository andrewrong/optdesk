package main

import (
	"bytes"
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// marketApp is an app whose marketPage fake prints what its readings argument gives for the figure of fig.
func marketApp(t *testing.T, env string, fig string, view *fakeView, dsns *[]string, seriesClosed *atomic.Int32) (*app, *bytes.Buffer) {
	t.Helper()
	a, _ := testApp(t, env)
	var out bytes.Buffer
	a.stdout = &out
	a.deps.marketPage = func(_ *time.Location, _, _, _ string, readings compute.ReadingFunc) pages.Page {
		build := func(ctx context.Context, _ store.SeriesReader, day contracts.Day, _ time.Time) ([]byte, error) {
			rows, err := readings(ctx, fig, "market", "VIXCLS", day, day)
			if err != nil {
				return nil, err
			}
			var b strings.Builder
			for _, r := range rows {
				b.WriteString(*r.Value + " ")
				for _, p := range r.Parameters {
					if p.Param == "data_day" && p.ValueTag != nil {
						b.WriteString(*p.ValueTag)
					}
				}
			}
			return []byte(b.String()), nil
		}
		return pages.Page{Meta: pages.Meta{Name: "market_page"}, Build: build}
	}
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		return fakeSeries{n: seriesClosed}, nil
	}
	a.deps.openReadingView = func(_ context.Context, dsn string) (store.ReadingViewReader, error) {
		*dsns = append(*dsns, dsn)
		return view, nil
	}
	return a, &out
}

func TestMarketOpensPagesRoleOnly(t *testing.T) {
	t.Parallel()
	const env = store.DSNEnvPages + "=pages-dsn\nOT_PG_DSN_READINGS=readings-dsn\nOT_PG_DSN_LOADER=loader-dsn\n"
	view := &fakeView{}
	var dsns []string
	var sc atomic.Int32
	a, _ := marketApp(t, env, "market.vix_pctile_3y", view, &dsns, &sc)
	var seriesDSN []string
	open := a.deps.openSeries
	a.deps.openSeries = func(ctx context.Context, d string) (store.SeriesStore, error) {
		seriesDSN = append(seriesDSN, d)
		return open(ctx, d)
	}
	if err := runPage(a, []string{"--name", "market_page", "--day", "2026-11-05"}); err != nil {
		t.Fatal(err)
	}
	if len(dsns) != 1 || dsns[0] != "pages-dsn" || len(seriesDSN) != 1 || seriesDSN[0] != "pages-dsn" {
		t.Fatalf("view dsns %v series dsns %v", dsns, seriesDSN)
	}
	if view.closed.Load() != 1 || sc.Load() != 1 {
		t.Fatalf("closed view %d series %d", view.closed.Load(), sc.Load())
	}

	const secret = "secret-dsn-text"
	a, _ = marketApp(t, "OT_PG_DSN_READINGS="+secret+"\n", "x", &fakeView{}, &dsns, &sc)
	a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) {
		t.Error("view opened without the pages line")
		return nil, nil
	}
	err := runPage(a, []string{"--name", "market_page"})
	if err == nil || !strings.Contains(err.Error(), store.DSNEnvPages) || strings.Contains(err.Error(), secret) {
		t.Fatalf("err %v", err)
	}
}

func TestMarketRankFilled(t *testing.T) {
	t.Parallel()
	view := &fakeView{}
	view.set(viewRow("2026-11-05", "0.685000", "2026-11-05"))
	var dsns []string
	var sc atomic.Int32
	a, out := marketApp(t, store.DSNEnvPages+"=pages-dsn\n", "market.vix_pctile_3y", view, &dsns, &sc)
	if err := runPage(a, []string{"--name", "market_page", "--day", "2026-11-05"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "0.685000 2026-11-05" || view.reads != 1 {
		t.Fatalf("out %q reads %d", out.String(), view.reads)
	}

	a, _ = marketApp(t, store.DSNEnvPages+"=pages-dsn\n", "x", view, &dsns, &sc)
	a.profile.Zone = nil
	a.deps.marketPage = func(*time.Location, string, string, string, compute.ReadingFunc) pages.Page {
		t.Error("marketPage called with a nil zone")
		return pages.Page{}
	}
	a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) {
		t.Error("view opened with a nil zone")
		return nil, nil
	}
	if err := runPage(a, []string{"--name", "market_page"}); err == nil || !strings.Contains(err.Error(), "zone") {
		t.Fatalf("err %v", err)
	}
}

func TestMarketDemoOpensNoStore(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Name = "demo"
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		t.Error("series store opened")
		return nil, nil
	}
	a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) {
		t.Error("view opened")
		return nil, nil
	}
	var got compute.ReadingFunc
	a.deps.marketPage = func(_ *time.Location, _, _, _ string, r compute.ReadingFunc) pages.Page {
		got = r
		return fakePage("market_page", "MARKET")
	}
	if err := runPage(a, []string{"--name", "market_page", "--day", "2026-11-05"}); err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("nil Readings")
	}
	for _, f := range []string{"market.vix_pctile_3y", "market.vxn_pctile_3y", "market.rvx_pctile_3y", "market.sp500_vs_200d", "market.vix_term_ratio"} {
		rows, err := got(t.Context(), f, "market", "VIXCLS", "2026-11-05", "2026-11-05")
		if err != nil || len(rows) != 1 || rows[0].Value == nil {
			t.Fatalf("%s: %v %v", f, rows, err)
		}
		var dd bool
		for _, p := range rows[0].Parameters {
			dd = dd || p.Param == "data_day"
		}
		if !dd {
			t.Errorf("%s: no data_day", f)
		}
	}
}
