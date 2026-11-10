package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

// readOnlyFake is a reader with no write method.
type readOnlyFake struct{ calls int }

func (r *readOnlyFake) LatestReadings(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
	r.calls++
	return nil, nil
}

func cfFiles() figureInputs {
	return figureInputs{strings.NewReader(""), strings.NewReader(""), strings.NewReader(""), strings.NewReader("")}
}

func cfReturning(rep jobs.FigureReport, err error) jobs.CompareFiguresFunc {
	return func(context.Context, io.Reader, io.Reader, io.Reader, io.Reader, store.ReadingReader) (jobs.FigureReport, error) {
		return rep, err
	}
}

func TestCompareFiguresGetsReaderOnly(t *testing.T) {
	t.Parallel()
	fake := &readOnlyFake{}
	called := false
	cmp := func(ctx context.Context, _, _, _, _ io.Reader, rd store.ReadingReader) (jobs.FigureReport, error) {
		called = true
		if _, ok := rd.(store.ReadingWriter); ok {
			t.Error("CompareFigures got a reader that can write")
		}
		_, _ = rd.LatestReadings(ctx, "n", "k", "s", "2026-11-01", "2026-11-02")
		return jobs.FigureReport{OK: true}, nil
	}
	var out bytes.Buffer
	if err := runCompareFigures(context.Background(), &out, "", cmp, cfFiles(), fake); err != nil {
		t.Fatal(err)
	}
	if !called || fake.calls != 1 {
		t.Fatalf("called %v, reads %d", called, fake.calls)
	}
}

func TestCompareFiguresExit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		rep  jobs.FigureReport
		err  error
		code int
	}{
		{"ok", jobs.FigureReport{OK: true}, nil, 0},
		{"not ok", jobs.FigureReport{OK: false, Results: []jobs.FigureResult{{ID: "a", Version: 1, Status: jobs.FigureUnexplained}}}, nil, 1},
		{"error", jobs.FigureReport{}, errors.New("unclassified ids: x"), 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := runCompareFigures(context.Background(), io.Discard, "", cfReturning(c.rep, c.err), cfFiles(), &readOnlyFake{})
			var ee *exitError
			switch {
			case c.code == 0 && err != nil:
				t.Fatalf("err = %v, want nil", err)
			case c.code == 1 && (!errors.As(err, &ee) || ee.code != 1):
				t.Fatalf("err = %v, want exit 1", err)
			}
		})
	}
}

func TestCompareFiguresPrintsNoValue(t *testing.T) {
	t.Parallel()
	rep := jobs.FigureReport{OK: true, Results: []jobs.FigureResult{
		{ID: "home.a", Version: 1, Status: jobs.FigureChecked},
		{ID: "home.b", Version: 2, Status: jobs.FigureUnchecked, Reason: "path_not_found"},
		{ID: "home.c", Version: 1, Status: jobs.FigureExplained, Reason: "rounding"},
	}}
	report := filepath.Join(t.TempDir(), "r.json")
	var out bytes.Buffer
	if err := runCompareFigures(context.Background(), &out, report, cfReturning(rep, nil), cfFiles(), &readOnlyFake{}); err != nil {
		t.Fatal(err)
	}
	want := "home.b 2 unchecked path_not_found\nhome.c 1 explained rounding\n"
	if out.String() != want {
		t.Fatalf("stdout %q, want %q", out.String(), want)
	}
	raw, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "home.a") || !strings.Contains(string(raw), `"ok":true`) {
		t.Fatalf("report %s", raw)
	}
}

func TestCompareFiguresFlagsAndDSN(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=x\n")
	if err := compareFigures(a, []string{"--old", "x"}); err == nil || !strings.Contains(err.Error(), "--register is required") {
		t.Fatalf("err = %v", err)
	}
	dir := t.TempDir()
	var args []string
	for _, f := range []string{"old", "register", "classes", "explained"} {
		p := filepath.Join(dir, f)
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--"+f, p)
	}
	if err := compareFigures(a, args); err == nil || !strings.Contains(err.Error(), store.DSNEnvReadings) {
		t.Fatalf("err = %v", err)
	}
}

func TestCompareFiguresWithholdsDSN(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_READINGS=postgres://u:secretpw@h/db\n")
	a.deps.openReading = func(context.Context, string) (store.ReadingStore, error) {
		return nil, errors.New("dial secretpw")
	}
	dir := t.TempDir()
	var args []string
	for _, f := range []string{"old", "register", "classes", "explained"} {
		p := filepath.Join(dir, f)
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--"+f, p)
	}
	err := compareFigures(a, args)
	if err == nil || strings.Contains(err.Error(), "secretpw") {
		t.Fatalf("err = %v", err)
	}
}
