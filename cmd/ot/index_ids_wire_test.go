package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeIndexIDs struct {
	computefigures.IndexIDStore
	n *atomic.Int32
}

func (f fakeIndexIDs) Close() error { f.n.Add(1); return nil }

// computeStoresDeps is a deps whose pages and readings openers are fakes that dial nothing.
func computeStoresDeps(closed *atomic.Int32) deps {
	return deps{
		openOptionCatalog:  func(context.Context, string) (store.OptionEodCatalogStore, error) { return fakeCat{n: closed}, nil },
		openOptionEod:      func(context.Context, string) (store.OptionEodStore, error) { return fakeEod{n: closed}, nil },
		openOptionTerms:    func(context.Context, string) (store.OptionTermsStore, error) { return fakeTerms{n: closed}, nil },
		openInstrumentName: func(context.Context, string) (store.InstrumentNameStore, error) { return fakeInstName{n: closed}, nil },
		openSeries:         func(context.Context, string) (store.SeriesStore, error) { return fakeSeries{n: closed}, nil },
		openReading:        func(context.Context, string) (store.ReadingStore, error) { return fakeReading{n: closed}, nil },
	}
}

func TestIndexIDsSetWhenOpenerSet(t *testing.T) {
	t.Parallel()
	var closed atomic.Int32
	var gotDSN string
	d := computeStoresDeps(&closed)
	d.openIndexIDs = func(_ context.Context, dsn string) (computefigures.IndexIDStore, error) {
		gotDSN = dsn
		return fakeIndexIDs{n: &closed}, nil
	}
	cfg := runOnceConfig{pagesDSN: "pages-dsn"}
	cs, closeAll, err := openComputeStores(context.Background(), slog.New(slog.DiscardHandler), d, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cs.IndexIDs == nil || gotDSN != "pages-dsn" {
		t.Fatalf("IndexIDs %v, dsn %q", cs.IndexIDs, gotDSN)
	}
	closeAll()
	if closed.Load() != 7 {
		t.Fatalf("closed %d stores, want 7", closed.Load())
	}
}

func TestIndexIDsNilWhenNoOpener(t *testing.T) {
	t.Parallel()
	var closed atomic.Int32
	var buf bytes.Buffer
	cs, closeAll, err := openComputeStores(context.Background(), slog.New(slog.NewTextHandler(&buf, nil)), computeStoresDeps(&closed), runOnceConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	if cs.IndexIDs != nil {
		t.Fatalf("IndexIDs %v, want nil", cs.IndexIDs)
	}
}

func TestIndexIDsNilWithWarningWhenOpenerErrors(t *testing.T) {
	t.Parallel()
	var closed atomic.Int32
	var buf bytes.Buffer
	d := computeStoresDeps(&closed)
	d.openIndexIDs = func(context.Context, string) (computefigures.IndexIDStore, error) {
		return nil, errors.New("dial postgres://user:secret@host failed")
	}
	cs, closeAll, err := openComputeStores(context.Background(), slog.New(slog.NewTextHandler(&buf, nil)), d, runOnceConfig{}, nil)
	if err != nil {
		t.Fatalf("openComputeStores failed: %v", err)
	}
	defer closeAll()
	if cs.IndexIDs != nil {
		t.Fatalf("IndexIDs %v, want nil", cs.IndexIDs)
	}
	out := buf.String()
	if !strings.Contains(out, "market.rv20") || !strings.Contains(out, "level=WARN") || strings.Contains(out, "secret") {
		t.Fatalf("log %q", out)
	}
}
