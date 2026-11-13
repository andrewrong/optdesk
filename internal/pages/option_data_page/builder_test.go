package option_data_page

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/contracts"
)

var fixedAt = time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)

type quoteCall struct {
	id       int64
	from, to contracts.Day
}

type fakeEod struct {
	rows     []contracts.OptionEod
	err      error
	mu       sync.Mutex
	from, to []contracts.Day
}

func (f *fakeEod) OptionEod(_ context.Context, _ int64, from, to contracts.Day) ([]contracts.OptionEod, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.from, f.to = append(f.from, from), append(f.to, to)
	return f.rows, f.err
}

type fakeQuotes struct {
	rows  map[int64][]contracts.QuoteSnapshot
	err   error
	mu    sync.Mutex
	calls []quoteCall
}

func (f *fakeQuotes) QuoteSnapshots(_ context.Context, id int64, from, to contracts.Day) ([]contracts.QuoteSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, quoteCall{id, from, to})
	return f.rows[id], f.err
}

func f64(v float64) *float64 { return &v }

func sday(t *testing.T, s string) types.SerializableDate {
	t.Helper()
	d, err := time.Parse(dayLayout, s)
	if err != nil {
		t.Fatal(err)
	}
	return types.SerializableDate{Time: d}
}

func eodRow(t *testing.T, id int, oi *float64, recv string) contracts.OptionEod {
	t.Helper()
	at, err := time.Parse(time.RFC3339, recv)
	if err != nil {
		t.Fatal(err)
	}
	return contracts.OptionEod{
		SessionDay: sday(t, "2026-11-06"), InstrumentID: id, UnderlyingID: 9000, Status: 1,
		ReceivedAt: at, OpenInterest: oi, Bid: f64(1), Ask: f64(2), Last: f64(1.5),
	}
}

func cfg(e *fakeEod, q *fakeQuotes) Config {
	return Config{RunID: "run_1", BuildID: "build_1", Scope: "scope_1", Zone: time.UTC,
		UnderlyingID: 9000, Label: "Synthetic underlying", Eod: e, Quotes: q}
}

