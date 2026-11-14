package home_macro_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/home_macro"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	day     = "2026-11-05"
	weekAgo = "2026-10-29"
)

var zone = time.FixedZone("CST", 8*3600)
var generated = time.Date(2026, 11, 6, 9, 30, 0, 0, zone)

func row(t *testing.T, d, value, band string) contracts.ReadingRecord {
	t.Helper()
	tm, err := time.Parse("2006-01-02", d)
	if err != nil {
		t.Fatal(err)
	}
	r := contracts.ReadingRecord{
		Name: "macro.ease", Version: 2, SubjectKind: "market", Subject: "WALCL",
		Day: types.SerializableDate{Time: tm}, Rev: 1, State: "ok", Value: &value,
	}
	params := `[{"param":"hy_oas.data_day","value_num":null,"value_tag":"` + d + `"},{"param":"band","value_num":null,"value_tag":"` + band + `"}]`
	if err := json.Unmarshal([]byte(params), &r.Parameters); err != nil {
		t.Fatal(err)
	}
	return r
}

func readerOf(rows ...contracts.ReadingRecord) compute.ReadingFunc {
	return func(_ context.Context, name, kind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
		var out []contracts.ReadingRecord
		for _, r := range rows {
			d := contracts.Day(r.Day.Format("2006-01-02"))
			if r.Name == name && r.SubjectKind == kind && r.Subject == subject && d >= from && d <= to {
				out = append(out, r)
			}
		}
		return out, nil
	}
}

func build(t *testing.T, read compute.ReadingFunc) []byte {
	t.Helper()
	b, err := home_macro.Build(context.Background(), home_macro.Config{
		RunID: "run-test", BuildID: "build-test", Scope: "scope-test", Zone: zone, Read: read,
	}, day, generated)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
	return b
}

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", filepath.Base(url)))
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func validate(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile("https://optdesk.invalid/schemas/page.macro_summary.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(v); err != nil {
		t.Fatalf("output is invalid: %v\n%s", err, b)
	}
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestGoldenOK(t *testing.T) {
	t.Parallel()
	got := build(t, readerOf(row(t, day, "56.04", "steady"), row(t, weekAgo, "56.24", "steady")))
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, got, "", "  "); err != nil {
		t.Fatal(err)
	}
	pretty.WriteByte('\n')
	want, err := os.ReadFile(filepath.Join("testdata", "ok.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pretty.Bytes(), want) {
		t.Fatalf("golden differs:\n%s", pretty.Bytes())
	}
}

func TestEventsAreNotBuilt(t *testing.T) {
	t.Parallel()
	m := decode(t, build(t, readerOf(row(t, day, "56", "steady"))))
	card := m["card"].(map[string]any)
	if ev := card["events"].([]any); len(ev) != 0 {
		t.Fatalf("events = %v", ev)
	}
	gaps := card["gaps"].([]any)
	last := gaps[len(gaps)-1].(map[string]any)
	if last["reason"] != "not_built" || m["run_status"] != "partial" {
		t.Fatalf("gaps = %v, status = %v", gaps, m["run_status"])
	}
}

func TestNilReadIsNoReaderGap(t *testing.T) {
	t.Parallel()
	card := decode(t, build(t, nil))["card"].(map[string]any)
	if g := card["ease"].(map[string]any); g["reason"] != "no_reader" {
		t.Fatalf("ease = %v", g)
	}
}

func TestNoReadingIsNoDataGap(t *testing.T) {
	t.Parallel()
	m := decode(t, build(t, readerOf()))
	if g := m["card"].(map[string]any)["ease"].(map[string]any); g["reason"] != "no_data" {
		t.Fatalf("ease = %v", g)
	}
	if dt := m["data_times"].([]any); len(dt) != 0 {
		t.Fatalf("data_times = %v", dt)
	}
}

func TestWeekChangeNeedsLastWeek(t *testing.T) {
	t.Parallel()
	card := decode(t, build(t, readerOf(row(t, day, "56", "looser"))))["card"].(map[string]any)
	ease := card["ease"].(map[string]any)
	if g := ease["week_change"].(map[string]any); g["reason"] != "no_data" {
		t.Fatalf("week_change = %v", g)
	}
}

func TestReaderErrorIsError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	read := func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
		return nil, boom
	}
	_, err := home_macro.Build(context.Background(), home_macro.Config{
		RunID: "r", BuildID: "b", Scope: "s", Zone: zone, Read: read,
	}, day, generated)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func TestBadConfigIsError(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]home_macro.Config{
		"nil zone": {RunID: "r", BuildID: "b", Scope: "s"},
		"bad id":   {RunID: "r r", BuildID: "b", Scope: "s", Zone: zone},
	} {
		if _, err := home_macro.Build(context.Background(), c, day, generated); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := home_macro.Build(context.Background(), home_macro.Config{RunID: "r", BuildID: "b", Scope: "s", Zone: zone}, "nope", generated); err == nil {
		t.Error("bad day: no error")
	}
}

