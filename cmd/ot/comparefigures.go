package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/cutover"
	"github.com/andrewrong/optdesk/internal/store"
)

func init() { register("compare-figures", compareFigures) }

// figureReportFile is the file of --report: one entry for each figure that is not checked, and ok; no value.
type figureReportFile struct {
	OK      bool               `json:"ok"`
	Results []figureReportLine `json:"results"`
}

type figureReportLine struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
}

// figureInputs are the four input files of the comparison.
type figureInputs struct{ old, register, classes, explained io.Reader }

// readerOnly hides every method of a ReadingStore but the read: what CompareFigures gets cannot write.
type readerOnly struct{ rd store.ReadingReader }

func (r readerOnly) LatestReadings(ctx context.Context, name, kind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
	return r.rd.LatestReadings(ctx, name, kind, subject, from, to)
}

// compareFigures is `ot compare-figures --old FILE --register FILE --classes FILE --explained FILE [--report FILE]`
// (Decisions 7 and 9 of docs/design/cut-over.md). It exits 1 when a figure is unexplained or the comparison fails.
func compareFigures(a *app, args []string) error {
	fs := flag.NewFlagSet("compare-figures", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	old := fs.String("old", "", "the old figure.jsonl (required)")
	reg := fs.String("register", "", "the figure register (required)")
	classes := fs.String("classes", "", "the class file (required)")
	expl := fs.String("explained", "", "the explained entries (required)")
	report := fs.String("report", "", "write the report to this file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	for _, f := range []struct{ name, v string }{{"--old", *old}, {"--register", *reg}, {"--classes", *classes}, {"--explained", *expl}} {
		if f.v == "" {
			return fmt.Errorf("%s is required", f.name)
		}
	}
	if a.profile.Name == "demo" {
		return errors.New("compare-figures needs a real profile; the demo profile opens no store")
	}
	files := make([]io.Reader, 0, 4)
	for _, p := range []string{*old, *reg, *classes, *expl} {
		f, err := os.Open(p) //nolint:gosec // the owner names the files
		if err != nil {
			return fmt.Errorf("cannot open %s", p)
		}
		defer func() { _ = f.Close() }()
		files = append(files, f)
	}
	env, err := a.envFile()
	if err != nil {
		return err
	}
	dsn := env[store.DSNEnvReadings]
	if dsn == "" {
		return fmt.Errorf("%s is not set", store.DSNEnvReadings)
	}
	ctx := context.Background()
	st, err := a.deps.openReading(ctx, dsn)
	if err != nil {
		// The text of an open error may hold the DSN: it is withheld.
		return fmt.Errorf("open readings store with %s failed", store.DSNEnvReadings)
	}
	defer func() { _ = st.Close() }()
	return runCompareFigures(ctx, a.stdout, *report, cutover.CompareFigures, figureInputs{files[0], files[1], files[2], files[3]}, st)
}

// runCompareFigures passes only the reader of st to cmp, prints each figure that is not checked and returns an
// exitError of code 1 when the report is not OK.
func runCompareFigures(ctx context.Context, out io.Writer, reportPath string, cmp jobs.CompareFiguresFunc, in figureInputs, st store.ReadingReader) error {
	rep, err := cmp(ctx, in.old, in.register, in.classes, in.explained, readerOnly{st})
	if err != nil {
		return &exitError{code: 1, err: fmt.Errorf("compare-figures: %w", err)}
	}
	file := figureReportFile{OK: rep.OK, Results: []figureReportLine{}}
	for _, r := range rep.Results {
		if r.Status == jobs.FigureChecked {
			continue
		}
		_, _ = fmt.Fprintf(out, "%s %d %s %s\n", r.ID, r.Version, r.Status, r.Reason)
		file.Results = append(file.Results, figureReportLine{ID: r.ID, Version: r.Version, Status: string(r.Status), Reason: r.Reason})
	}
	if reportPath != "" {
		raw, err := json.Marshal(file)
		if err != nil {
			return errors.New("cannot encode the report")
		}
		if err := os.WriteFile(reportPath, append(raw, '\n'), 0o644); err != nil { //nolint:gosec // a report holds no value
			return errors.New("cannot write the report file")
		}
	}
	if !rep.OK {
		return &exitError{code: 1, err: errors.New("compare-figures: a figure is unexplained")}
	}
	return nil
}
