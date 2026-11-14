package main

import (
	"context"
	"io"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/macroevents"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

type fakeEventReader struct{ macroevent.Reader }

type fakeEventWriter struct{ macroevent.Writer }

type fakeReleases struct{}

func (fakeReleases) Dates(context.Context, string, contracts.Day) ([]contracts.Day, error) {
	return nil, nil
}

func macroDeps(readerOpens *int) macroEventsDeps {
	return macroEventsDeps{
		newJob: func(macroevents.ReleaseSource, macroevent.Writer, clock.Clock) (contracts.Job, error) {
			return namedJob{id: jobMacroEvents}, nil
		},
		newRelease: func(string, string) (macroevents.ReleaseSource, error) { return fakeReleases{}, nil },
		openWriter: func(context.Context, string) (macroevent.Writer, io.Closer, error) {
			return fakeEventWriter{}, nopCloser{}, nil
		},
		openReader: func(context.Context, string) (macroevent.Reader, io.Closer, error) {
			*readerOpens++
			return fakeEventReader{}, nopCloser{}, nil
		},
	}
}

func TestMacroEventsRegistered(t *testing.T) {
	t.Parallel()
	if !slices.Contains(familyIDs(), jobMacroEvents) {
		t.Fatalf("families %v hold no macro_events", familyIDs())
	}
	off := startDisabled(runOnceConfig{}, true)
	if off[jobMacroEvents] != reasonNoFredKey {
		t.Fatalf("empty key: off %q, want %q", off[jobMacroEvents], reasonNoFredKey)
	}
	if r := startDisabled(runOnceConfig{fredKey: "k"}, true)[jobMacroEvents]; r != "" {
		t.Fatalf("with a key: off %q", r)
	}

	for _, key := range []string{"", "k"} {
		var opens int
		a, _ := testApp(t, "")
		a.deps.ext = map[string]any{extMacroEvents: macroDeps(&opens)}
		extra := &closerSet{}
		jc := &jobCtx{cfg: runOnceConfig{fredKey: key, seriesDSN: "ser"}, f: &jobFactory{d: a.deps}, extra: extra}
		j, reason, err := macroEventsJob(context.Background(), a, jc)
		if err != nil || j == nil || j.ID() != jobMacroEvents {
			t.Fatalf("key %q: job %v err %v", key, j, err)
		}
		want := ""
		if key == "" {
			want = reasonNoFredKey
		}
		if reason != want {
			t.Fatalf("key %q: reason %q, want %q", key, reason, want)
		}
		if len(extra.cs) != 1 {
			t.Fatalf("key %q: %d closers, want the writer's", key, len(extra.cs))
		}
	}
}

func TestMacroPageEventsNilWithoutDSN(t *testing.T) {
	t.Parallel()
	var opens int
	a, _ := testApp(t, "OT_UNRELATED=1\n")
	a.deps.ext = map[string]any{extMacroEvents: macroDeps(&opens)}
	called := 0
	a.deps.macroPage = func(*time.Location, string, compute.ReadingFunc) pages.Page { called++; return pages.Page{} }
	r, closeFn, err := openMacroEvents(context.Background(), a)
	if err != nil || r != nil {
		t.Fatalf("reader %v err %v without a pages line", r, err)
	}
	closeFn()
	if _, closeFn, err := builtPages()[macroPageName].build(context.Background(), a, "", ""); err != nil {
		t.Fatal(err)
	} else {
		closeFn()
	}
	if called != 1 || opens != 0 {
		t.Fatalf("macroPage called %d times, reader opened %d times", called, opens)
	}
}

// recordingEvents is a reader that records the window the page asks for.
type recordingEvents struct {
	macroevent.Reader
	between atomic.Int32
}

func (r *recordingEvents) Between(context.Context, contracts.Day, contracts.Day) ([]macroevent.Event, error) {
	r.between.Add(1)
	return []macroevent.Event{}, nil
}

func (r *recordingEvents) Kinds(context.Context) ([]macroevent.Kind, error) {
	return []macroevent.Kind{}, nil
}

func TestMacroPageEventsUsedWithReader(t *testing.T) {
	t.Parallel()
	rec := &recordingEvents{}
	var closed atomic.Int32
	md := macroDeps(new(int))
	md.openReader = func(_ context.Context, dsn string) (macroevent.Reader, io.Closer, error) {
		if dsn != "pages-dsn" {
			t.Errorf("reader opened with %q", dsn)
		}
		return rec, countCloser{&closed}, nil
	}
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
	a.deps.ext = map[string]any{extMacroEvents: md}
	a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) { return emptyView{}, nil }
	a.deps.macroPage = func(*time.Location, string, compute.ReadingFunc) pages.Page {
		t.Error("macroPage called with a reader")
		return pages.Page{}
	}
	p, closeFn, err := builtPages()[macroPageName].build(context.Background(), a, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Build(context.Background(), noSeries{}, "2026-11-06", time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if rec.between.Load() == 0 {
		t.Fatal("the reader never reached the page")
	}
	closeFn()
	if closed.Load() != 1 {
		t.Fatalf("reader closed %d times, want 1", closed.Load())
	}
}

type countCloser struct{ n *atomic.Int32 }

func (c countCloser) Close() error { c.n.Add(1); return nil }

type emptyView struct{}

func (emptyView) WinnerReadings(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
	return nil, nil
}
func (emptyView) Close() error { return nil }
