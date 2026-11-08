package home_page_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/pages/home_page"
)

const schemaHost = "https://optdesk.invalid/schemas/"

var fixedNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

var slots = []string{"household", "companies", "macro", "market", "options"}

// members reads one of the overview_*.json files: an object holding the five
// member documents by slot.
func members(t *testing.T, name string) map[string]json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func inputFrom(t *testing.T, name string) home_page.Input {
	t.Helper()
	m := members(t, name)
	return home_page.Input{
		RunID: "run-test", BuildID: "build-test", Scope: "scope-test",
		Zone:  time.FixedZone("UTC+8", 8*3600),
		Title: "Home", Say: "All quiet",
		CardLinks: home_page.CardLinks{
			Macro: "/macro", Market: "/market", Household: "/household",
			Companies: "/companies", Options: "/options",
		},
		ReportLinks: []home_page.Link{{Href: "/report", Text: "Report"}},
		Household:   m["household"], Companies: m["companies"],
		Macro: m["macro"], Market: m["market"], Options: m["options"],
	}
}

func input(t *testing.T) home_page.Input { return inputFrom(t, "overview_ok") }

func setSlot(in *home_page.Input, slot string, b []byte) {
	switch slot {
	case "household":
		in.Household = b
	case "companies":
		in.Companies = b
	case "macro":
		in.Macro = b
	case "market":
		in.Market = b
	case "options":
		in.Options = b
	}
}

func getSlot(in home_page.Input, slot string) []byte {
	return map[string][]byte{"household": in.Household, "companies": in.Companies,
		"macro": in.Macro, "market": in.Market, "options": in.Options}[slot]
}

// edit returns doc with fn applied to its top-level object.
func edit(t *testing.T, doc []byte, fn func(m map[string]any)) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(doc, &m); err != nil {
		t.Fatal(err)
	}
	fn(m)
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func compose(t *testing.T, in home_page.Input) map[string]any {
	t.Helper()
	b, err := home_page.Compose(in, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	file, ok := strings.CutPrefix(url, schemaHost)
	if !ok || strings.ContainsAny(file, "/\\") {
		return nil, fmt.Errorf("unexpected schema url %q", url)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", file))
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func TestComposeValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile(schemaHost + "page.home_page.v3.json")
	if err != nil {
		t.Fatal(err)
	}
	b, err := home_page.Compose(input(t), time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(v); err != nil {
		t.Fatalf("output is invalid: %v", err)
	}
}

func TestComposeGeneratedAtInZone(t *testing.T) {
	t.Parallel()
	m := compose(t, input(t))
	if got, want := m["generated_at"], "2026-01-02T11:04:05+08:00"; got != want {
		t.Errorf("generated_at = %v, want %v", got, want)
	}
}

func TestComposeNilZone(t *testing.T) {
	t.Parallel()
	in := input(t)
	in.Zone = nil
	if _, err := home_page.Compose(in, fixedNow); err == nil {
		t.Fatal("want an error")
	}
}

func TestComposeMemberMismatch(t *testing.T) {
	t.Parallel()
	faults := map[string]func(m map[string]any){
		"schema":         func(m map[string]any) { m["schema"] = "other" },
		"schema_version": func(m map[string]any) { m["schema_version"] = 99 },
		"run_id":         func(m map[string]any) { m["run_id"] = "run-other" },
	}
	for _, slot := range slots {
		for name, fn := range faults {
			t.Run(slot+"/"+name, func(t *testing.T) {
				t.Parallel()
				in := input(t)
				setSlot(&in, slot, edit(t, getSlot(in, slot), fn))
				_, err := home_page.Compose(in, fixedNow)
				if err == nil {
					t.Fatal("want an error")
				}
				if !strings.Contains(err.Error(), slot) {
					t.Errorf("error %q does not name %q", err, slot)
				}
			})
		}
	}
}

func TestComposeRunStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		statuses map[string]string
		want     string
	}{
		{"all ok", nil, "ok"},
		{"one partial", map[string]string{"macro": "partial"}, "partial"},
		{"partial and failed", map[string]string{"macro": "partial", "options": "failed"}, "failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := input(t)
			for slot, st := range tc.statuses {
				setSlot(&in, slot, edit(t, getSlot(in, slot), func(m map[string]any) { m["run_status"] = st }))
			}
			if got := compose(t, in)["run_status"]; got != tc.want {
				t.Errorf("run_status = %v, want %v", got, tc.want)
			}
		})
	}
}

func dataTimes(pairs ...[2]string) func(m map[string]any) {
	return func(m map[string]any) {
		var l []any
		for _, p := range pairs {
			l = append(l, map[string]any{"source": p[0], "data_time": p[1]})
		}
		m["data_times"] = l
	}
}

