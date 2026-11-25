package underlyings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

var fixedNow = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

func contractPath(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", "..", "contracts"}, parts...)...)
}

func at(day, hm string) time.Time {
	ts, err := time.Parse("2006-01-02 15:04", day+" "+hm)
	if err != nil {
		panic(err)
	}
	return ts
}

// fixtureRows are the six names the fixtures pin, in an order that is not the key order.
func fixtureRows() []underlying.Row {
	return []underlying.Row{
		{ID: 6, Key: "US:FFF", Symbol: "FFF", Market: "US", Kind: "etf", Name: "FFF Fund", ConID: 100006,
			Status: "active", Options: true, SetAt: at("2026-11-03", "11:30")},
		{ID: 5, Key: "US:EEE", Symbol: "EEE", Market: "US", Kind: "stock", Name: "EEE Co", Issuer: "cik:0000000005",
			Status: "unresolved", Reason: "not listed", Options: true, Fundamentals: true, SetAt: at("2026-11-04", "09:05")},
		{ID: 4, Key: "JP:DDD", Symbol: "DDD", Market: "JP", Kind: "stock", Name: "DDD Holdings", Issuer: "edinet:E00000",
			Status: "pending", Options: true, Fundamentals: true, SetAt: at("2026-11-04", "09:00")},
		{ID: 3, Key: "US:CCC", Symbol: "CCC", Market: "US", Kind: "stock", Name: "CCC Ltd", Issuer: "cik:0000000003",
			ConID: 100003, Status: "active", Fundamentals: true, BuyPrice: "45.0000", SetAt: at("2026-11-03", "11:00")},
		{ID: 2, Key: "US:BBB", Symbol: "BBB", Market: "US", Kind: "stock", Name: "BBB Inc", Issuer: "cik:0000000002",
			ConID: 100002, Status: "paused", Options: true, Fundamentals: true, Note: "paused for review", SetAt: at("2026-11-02", "10:15")},
		{ID: 1, Key: "US:AAA", Symbol: "AAA", Market: "US", Kind: "stock", Name: "AAA Corp", Issuer: "cik:0000000001",
			ConID: 100001, Status: "active", Options: true, Fundamentals: true, BuyPrice: "90.5000", SalePrice: "120.0000",
			ReportDateOverride: "2026-11-20", Note: "core holding", SetAt: at("2026-11-01", "08:30")},
	}
}

func fixtureInput() Input {
	return Input{
		Rows: fixtureRows(),
		Held: map[string]bool{"AAA": true},
		History: []underlying.Row{
			{ID: 5, Key: "US:EEE", Symbol: "EEE", Status: "unresolved", Reason: "not listed", SetAt: at("2026-11-04", "09:05")},
			{ID: 1, Key: "US:AAA", Symbol: "AAA", Status: "active", SetAt: at("2026-11-01", "08:30")},
		},
		Form: &Form{
			Expires: "2000-01-01T00:15:00Z",
			MAC:     "0000000000000000000000000000000000000000000000000000000000000000",
		},
		Now:        fixedNow,
		SessionDay: "2026-11-04",
		Zone:       time.UTC,
	}
}

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	for _, name := range []string{"common.v2.json", "page.underlyings.v2.json"} {
		f, err := os.Open(contractPath("schemas", name))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddResource("https://optdesk.invalid/schemas/"+name, doc); err != nil {
			t.Fatal(err)
		}
	}
	s, err := c.Compile("https://optdesk.invalid/schemas/page.underlyings.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validate(t *testing.T, out []byte) {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(bytesReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if err := compileSchema(t).Validate(doc); err != nil {
		t.Fatalf("schema: %v", err)
	}
}

// checkFixture builds in and compares it with the named fixture, decoded, and validates it.
// englishTexts are the English sentences the shared fixtures of contracts/ still hold, with the text the page writes
// now: the page is compared with the fixture once the sentences are put in Chinese.
var englishTexts = map[string]string{
	"Pending: resolves after the close": pendingText,
	"Followed by nothing":               followedNone,
	"Days missed while a group is off cannot be refilled: the daily history of a skipped day is not fetched later.": refillText,
	"Added: held position; waits to be resolved":                        noteHeldWaits,
	"Editing is off: start ot serve with --owner-edit to change names.": readOnlyText,
}

