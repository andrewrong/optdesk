package main

import (
	"context"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/optionday"
	"github.com/andrewrong/optdesk/internal/store/snapshot"
)

// recordDays is an optionday.DayReader that counts its Close.
type recordDays struct {
	optionday.DayReader
	closed atomic.Int32
}

func (r *recordDays) Close() error {
	r.closed.Add(1)
	return nil
}

func TestOpenSiteReadersOpensDayReader(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
	a.profile.Name = "real"
	rec := &recordDays{}
	var dsns []string
	a.deps.openOptionDay = func(_ context.Context, dsn string) (optionday.DayReader, error) {
		dsns = append(dsns, dsn)
		return rec, nil
	}
	var other atomic.Int32
	cc := closeCounter{&other}
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) { return cSeries{closeCounter: cc}, nil }
	a.deps.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) { return cEod{closeCounter: cc}, nil }
	a.deps.openQuoteSnapshot = func(context.Context, string) (store.QuoteSnapshotStore, error) {
		return cQuotes{closeCounter: cc}, nil
	}
	a.deps.openOptionCatalog = func(context.Context, string) (store.OptionEodCatalogStore, error) {
		return cCatalog{closeCounter: cc}, nil
	}
	a.deps.openOptionTerms = func(context.Context, string) (store.OptionTermsStore, error) { return cTerms{closeCounter: cc}, nil }
	a.deps.openInstrumentName = func(context.Context, string) (store.InstrumentNameStore, error) {
		return cNames{closeCounter: cc}, nil
	}
	a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) { return cView{closeCounter: cc}, nil }
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	if len(dsns) != 1 || dsns[0] != "pages-dsn" || rd.days != optionday.DayReader(rec) {
		t.Fatalf("dsns %v days %v", dsns, rd.days)
	}
	if rec.closed.Load() != 0 {
		t.Fatal("closed before closeFn")
	}
	closeFn()
	if rec.closed.Load() != 1 {
		t.Fatalf("closed %d", rec.closed.Load())
	}

	demo, _ := testApp(t, "")
	demo.profile.Name = "demo"
	demo.deps.openOptionDay = func(context.Context, string) (optionday.DayReader, error) {
		t.Error("opened with the demo profile")
		return rec, nil
	}
	drd, dclose, err := openSiteReaders(t.Context(), demo)
	if err != nil {
		t.Fatal(err)
	}
	defer dclose()
	if drd.days != nil {
		t.Fatalf("demo days %v", drd.days)
	}
}

func TestDefaultOptionDayOpenerIsConid(t *testing.T) {
	t.Parallel()
	got := reflect.ValueOf(newBaseDeps().openOptionDay).Pointer()
	want := reflect.ValueOf(snapshot.OpenConidDayReader).Pointer()
	if got != want {
		t.Fatal("openOptionDay is not snapshot.OpenConidDayReader")
	}
}