func TestComposeDataTimes(t *testing.T) {
	t.Parallel()
	t.Run("order and dropped repeat", func(t *testing.T) {
		t.Parallel()
		in := input(t)
		set := func(slot string, pairs ...[2]string) {
			setSlot(&in, slot, edit(t, getSlot(in, slot), dataTimes(pairs...)))
		}
		set("household", [2]string{"a", "2026-01-01T00:00:00Z"}, [2]string{"b", "2026-01-01T00:00:00Z"})
		set("companies", [2]string{"a", "2026-01-01T00:00:00Z"}, [2]string{"c", "2026-01-01T00:00:00Z"})
		set("macro", [2]string{"a", "2026-01-02T00:00:00Z"})
		set("market", [2]string{"b", "2026-01-01T00:00:00Z"})
		set("options", [2]string{"d", "2026-01-03T00:00:00Z"})
		got, _ := json.Marshal(compose(t, in)["data_times"])
		want := `[{"data_time":"2026-01-01T00:00:00Z","source":"a"},{"data_time":"2026-01-01T00:00:00Z","source":"b"},` +
			`{"data_time":"2026-01-01T00:00:00Z","source":"c"},{"data_time":"2026-01-02T00:00:00Z","source":"a"},` +
			`{"data_time":"2026-01-03T00:00:00Z","source":"d"}]`
		if string(got) != want {
			t.Errorf("data_times = %s\nwant %s", got, want)
		}
	})
	t.Run("error past 20", func(t *testing.T) {
		t.Parallel()
		in := input(t)
		for i, slot := range slots {
			var pairs [][2]string
			for j := 0; j < 5; j++ {
				pairs = append(pairs, [2]string{fmt.Sprintf("s%d_%d", i, j), "2026-01-01T00:00:00Z"})
			}
			if slot == "options" {
				pairs = append(pairs, [2]string{"extra", "2026-01-01T00:00:00Z"})
			}
			setSlot(&in, slot, edit(t, getSlot(in, slot), dataTimes(pairs...)))
		}
		if _, err := home_page.Compose(in, fixedNow); err == nil {
			t.Fatal("want an error for 21 entries")
		}
		// exactly 20 is allowed.
		setSlot(&in, "options", edit(t, in.Options, dataTimes()))
		if got := compose(t, in)["data_times"].([]any); len(got) != 20 {
			t.Errorf("len = %d, want 20", len(got))
		}
	})
}

func TestComposeStaleAfter(t *testing.T) {
	t.Parallel()
	set := func(in *home_page.Input, slot string, v *string) {
		setSlot(in, slot, edit(t, getSlot(*in, slot), func(m map[string]any) {
			if v == nil {
				delete(m, "stale_after")
			} else {
				m["stale_after"] = *v
			}
		}))
	}
	late, early := "2026-03-01T00:00:00Z", "2026-02-01T09:00:00+08:00"
	t.Run("earliest of two", func(t *testing.T) {
		t.Parallel()
		in := input(t)
		for _, s := range slots {
			set(&in, s, nil)
		}
		set(&in, "macro", &late)
		set(&in, "market", &early)
		if got := compose(t, in)["stale_after"]; got != early {
			t.Errorf("stale_after = %v, want %v", got, early)
		}
	})
	t.Run("fallback from data times", func(t *testing.T) {
		t.Parallel()
		in := input(t)
		for _, s := range slots {
			set(&in, s, nil)
			setSlot(&in, s, edit(t, getSlot(in, s), dataTimes([2]string{"x", "2026-10-23T21:00:00+08:00"})))
		}
		setSlot(&in, "macro", edit(t, in.Macro, dataTimes([2]string{"x", "2026-10-22T05:00:00+08:00"})))
		got := compose(t, in)["stale_after"]
		if want := "2026-10-26T09:30:00+08:00"; got != want {
			t.Errorf("stale_after = %v, want %v", got, want)
		}
	})
	t.Run("no stale_after and no data times is an error", func(t *testing.T) {
		t.Parallel()
		in := input(t)
		for _, s := range slots {
			set(&in, s, nil)
			setSlot(&in, s, edit(t, getSlot(in, s), dataTimes()))
		}
		if _, err := home_page.Compose(in, fixedNow); err == nil {
			t.Fatal("want an error")
		}
	})
}

func compact(t *testing.T, b []byte) string {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, b); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestComposeKeepsMembers(t *testing.T) {
	t.Parallel()
	in := input(t)
	b, err := home_page.Compose(in, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	for _, slot := range slots {
		if got, want := string(out[slot]), compact(t, getSlot(in, slot)); got != want {
			t.Errorf("%s: member changed:\n got %s\nwant %s", slot, got, want)
		}
	}
}