// localize puts the English sentences of a decoded fixture in the Chinese the page writes.
func localize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = localize(e)
		}
	case []any:
		for i, e := range x {
			x[i] = localize(e)
		}
	case string:
		if z, ok := englishTexts[x]; ok {
			return z
		}
		if rest, ok := strings.CutPrefix(x, "Followed by: "); ok {
			return followedPref + rest
		}
	}
	return v
}

func checkFixture(t *testing.T, in Input, name string) {
	t.Helper()
	out, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(contractPath("fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	if got, w := decode(t, out), localize(decode(t, want)); !reflect.DeepEqual(got, w) {
		t.Fatalf("output differs from %s:\n got %s\nwant %s", name, out, want)
	}
	validate(t, out)
}

func TestBuildGolden(t *testing.T) {
	t.Parallel()
	checkFixture(t, fixtureInput(), "page.underlyings.v2.golden.json")
}

func TestBuildConfirm(t *testing.T) {
	t.Parallel()
	in := fixtureInput()
	in.Notice = "Review the change below, then confirm."
	in.Confirm = &Confirm{
		Changes: []Change{{Label: "AAA status", Old: "active", New: "paused"}},
		Stops: []Stop{
			{Group: "bars", Symbol: "AAA", Held: true},
			{Group: "dividends", Symbol: "AAA", Held: true},
			{Group: "options", Symbol: "AAA", Stopped: true},
			{Group: "fundamentals", Symbol: "AAA", Stopped: true},
		},
		Fields: []Field{{Name: "key", Value: "US:AAA"}, {Name: "status", Value: "paused"}},
	}
	checkFixture(t, in, "page.underlyings.v2.confirm.json")
}

func TestBuildConfirmCarriesFields(t *testing.T) {
	t.Parallel()
	build := func(c *Confirm, withForm bool) error {
		in := fixtureInput()
		if !withForm {
			in.Form = nil
		}
		in.Confirm = c
		_, err := Build(in)
		return err
	}
	t.Run("two fields in order", func(t *testing.T) {
		t.Parallel()
		in := fixtureInput()
		in.Confirm = &Confirm{Fields: []Field{{Name: "note", Value: ""}, {Name: "key", Value: "US:AAA"}}}
		m := decode(t, mustBuild(t, in))
		got, _ := m["confirm"].(map[string]any)["fields"].([]any)
		want := []any{
			map[string]any{"name": "note", "value": ""},
			map[string]any{"name": "key", "value": "US:AAA"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("fields = %v, want %v", got, want)
		}
	})
	t.Run("nil fields is a list", func(t *testing.T) {
		t.Parallel()
		in := fixtureInput()
		in.Confirm = &Confirm{}
		m := decode(t, mustBuild(t, in))
		if got, ok := m["confirm"].(map[string]any)["fields"].([]any); !ok || len(got) != 0 {
			t.Errorf("fields = %v, want []", m["confirm"])
		}
	})
	t.Run("sixteen fields fail", func(t *testing.T) {
		t.Parallel()
		if err := build(&Confirm{Fields: make([]Field, 16)}, true); err == nil {
			t.Error("16 fields built")
		}
	})
	t.Run("long value fails", func(t *testing.T) {
		t.Parallel()
		f := []Field{{Name: "note", Value: strings.Repeat("x", 301)}}
		if err := build(&Confirm{Fields: f}, true); err == nil {
			t.Error("301-character value built")
		}
	})
	t.Run("no form fails", func(t *testing.T) {
		t.Parallel()
		if err := build(&Confirm{Fields: []Field{{Name: "key", Value: "US:AAA"}}}, false); err == nil {
			t.Error("confirm without form built")
		}
	})
}

func TestBuildReadOnly(t *testing.T) {
	t.Parallel()
	in := fixtureInput()
	in.Form = nil
	checkFixture(t, in, "page.underlyings.v2.readonly.json")
}

func TestBuildHistoryLimitAndDiff(t *testing.T) {
	t.Parallel()
	in := fixtureInput()
	in.History = nil
	for i := range 25 {
		in.History = append(in.History, underlying.Row{
			ID: int64(100 - i), Key: "US:AAA", Symbol: "AAA", Status: "active", Options: true,
			Note: fmt.Sprintf("n%d", 100-i), SetAt: at("2026-11-04", "09:00"),
		})
	}
	m := decode(t, mustBuild(t, in))
	h := m["history"].([]any)
	if len(h) != 20 {
		t.Fatalf("history = %d, want 20", len(h))
	}
	if got := h[0].(map[string]any)["text"]; got != "AAA: note n100" {
		t.Fatalf("text = %v", got)
	}
}

func TestBuildStaleAndNoConfirmStops(t *testing.T) {
	t.Parallel()
	in := fixtureInput()
	in.Now = at("2026-11-06", "00:00") // DDD is 39 hours old
	in.Confirm = &Confirm{Stops: []Stop{{Group: "bars", Symbol: "CCC"}}}
	m := decode(t, mustBuild(t, in))
	row := m["rows"].([]any)[0].(map[string]any)
	if row["key"] != "JP:DDD" || row["status_text"] != staleText {
		t.Fatalf("row = %v", row)
	}
	stops := m["confirm"].(map[string]any)["stops"].([]any)
	if len(stops) != 1 || stops[0] != "bars: not stopped for CCC" {
		t.Fatalf("stops = %v", stops)
	}
}

func TestBuildZoneAndConID(t *testing.T) {
	t.Parallel()
	in := fixtureInput()
	in.Zone = time.FixedZone("JST", 9*3600)
	rows := decode(t, mustBuild(t, in))["rows"].([]any)
	first := rows[0].(map[string]any)
	if first["set_at"] != "2026-11-04 18:00" || first["con_id"] != nil {
		t.Fatalf("row = %v", first)
	}
	if rows[1].(map[string]any)["con_id"] != float64(100001) {
		t.Fatalf("row = %v", rows[1])
	}
}

func mustBuild(t *testing.T, in Input) []byte {
	t.Helper()
	out, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

func heldGapTexts(t *testing.T, in Input) []string {
	t.Helper()
	out, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Gaps []gapJSON `json:"gaps"`
	}
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, g := range p.Gaps {
		if g.Reason == "no_data" {
			texts = append(texts, g.Text)
		}
	}
	return texts
}

func heldInput(rows ...underlying.Row) Input {
	return Input{Rows: rows, Now: fixedNow, Zone: time.UTC, Form: &Form{}}
}

func TestHeldNameWithoutRowIsAGap(t *testing.T) {
	t.Parallel()
	in := heldInput()
	in.Held = map[string]bool{"ZZZ": true}
	in.HeldOptions = map[string]bool{"AAA": true, "ZZZ": true}
	got := heldGapTexts(t, in)
	want := []string{"AAA: held, not on the list", "ZZZ: held, not on the list"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHeldPendingRowIsAGap(t *testing.T) {
	t.Parallel()
	in := heldInput(underlying.Row{Key: "US:AAA", Symbol: "AAA", Market: "US", Status: "pending"})
	in.Held = map[string]bool{"AAA": true}
	want := []string{"AAA: pending; the first option chain comes at least one session day after the resolve"}
	if got := heldGapTexts(t, in); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHeldUnresolvedRowIsAGap(t *testing.T) {
	t.Parallel()
	in := heldInput(underlying.Row{Key: "US:AAA", Symbol: "AAA", Market: "US", Status: "unresolved", Reason: "not listed"})
	in.HeldOptions = map[string]bool{"AAA": true}
	want := []string{"AAA: unresolved, no option chain: not listed"}
	if got := heldGapTexts(t, in); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHeldRefusedNameShowsItsReason(t *testing.T) {
	t.Parallel()
	in := heldInput()
	in.Held = map[string]bool{"BBB": true}
	in.HeldRefused = map[string]string{"BBB": "currency GBP has no market"}
	got := heldGapTexts(t, in)
	if len(got) != 1 || !strings.Contains(got[0], "BBB") || !strings.Contains(got[0], "currency GBP has no market") ||
		strings.Contains(got[0], "held, not on the list") {
		t.Fatalf("got %q", got)
	}
}

func TestHeldActiveRowGivesNoGap(t *testing.T) {
	t.Parallel()
	in := heldInput(underlying.Row{Key: "US:AAA", Symbol: "AAA", Market: "US", Status: "active"})
	in.Held = map[string]bool{"AAA": true}
	in.HeldOptions = map[string]bool{"AAA": true}
	if got := heldGapTexts(t, in); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
}

func TestHeldReadFailureIsAGap(t *testing.T) {
	t.Parallel()
	in := heldInput()
	in.HeldError = true
	want := []string{heldReadText}
	if got := heldGapTexts(t, in); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHeldTextsAtMost200Chars(t *testing.T) {
	t.Parallel()
	sym, reason := strings.Repeat("S", 20), strings.Repeat("r", 300)
	texts := map[string]string{
		"norow": heldNoRowText(sym), "pending": heldPendingText(sym),
		"unresolved": heldUnresolvedText(sym, reason), "refused": heldRefusedText(sym, reason),
	}
	for name, s := range texts {
		if n := len([]rune(s)); n > 200 || n == 0 {
			t.Errorf("%s: length %d", name, n)
		}
	}
	if !strings.Contains(texts["pending"], "at least one session day") {
		t.Errorf("pending text %q", texts["pending"])
	}
}

func TestHeldAddedRowShowsResolvedText(t *testing.T) {
	t.Parallel()
	in := heldInput(
		underlying.Row{Key: "US:AAA", Symbol: "AAA", Market: "US", Name: "AAA Corp", Status: "active", ConID: 42, Note: underlying.NoteHeld},
		underlying.Row{Key: "JP:7203", Symbol: "7203", Market: "JP", Name: "Toyota", Status: "active", ConID: 7, Note: underlying.NoteHeld},
		underlying.Row{Key: "US:BBB", Symbol: "BBB", Market: "US", Name: "BBB Inc", Status: "pending", Note: underlying.NoteHeld},
	)
	out, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	var p struct{ Rows []rowJSON }
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	notes := map[string]string{}
	for _, r := range p.Rows {
		notes[r.Key] = r.Note
	}
	if notes["US:AAA"] != "Resolved as US stock AAA, con_id 42" ||
		notes["JP:7203"] != "Resolved as JP stock 7203, con_id 7" ||
		notes["US:BBB"] != noteHeldWaits {
		t.Fatalf("notes %q", notes)
	}
}

func TestHeldRefusedLongSymbolStaysWithin200(t *testing.T) {
	t.Parallel()
	sym := strings.Repeat("S", 190)
	in := heldInput()
	in.Held = map[string]bool{sym: true}
	in.HeldRefused = map[string]string{sym: "symbol does not match the pattern of market JP"}
	got := heldGapTexts(t, in)
	if len(got) != 1 || len([]rune(got[0])) > 200 {
		t.Fatalf("got %q", got)
	}
}

func rowsOfInput(t *testing.T, rows ...underlying.Row) map[string]rowJSON {
	t.Helper()
	out, err := Build(heldInput(rows...))
	if err != nil {
		t.Fatal(err)
	}
	var p struct{ Rows []rowJSON }
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	m := map[string]rowJSON{}
	for _, r := range p.Rows {
		m[r.Key] = r
	}
	return m
}

func TestRowWithoutNameIsAGap(t *testing.T) {
	t.Parallel()
	m := rowsOfInput(t,
		underlying.Row{Key: "US:AAA", Symbol: "AAA", Market: "US", Status: "active"},
		underlying.Row{Key: "US:BBB", Symbol: "BBB", Market: "US", Name: "BBB Inc", Status: "active"},
	)
	if r := m["US:AAA"]; r.Name != zhNoName || !strings.Contains(r.Note, "No name is stored") {
		t.Errorf("no name: name %q note %q", r.Name, r.Note)
	}
	if r := m["US:BBB"]; r.Name != "BBB Inc" || strings.Contains(r.Note, "No name") {
		t.Errorf("named: name %q note %q", r.Name, r.Note)
	}
}

func TestResolvedNoteDoesNotAskToCheck(t *testing.T) {
	t.Parallel()
	m := rowsOfInput(t,
		underlying.Row{Key: "US:AAA", Symbol: "AAA", Market: "US", Name: "AAA Corp", Status: "active", ConID: 42, Note: underlying.NoteHeld},
		underlying.Row{Key: "US:BBB", Symbol: "BBB", Market: "US", Name: "BBB Inc", Status: "pending", Note: underlying.NoteHeld},
	)
	if n := m["US:AAA"].Note; n != "Resolved as US stock AAA, con_id 42" {
		t.Errorf("resolved note %q", n)
	}
	if n := m["US:BBB"].Note; !strings.Contains(n, "等待解析") || strings.Contains(n, "check the con_id") {
		t.Errorf("pending note %q", n)
	}
}

func TestRowWithoutNameValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	in := fixtureInput()
	in.Rows = append([]underlying.Row(nil), in.Rows...)
	in.Rows[0].Name = ""
	out, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, out)
}
