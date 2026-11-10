package home_macro_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/home_macro"
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
