package fredseries

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/seriesdef"
)

// registryWriter adds the registry and fill methods to fakeWriter.
type registryWriter struct {
	fakeWriter
	defs  []seriesdef.SeriesDefinition
	fills [][]contracts.SeriesPoint
}

func (w *registryWriter) ActiveSeries(context.Context) ([]seriesdef.SeriesDefinition, error) {
	return w.defs, nil
}

func (w *registryWriter) FillSeriesPoints(_ context.Context, rows []contracts.SeriesPoint, _ string) (seriesdef.WriteCount, error) {
	w.fills = append(w.fills, rows)
	return seriesdef.WriteCount{Written: len(rows)}, nil
}

func defOf(id, second string) []seriesdef.SeriesDefinition {
	return []seriesdef.SeriesDefinition{{SeriesID: id, SecondarySource: second, Active: true}}
}

func sofr(v float64) contracts.SeriesPoint {
	p := val(v)
	p.SeriesID = "SOFR"
	return p
}

func runSecond(t *testing.T, primary, second *fakeSource, w store.SeriesWriter) (contracts.JobResult, *bytes.Buffer) {
	t.Helper()
	j, err := NewWithSecondary(primary, second, w, fakeClock{})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	j.(*job).log = slog.New(slog.NewTextHandler(&buf, nil))
	k := []contracts.ExpectedKey{{DatasetID: "obs.series_point", SubjectKey: "SOFR"}}
	return j.Run(context.Background(), "2026-11-05", k), &buf
}

func TestFallbackRunsOnNoValue(t *testing.T) {
	t.Parallel()
	w := &registryWriter{defs: defOf("SOFR", "nyfed")}
	second := &fakeSource{results: []fetchResult{{points: []contracts.SeriesPoint{sofr(4.3)}}}}
	gap := []contracts.SeriesPoint{{SeriesID: "SOFR", Status: "no_value"}}
	res, _ := runSecond(t, &fakeSource{results: []fetchResult{{points: gap}}}, second, w)
	if res.Err != nil || len(second.plans) != 1 || len(w.fills) != 1 {
		t.Fatalf("res=%+v plans=%v fills=%v", res, second.plans, w.fills)
	}
	if p := second.plans[0]; len(p.SeriesIDs) != 1 || p.SeriesIDs[0] != "SOFR" || p.To != "2026-11-05" {
		t.Fatalf("plan = %+v", p)
	}
}

func TestFallbackRunsOnMissingDate(t *testing.T) {
	t.Parallel()
	w := &registryWriter{defs: defOf("SOFR", "nyfed")}
	second := &fakeSource{results: []fetchResult{{points: []contracts.SeriesPoint{sofr(4.3)}}}}
	runSecond(t, &fakeSource{}, second, w)
	if len(w.fills) != 1 {
		t.Fatalf("fills = %v", w.fills)
	}
}

func TestFallbackNotRunWhenPrimaryHasValue(t *testing.T) {
	t.Parallel()
	w := &registryWriter{defs: defOf("SOFR", "nyfed")}
	second := &fakeSource{}
	today := sofr(4.1)
	today.ObservationDate = types.SerializableDate{Time: time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)}
	runSecond(t, &fakeSource{results: []fetchResult{{points: []contracts.SeriesPoint{today}}}}, second, w)
	if len(second.plans) != 0 || len(w.fills) != 0 {
		t.Fatalf("fallback ran with a value: %v", second.plans)
	}
}

func TestFallbackSkipsSeriesWithoutSecondary(t *testing.T) {
	t.Parallel()
	w := &registryWriter{defs: defOf("SOFR", "")}
	second := &fakeSource{}
	runSecond(t, &fakeSource{}, second, w)
	if len(second.plans) != 0 || len(w.fills) != 0 {
		t.Fatalf("second source asked: %v", second.plans)
	}
}

func TestFallbackUsesFillNotUpsert(t *testing.T) {
	t.Parallel()
	w := &registryWriter{defs: defOf("SOFR", "nyfed")}
	second := &fakeSource{results: []fetchResult{{points: []contracts.SeriesPoint{sofr(4.3)}}}}
	runSecond(t, &fakeSource{}, second, w)
	if len(w.calls) != 0 {
		t.Fatalf("upsert called with %v", w.calls)
	}
	if len(w.fills) != 1 || *w.fills[0][0].Value != 4.3 {
		t.Fatalf("fills = %v", w.fills)
	}
}

func TestFallbackWriterWithoutRegistry(t *testing.T) {
	t.Parallel()
	second := &fakeSource{}
	runSecond(t, &fakeSource{}, second, &fakeWriter{})
	if len(second.plans) != 0 {
		t.Fatalf("second source asked: %v", second.plans)
	}
}

func TestSecondSourceErrorKeepsResultAndIsLogged(t *testing.T) {
	t.Parallel()
	w := &registryWriter{defs: defOf("SOFR", "nyfed")}
	second := &fakeSource{results: []fetchResult{{err: errors.New("nyfed down")}}}
	gap := []contracts.SeriesPoint{{SeriesID: "SOFR", Status: "no_value"}}
	res, buf := runSecond(t, &fakeSource{results: []fetchResult{{points: gap}}}, second, w)
	if res.Err != nil || res.Returned != 1 || res.Usable != 0 {
		t.Fatalf("result changed: %+v", res)
	}
	if !strings.Contains(buf.String(), "second source failed") || !strings.Contains(buf.String(), "nyfed down") {
		t.Fatalf("log = %q", buf.String())
	}
}

func TestFallbackFillsMissingDateInWindow(t *testing.T) {
	t.Parallel()
	w := &registryWriter{defs: defOf("SOFR", "nyfed")}
	second := &fakeSource{results: []fetchResult{{points: []contracts.SeriesPoint{sofr(4.3)}}}}
	yesterday := sofr(4.2)
	yesterday.ObservationDate = types.SerializableDate{Time: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)} // run day 2026-11-05 missing from FRED
	runSecond(t, &fakeSource{results: []fetchResult{{points: []contracts.SeriesPoint{yesterday}}}}, second, w)
	if len(second.plans) != 1 || len(w.fills) != 1 {
		t.Fatalf("missing run day not filled: plans=%v fills=%v", second.plans, w.fills)
	}
}
