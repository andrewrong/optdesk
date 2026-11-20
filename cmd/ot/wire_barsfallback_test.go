package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

type fbCalendar struct{ clock.Calendar }

type fbJob struct{ contracts.Job }

// fbFactory is a factory with a pyside project directory and a session-day store, the two things the fallback needs.
func fbFactory(t *testing.T) jobFactory {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return jobFactory{project: dir, cal: fbCalendar{}}
}

type attachRec struct {
	src sources.BarSource
	cal clock.Calendar
	n   int
}

func (r *attachRec) attach(j contracts.Job, src sources.BarSource, cal clock.Calendar) (contracts.Job, error) {
	r.n++
	r.src, r.cal = src, cal
	return fbJob{j}, nil
}

func TestBarsRefreshHasFallback(t *testing.T) {
	t.Parallel()
	f := fbFactory(t)
	rec := &attachRec{}
	in := namedJob{id: jobBarsRefresh}
	got := f.withBarsFallbackIn(in, barSources, rec.attach)
	if rec.n != 1 || rec.src == nil || rec.cal == nil {
		t.Fatalf("attach calls %d source %v calendar %v", rec.n, rec.src, rec.cal)
	}
	if _, ok := got.(fbJob); !ok {
		t.Fatalf("the job returned is %T, want the one the fallback returned", got)
	}
}

func TestBarsRefreshWithoutCommand(t *testing.T) {
	t.Parallel()
	f := fbFactory(t)
	rec := &attachRec{}
	in := namedJob{id: jobBarsRefresh}
	if got := f.withBarsFallbackIn(in, map[string]sources.NewBarSourceFunc{}, rec.attach); got != contracts.Job(in) || rec.n != 0 {
		t.Fatalf("job %v attach calls %d, want the job unchanged and no attach", got, rec.n)
	}
	// no pyside project: the command is missing
	f.project = t.TempDir()
	if got := f.withBarsFallbackIn(in, barSources, rec.attach); got != contracts.Job(in) || rec.n != 0 {
		t.Fatalf("job %v attach calls %d, want the job unchanged and no attach", got, rec.n)
	}
}

func TestBarsRefreshWithoutCalendar(t *testing.T) {
	t.Parallel()
	f := fbFactory(t)
	f.cal = nil
	rec := &attachRec{}
	in := namedJob{id: jobBarsRefresh}
	if got := f.withBarsFallbackIn(in, barSources, rec.attach); got != contracts.Job(in) || rec.n != 0 {
		t.Fatalf("job %v attach calls %d, want the job unchanged and no attach", got, rec.n)
	}
}