func buildMap(t *testing.T, c Config) map[string]any {
	t.Helper()
	b, err := Build(t.Context(), c, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func rowsOf(m map[string]any) []any { return m["chain"].(map[string]any)["rows"].([]any) }

func ids(m map[string]any) []float64 {
	var out []float64
	for _, r := range rowsOf(m) {
		out = append(out, r.(map[string]any)["instrument_id"].(float64))
	}
	return out
}

func schemaDir() string { return filepath.Join("..", "..", "..", "contracts", "schemas") }

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	const base = "https://optdesk.invalid/schemas/"
	for _, name := range []string{"common.v2.json", "page.option_data_page.v1.json"} {
		f, err := os.Open(filepath.Join(schemaDir(), name))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddResource(base+name, doc); err != nil {
			t.Fatal(err)
		}
	}
	s, err := c.Compile(base + "page.option_data_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validateBytes(t *testing.T, b []byte) error {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return compileSchema(t).Validate(v)
}

type input struct {
	Day          string       `json:"day"`
	RunID        string       `json:"run_id"`
	BuildID      string       `json:"build_id"`
	Scope        string       `json:"scope"`
	GeneratedAt  string       `json:"generated_at"`
	UnderlyingID int64        `json:"underlying_id"`
	Label        string       `json:"label"`
	Eod          []plainEod   `json:"eod"`
	Quotes       []plainQuote `json:"quotes"`
}

// plainEod and plainQuote drop the generated UnmarshalJSON, which demands fields the sample omits.
type (
	plainEod   contracts.OptionEod
	plainQuote contracts.QuoteSnapshot
)

func loadInput(t *testing.T) input { return loadInputNamed(t, "input") }

func loadInputNamed(t *testing.T, name string) input {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "fixtures", "page.option_data_page.v1."+name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var in input
	if err := json.Unmarshal(b, &in); err != nil {
		t.Fatal(err)
	}
	return in
}

func TestBuildValidates(t *testing.T) {
	t.Parallel()
	in := loadInput(t)
	e, q := fromInput(in)
	b, err := Build(t.Context(), inputCfg(in, e, q), contracts.Day(in.Day), fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateBytes(t, b); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	empty, err := Build(t.Context(), cfg(&fakeEod{}, &fakeQuotes{}), "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateBytes(t, empty); err != nil {
		t.Fatalf("empty page: %v\n%s", err, empty)
	}
}

func inputCfg(in input, e *fakeEod, q *fakeQuotes) Config {
	c := cfg(e, q)
	c.RunID, c.BuildID, c.Scope, c.UnderlyingID, c.Label = in.RunID, in.BuildID, in.Scope, in.UnderlyingID, in.Label
	return c
}

func fromInput(in input) (*fakeEod, *fakeQuotes) {
	e := &fakeEod{}
	for _, r := range in.Eod {
		e.rows = append(e.rows, contracts.OptionEod(r))
	}
	q := &fakeQuotes{rows: map[int64][]contracts.QuoteSnapshot{}}
	for _, r := range in.Quotes {
		q.rows[int64(r.InstrumentID)] = append(q.rows[int64(r.InstrumentID)], contracts.QuoteSnapshot(r))
	}
	return e, q
}

func TestBuildGolden(t *testing.T) {
	t.Parallel()
	in := loadInput(t)
	e, q := fromInput(in)
	// ok.json holds the note of the sample; the input has none, so the config carries it.
	c := inputCfg(in, e, q)
	c.Notes = []string{"Synthetic fixture."}
	got := func() map[string]any {
		b, err := Build(t.Context(), c, contracts.Day(in.Day), fixedAt)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "fixtures", "page.option_data_page.v1.ok.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		g, _ := json.MarshalIndent(got, "", " ")
		w, _ := json.MarshalIndent(want, "", " ")
		t.Fatalf("page differs\n got: %s\nwant: %s", g, w)
	}
}

func TestBuildOffSessionGolden(t *testing.T) {
	t.Parallel()
	in := loadInputNamed(t, "off-session-input")
	e, q := fromInput(in)
	c := inputCfg(in, e, q)
	c.Notes = []string{"Synthetic fixture."}
	b, err := Build(t.Context(), c, contracts.Day(in.Day), fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateBytes(t, b); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	var got, want map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "fixtures", "page.option_data_page.v1.off-session.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		g, _ := json.MarshalIndent(got, "", " ")
		w, _ := json.MarshalIndent(want, "", " ")
		t.Fatalf("page differs\n got: %s\nwant: %s", g, w)
	}
	if got["underlying"].(map[string]any)["off_session"] != true {
		t.Fatalf("off_session not true: %v", got["underlying"])
	}
}

func TestBuildOffSessionMark(t *testing.T) {
	t.Parallel()
	row := func(id int, kind, recv string) contracts.OptionEod {
		r := eodRow(t, id, nil, recv)
		r.SnapshotKind = kind
		return r
	}
	has := func(m map[string]any) bool {
		_, ok := m["underlying"].(map[string]any)["off_session"]
		return ok
	}
	tests := []struct {
		name string
		rows []contracts.OptionEod
		want bool
	}{
		{"all off session", []contracts.OptionEod{row(1, "off_session", "2026-11-06T21:00:00Z"), row(2, "off_session", "2026-11-06T21:00:00Z")}, true},
		{"mixed", []contracts.OptionEod{row(1, "off_session", "2026-11-06T21:00:00Z"), row(2, "session", "2026-11-06T21:00:00Z")}, false},
		{"all session", []contracts.OptionEod{row(1, "session", "2026-11-06T21:00:00Z")}, false},
		{"legacy eod", []contracts.OptionEod{row(1, "eod", "2026-11-06T21:00:00Z")}, false},
		{"picked capture decides", []contracts.OptionEod{row(1, "session", "2026-11-06T20:00:00Z"), row(1, "off_session", "2026-11-06T21:00:00Z")}, true},
		{"no rows", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := buildMap(t, cfg(&fakeEod{rows: tc.rows}, &fakeQuotes{}))
			if has(m) != tc.want {
				t.Fatalf("off_session present = %v, want %v", has(m), tc.want)
			}
			if tc.want && m["underlying"].(map[string]any)["off_session"] != true {
				t.Fatal("off_session is not true")
			}
		})
	}
}

func TestBuildSelection(t *testing.T) {
	t.Parallel()
	t.Run("later capture, order and series", func(t *testing.T) {
		t.Parallel()
		early := eodRow(t, 1, f64(10), "2026-11-06T20:00:00Z")
		early.Bid = f64(5)
		late := eodRow(t, 1, f64(10), "2026-11-06T21:00:00Z")
		late.Bid = f64(6)
		e := &fakeEod{rows: []contracts.OptionEod{
			late, early,
			eodRow(t, 2, nil, "2026-11-06T21:00:00Z"),
			eodRow(t, 3, f64(50), "2026-11-06T21:00:00Z"),
			eodRow(t, 4, f64(10), "2026-11-06T21:00:00Z"),
		}}
		q := &fakeQuotes{rows: map[int64][]contracts.QuoteSnapshot{}}
		m := buildMap(t, cfg(e, q))
		if got, want := ids(m), []float64{3, 1, 4, 2}; !reflect.DeepEqual(got, want) {
			t.Fatalf("order = %v, want %v", got, want)
		}
		bid := rowsOf(m)[1].(map[string]any)["bid"].(map[string]any)["value"]
		if bid != 6.0 {
			t.Fatalf("bid = %v, want the later capture's 6", bid)
		}
		if n := len(m["quote_series"].([]any)); n != 3 {
			t.Fatalf("series = %d, want 3 for 4 instruments", n)
		}
		if len(q.calls) != 3 {
			t.Fatalf("quote reads = %d", len(q.calls))
		}
		for _, c := range q.calls {
			if c.from != "2026-10-30" || c.to != "2026-11-06" {
				t.Fatalf("quote read %+v, want 2026-10-30 to 2026-11-06", c)
			}
		}
		if e.from[0] != "2026-10-30" || e.to[0] != "2026-11-06" || len(e.from) != 1 {
			t.Fatalf("eod read %v %v", e.from, e.to)
		}
	})
	t.Run("tie keeps the later row", func(t *testing.T) {
		t.Parallel()
		a := eodRow(t, 1, f64(10), "2026-11-06T21:00:00Z")
		a.Bid = f64(1)
		b := a
		b.Bid = f64(2)
		m := buildMap(t, cfg(&fakeEod{rows: []contracts.OptionEod{a, b}}, &fakeQuotes{}))
		if v := rowsOf(m)[0].(map[string]any)["bid"].(map[string]any)["value"]; v != 2.0 {
			t.Fatalf("bid = %v, want 2", v)
		}
	})
	t.Run("latest session day only", func(t *testing.T) {
		t.Parallel()
		old := eodRow(t, 1, f64(10), "2026-11-05T21:00:00Z")
		old.SessionDay = sday(t, "2026-11-05")
		m := buildMap(t, cfg(&fakeEod{rows: []contracts.OptionEod{old, eodRow(t, 2, f64(1), "2026-11-06T21:00:00Z")}}, &fakeQuotes{}))
		if got := ids(m); !reflect.DeepEqual(got, []float64{2}) {
			t.Fatalf("ids = %v", got)
		}
	})
	t.Run("underlying price is the latest capture's", func(t *testing.T) {
		t.Parallel()
		a := eodRow(t, 1, f64(10), "2026-11-06T21:00:00Z")
		a.UnderlyingPrice = f64(100)
		b := eodRow(t, 2, f64(5), "2026-11-06T22:00:00Z")
		b.UnderlyingPrice = f64(101)
		m := buildMap(t, cfg(&fakeEod{rows: []contracts.OptionEod{b, a}}, &fakeQuotes{}))
		if v := m["underlying"].(map[string]any)["price"].(map[string]any)["value"]; v != 101.0 {
			t.Fatalf("price = %v", v)
		}
	})
	t.Run("sizes", checkSizes)
}

func checkSizes(t *testing.T) {
	t.Parallel()
	var rows []contracts.OptionEod
	for i := 1; i <= 600; i++ {
		rows = append(rows, eodRow(t, i, f64(float64(i)), "2026-11-06T21:00:00Z"))
	}
	var qs []contracts.QuoteSnapshot
	for i := 0; i < 300; i++ {
		qs = append(qs, contracts.QuoteSnapshot{SessionDay: sday(t, "2026-11-06"), Slot: fmt.Sprint(i), Bid: f64(1)})
	}
	m := buildMap(t, cfg(&fakeEod{rows: rows}, &fakeQuotes{rows: map[int64][]contracts.QuoteSnapshot{600: qs}}))
	if n := len(rowsOf(m)); n != 500 {
		t.Fatalf("rows = %d", n)
	}
	pts := m["quote_series"].([]any)[0].(map[string]any)["points"].([]any)
	if len(pts) != 200 || pts[0].(map[string]any)["slot"] != "0" || pts[199].(map[string]any)["slot"] != "199" {
		t.Fatalf("points = %d, want the first 200 in reader order", len(pts))
	}
	if got := len(limit(make([]gap, 30), maxGaps)); got != 20 {
		t.Fatalf("gaps = %d", got)
	}
}

func TestBuildGaps(t *testing.T) {
	t.Parallel()
	t.Run("null values are gaps", func(t *testing.T) {
		t.Parallel()
		r := eodRow(t, 1, f64(10), "2026-11-06T21:00:00Z")
		r.Bid, r.Delta, r.Status = nil, nil, 2
		m := buildMap(t, cfg(&fakeEod{rows: []contracts.OptionEod{r}}, &fakeQuotes{}))
		row := rowsOf(m)[0].(map[string]any)
		for _, k := range []string{"bid", "delta", "gamma", "iv", "strike"} {
			c := row[k].(map[string]any)
			if c["kind"] != "gap" || c["reason"] != "no_data" {
				t.Fatalf("%s = %v, want a gap", k, c)
			}
			if _, has := c["value"]; has {
				t.Fatalf("%s carries a value: %v", k, c)
			}
		}
		if s := row["status"].(map[string]any); s["code"] != 2.0 || s["text"] != "no quote" {
			t.Fatalf("status = %v", s)
		}
		if row["ask"].(map[string]any)["kind"] != "value" {
			t.Fatalf("ask = %v", row["ask"])
		}
	})
	t.Run("empty read", func(t *testing.T) {
		t.Parallel()
		q := &fakeQuotes{}
		m := buildMap(t, cfg(&fakeEod{}, q))
		if m["run_status"] != "partial" || len(m["gaps"].([]any)) == 0 || len(rowsOf(m)) != 0 || len(q.calls) != 0 {
			t.Fatalf("page = %v, quote reads %d", m, len(q.calls))
		}
		if m["underlying"].(map[string]any)["price"].(map[string]any)["kind"] != "gap" {
			t.Fatal("price is not a gap")
		}
	})
	t.Run("reader errors", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("boom")
		b, err := Build(t.Context(), cfg(&fakeEod{err: boom}, &fakeQuotes{}), "2026-11-06", fixedAt)
		if !errors.Is(err, boom) || b != nil {
			t.Fatalf("eod: %v, %d bytes", err, len(b))
		}
		r := eodRow(t, 1, f64(1), "2026-11-06T21:00:00Z")
		b, err = Build(t.Context(), cfg(&fakeEod{rows: []contracts.OptionEod{r}}, &fakeQuotes{err: boom}), "2026-11-06", fixedAt)
		if !errors.Is(err, boom) || b != nil {
			t.Fatalf("quotes: %v, %d bytes", err, len(b))
		}
	})
	t.Run("bad config", func(t *testing.T) {
		t.Parallel()
		c := cfg(&fakeEod{}, &fakeQuotes{})
		c.Zone = nil
		if _, err := Build(t.Context(), c, "2026-11-06", fixedAt); err == nil {
			t.Fatal("nil zone accepted")
		}
		c = cfg(&fakeEod{}, &fakeQuotes{})
		c.Scope = "bad scope"
		if _, err := Build(t.Context(), c, "2026-11-06", fixedAt); err == nil {
			t.Fatal("bad scope accepted")
		}
	})
}