type seriesFake struct {
	points []contracts.SeriesPoint
	err    error
}

func (s seriesFake) SeriesPoints(_ context.Context, id string, _, _ contracts.Day) ([]contracts.SeriesPoint, error) {
	if s.err != nil {
		return nil, s.err
	}
	if id != "WRESBAL" {
		return nil, errors.New("only WRESBAL is read, got " + id)
	}
	return s.points, nil
}

func wresbal(t *testing.T, d string, v float64) contracts.SeriesPoint {
	t.Helper()
	tm, err := time.Parse("2006-01-02", d)
	if err != nil {
		t.Fatal(err)
	}
	return contracts.SeriesPoint{ObservationDate: types.SerializableDate{Time: tm}, RealtimeStart: types.SerializableDate{Time: tm}, Value: &v}
}

func reservesResult(level, change string) liquidityapi.Result {
	return liquidityapi.Result{
		Reserves:   liquidityapi.Num{Value: level, Day: "2026-11-04"},
		Reserves4w: liquidityapi.Change{Num: liquidityapi.Num{Value: change, Day: "2026-11-04"}, Weeks: 4, From: "2026-10-07"},
	}
}

func buildReserves(t *testing.T, s store.SeriesReader, res liquidityapi.Result, read compute.ReadingFunc) []byte {
	t.Helper()
	var got liquidityapi.Input
	b, err := home_macro.Build(context.Background(), home_macro.Config{
		RunID: "run-test", BuildID: "build-test", Scope: "scope-test", Zone: zone, Read: read,
		Series: s, LiquidityParams: liquidityapi.Params{DailyMaxAgeDays: 4},
		Liquidity: func(in liquidityapi.Input) liquidityapi.Result {
			got = in
			return res
		},
	}, day, generated)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
	if s != nil {
		if got.Day != day || got.Params.DailyMaxAgeDays != 4 || len(got.WRESBAL) != 1 || got.WALCL != nil || got.SOFR != nil {
			t.Fatalf("input = %+v, want the WRESBAL series alone", got)
		}
	}
	return b
}

func sayOf(t *testing.T, b []byte) string {
	t.Helper()
	return decode(t, b)["card"].(map[string]any)["say"].(string)
}

func TestSayIsReservesFact(t *testing.T) {
	t.Parallel()
	s := seriesFake{points: []contracts.SeriesPoint{wresbal(t, "2026-11-04", 2.97e12)}}
	read := readerOf(row(t, day, "56.04", "steady"), row(t, weekAgo, "56.24", "steady"))
	cases := []struct{ level, change, want string }{
		{"2970000000000", "-40000000000", "Reserves $3.0tn, 4 weeks −$40bn (2026-11-04)."},
		{"2970000000000", "48000000000", "Reserves $3.0tn, 4 weeks +$48bn (2026-11-04)."},
		{"820000000000", "0", "Reserves $820bn, 4 weeks $0bn (2026-11-04)."},
		{"1000000000000", "-400000000", "Reserves $1.0tn, 4 weeks $0bn (2026-11-04)."},
	}
	for _, tc := range cases {
		if got := sayOf(t, buildReserves(t, s, reservesResult(tc.level, tc.change), read)); got != tc.want {
			t.Errorf("say = %q, want %q", got, tc.want)
		}
	}
}

