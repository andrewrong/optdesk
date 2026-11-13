package main

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

func builtOptionEod(t *testing.T) contracts.Job {
	t.Helper()
	clk := fakeClock{time.Date(2026, 11, 9, 7, 0, 0, 0, time.UTC)}
	watch, err := parseWatch([]byte(jpStockWatch))
	if err != nil {
		t.Fatal(err)
	}
	j, err := buildOptionEodJP(newDeps(), unreachableBroker{}, &memResolver{}, &memSink{}, watch, clk)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

// The writer is handed to the job the constructor built, and the same job comes back. (No run reads the writer
// until the tally of entry 5, so the call count of a run is asserted there.)
func TestWrapOptionEodWithWriter(t *testing.T) {
	t.Parallel()
	built := builtOptionEod(t)
	got, err := withOptionEodReasons(built, nil, &fakeRunStore{})
	if err != nil || got != built {
		t.Fatalf("got %v, %v; want the built job", got, err)
	}
	// A job another package built cannot take the writer: the wiring fails, it does not drop the writer.
	if _, err := withOptionEodReasons(reasonForeignJob{}, nil, &fakeRunStore{}); err == nil {
		t.Fatal("a foreign job took the writer")
	}
	// A failed build keeps its error and is not wrapped.
	boom := errors.New("boom")
	if _, err := withOptionEodReasons(nil, boom, &fakeRunStore{}); !errors.Is(err, boom) {
		t.Fatalf("err %v, want the build error", err)
	}
}

func TestWrapOptionEodNilWriterUnchanged(t *testing.T) {
	t.Parallel()
	built := builtOptionEod(t)
	got, err := withOptionEodReasons(built, nil, nil)
	if err != nil || got != built {
		t.Fatalf("got %v, %v; want the job as built", got, err)
	}
	// Even a foreign job passes with no writer.
	if got, err := withOptionEodReasons(reasonForeignJob{}, nil, nil); err != nil || got != (reasonForeignJob{}) {
		t.Fatalf("foreign job: %v, %v", got, err)
	}
}

// The reader the site opened with the pages line reaches the page input; the demo profile has none.
func TestOptionDataPageCarriesReasons(t *testing.T) {
	t.Parallel()
	st := opsTodayFixture()
	a := opsTodayApp(t, st)
	var closers []io.Closer
	track := func(c io.Closer, err error) bool {
		if err == nil {
			closers = append(closers, c)
		}
		return err == nil
	}
	v, ok := openOpsTodayRuns(t.Context(), a, track)
	if !ok {
		t.Fatal("open failed")
	}
	rd := siteReaders{ext: map[string]any{siteOpsTodayKey: v}}
	r := optionReasonReader(rd)
	if r == nil {
		t.Fatal("no reason reader for the option data page")
	}
	rows, err := r.RunsOn(context.Background(), "2026-11-13")
	if err != nil || len(rows) != 1 {
		t.Fatalf("reader answered %v, %v", rows, err)
	}
	if _, ok := r.(runview.ReasonWriter); ok {
		t.Fatal("the page reader can write")
	}
	if st.openedDSN != "postgres://pages" {
		t.Fatalf("reader opened with %q, want the pages line", st.openedDSN)
	}
	if optionReasonReader(siteReaders{}) != nil {
		t.Fatal("a site with no run store has a reader")
	}
}

// reasonForeignJob is a job this package of the test built, not one of optioneod.
type reasonForeignJob struct{ contracts.Job }
