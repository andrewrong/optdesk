package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

// cSessions is a session-day store whose Close is counted.
type cSessions struct {
	store.SessionDayStore
	closeCounter
}

func (c cSessions) Close() error { return c.closeCounter.Close() }

// sessionApp is a real-profile app whose seven mandatory stores open (and count their closes) and whose
// session-day opener is left nil.
func sessionApp(t *testing.T, closed *atomic.Int32) *app {
	t.Helper()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
	a.profile.Name = "real"
	cc := closeCounter{closed}
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
	return a
}

func TestSiteOpensSessionDay(t *testing.T) {
	t.Parallel()
	var closed atomic.Int32
	a := sessionApp(t, &closed)
	var dsn string
	sd := cSessions{closeCounter: closeCounter{&closed}}
	a.deps.openSessionDay = func(_ context.Context, d string) (store.SessionDayStore, error) {
		dsn = d
		return sd, nil
	}
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	if dsn != "pages-dsn" {
		t.Fatalf("dsn %q", dsn)
	}
	if rd.sessions != store.SessionDayReader(sd) {
		t.Fatalf("sessions %v", rd.sessions)
	}
	closeFn()
	if closed.Load() != 8 {
		t.Fatalf("closed %d want 8", closed.Load())
	}
}

func TestSiteSessionOpenFails(t *testing.T) {
	t.Parallel()
	var closed atomic.Int32
	a := sessionApp(t, &closed)
	a.deps.openSessionDay = func(context.Context, string) (store.SessionDayStore, error) {
		return nil, errors.New("open sentinel-secret")
	}
	_, _, err := openSiteReaders(t.Context(), a)
	if err == nil || err.Error() != "open stores with OT_PG_DSN_PAGES failed" || strings.Contains(err.Error(), "sentinel") {
		t.Fatalf("err %v", err)
	}
	if closed.Load() != 7 {
		t.Fatalf("closed %d want 7", closed.Load())
	}
}

func TestSiteSessionNilOpener(t *testing.T) {
	t.Parallel()
	var closed atomic.Int32
	a := sessionApp(t, &closed)
	rd, _, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	if rd.sessions != nil {
		t.Fatalf("sessions %v", rd.sessions)
	}
}