func TestSayMissingReserves(t *testing.T) {
	t.Parallel()
	s := seriesFake{points: []contracts.SeriesPoint{wresbal(t, "2026-11-04", 2.97e12)}}
	stale := reservesResult("2970000000000", "1")
	stale.Reserves.Reason = compute.StaleInput
	noDay := reservesResult("2970000000000", "1")
	noDay.Reserves4w.Reason = liquidityapi.MissingInput
	cases := map[string]struct {
		s   store.SeriesReader
		res liquidityapi.Result
	}{
		"no series":      {nil, reservesResult("2970000000000", "1")},
		"stale figure":   {s, stale},
		"missing 4w day": {s, noDay},
	}
	for name, tc := range cases {
		b := buildReserves(t, tc.s, tc.res, nil)
		if got := sayOf(t, b); got != "Reserves: no data yet." {
			t.Errorf("%s: say = %q", name, got)
		}
		found := false
		for _, g := range decode(t, b)["card"].(map[string]any)["gaps"].([]any) {
			if g.(map[string]any)["reason"] == "no_data" && g.(map[string]any)["text"] == "Reserves: no data yet" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no no_data gap in %s", name, b)
		}
	}
}

func TestSayHasNoEaseWords(t *testing.T) {
	t.Parallel()
	s := seriesFake{points: []contracts.SeriesPoint{wresbal(t, "2026-11-04", 2.97e12)}}
	read := readerOf(row(t, day, "56.04", "steady"), row(t, weekAgo, "56.24", "steady"))
	for _, b := range [][]byte{
		buildReserves(t, s, reservesResult("2970000000000", "1000000000"), read),
		buildReserves(t, nil, liquidityapi.Result{}, read),
	} {
		say := sayOf(t, b)
		if strings.Contains(say, "Ease") || strings.Contains(say, "vs last week") {
			t.Errorf("say = %q", say)
		}
	}
}

func TestSayUnchangedWithoutLiquidity(t *testing.T) {
	t.Parallel()
	b, err := home_macro.Build(context.Background(), home_macro.Config{
		RunID: "run-test", BuildID: "build-test", Scope: "scope-test", Zone: zone,
		Read:   readerOf(row(t, day, "56.04", "steady"), row(t, weekAgo, "56.24", "steady")),
		Series: seriesFake{err: errors.New("must not be read")},
	}, day, generated)
	if err != nil {
		t.Fatal(err)
	}
	if got := sayOf(t, b); got != "Ease neutral, vs last week −0.2." {
		t.Errorf("say = %q", got)
	}
	if got := sayOf(t, build(t, nil)); got != "Ease: no data yet." {
		t.Errorf("say = %q", got)
	}
}

func TestReservesSeriesErrorFailsBuild(t *testing.T) {
	t.Parallel()
	_, err := home_macro.Build(context.Background(), home_macro.Config{
		RunID: "run-test", BuildID: "build-test", Scope: "scope-test", Zone: zone,
		Series: seriesFake{err: errors.New("boom")}, Liquidity: func(liquidityapi.Input) liquidityapi.Result { return liquidityapi.Result{} },
	}, day, generated)
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestGoldenReserves(t *testing.T) {
	t.Parallel()
	s := seriesFake{points: []contracts.SeriesPoint{wresbal(t, "2026-11-04", 2.97e12)}}
	got := buildReserves(t, s, reservesResult("2970000000000", "-40000000000"),
		readerOf(row(t, day, "56.04", "steady"), row(t, weekAgo, "56.24", "steady")))
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, got, "", "  "); err != nil {
		t.Fatal(err)
	}
	pretty.WriteByte('\n')
	want, err := os.ReadFile(filepath.Join("testdata", "reserves.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pretty.Bytes(), want) {
		t.Fatalf("golden differs:\n%s", pretty.Bytes())
	}
}
