// Package calendarcli is the session source that runs the pyside calendar generator as a command.
package calendarcli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// New is clock.NewCommandSessionSourceFunc: argv is the command prefix, workDir the directory
// under which each call makes its own temporary directory, run the process runner.
func New(argv []string, workDir string, run clock.CommandFunc) clock.SessionSource {
	return source{argv: append([]string(nil), argv...), workDir: workDir, run: run}
}

type source struct {
	argv    []string
	workDir string
	run     clock.CommandFunc
}

type typedError struct {
	kind contracts.ErrorKind
	msg  string
}

func (e *typedError) Error() string             { return "calendarcli: " + e.msg }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func permanent(msg string) error { return &typedError{contracts.Permanent, msg} }

type provenance struct {
	Generator            string `json:"generator"`
	GeneratorVersion     string `json:"generator_version"`
	SourceLibrary        string `json:"source_library"`
	SourceLibraryVersion string `json:"source_library_version"`
	GeneratedAt          string `json:"generated_at"`
}

type file struct {
	Provenance *provenance            `json:"provenance"`
	Rows       []contracts.SessionDay `json:"rows"`
}

// SessionDays runs the generator over the days and returns the rows it wrote. No error text holds
// a path or a value of the file.
func (s source) SessionDays(ctx context.Context, calendarID string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	first, err1 := time.Parse(time.DateOnly, string(from))
	last, err2 := time.Parse(time.DateOnly, string(to))
	if err1 != nil || err2 != nil {
		return nil, permanent("the requested days are not dates")
	}
	dir, err := os.MkdirTemp(s.workDir, "calendar-")
	if err != nil {
		return nil, &typedError{contracts.Retryable, "cannot make the temporary directory"}
	}
	defer func() { _ = os.RemoveAll(dir) }()
	out := filepath.Join(dir, "calendar.json")

	argv := append(append([]string(nil), s.argv...),
		"--calendar", calendarID, "--from", string(from), "--to", string(to), "--out", out)
	code, err := s.run(ctx, argv)
	if err != nil {
		return nil, &typedError{contracts.Retryable, "the generator could not be run or waited for"}
	}
	switch code {
	case 0:
	case 1:
		return nil, permanent("the generator failed (exit 1)")
	case 2:
		return nil, permanent("the generator refused its arguments (exit 2)")
	default:
		return nil, permanent("the generator exited with a nonzero code")
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		return nil, permanent("the generator wrote no readable file")
	}
	var f file
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, permanent("the generated file is not a valid calendar object")
	}
	if err := check(f, calendarID, first, last); err != nil {
		return nil, err
	}
	return f.Rows, nil
}

func check(f file, calendarID string, first, last time.Time) error {
	p := f.Provenance
	if p == nil {
		return permanent("the generated file has no provenance")
	}
	for _, v := range []string{p.Generator, p.GeneratorVersion, p.SourceLibrary, p.SourceLibraryVersion, p.GeneratedAt} {
		if v == "" {
			return permanent("the provenance of the generated file is incomplete")
		}
	}
	if len(f.Rows) == 0 {
		return permanent("the generated file has no rows")
	}
	if !f.Rows[0].SessionDay.Time.Equal(first) || !f.Rows[len(f.Rows)-1].SessionDay.Time.Equal(last) {
		return permanent("the rows do not cover the requested days")
	}
	var prev time.Time
	for i, r := range f.Rows {
		if r.CalendarID != calendarID {
			return permanent("a row has another calendar id than requested")
		}
		day := r.SessionDay.Time
		if day.Before(first) || day.After(last) {
			return permanent("a row is outside the requested days")
		}
		if i > 0 && !day.After(prev) {
			return permanent("the days are not unique and ascending")
		}
		if i > 0 && !day.Equal(prev.AddDate(0, 0, 1)) {
			return permanent("the days are not one row per calendar day")
		}
		prev = day
		if r.IsSession && !r.OpenAt.Before(r.CloseAt) {
			return permanent("a session day needs open_at before close_at")
		}
	}
	return nil
}
