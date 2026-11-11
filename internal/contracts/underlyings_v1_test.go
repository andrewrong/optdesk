package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func loadUnderlyingsV1(t *testing.T, name string) map[string]any {
	t.Helper()
	return readRaw(t, optionPageFixturesDir, "page.underlyings.v1."+name+".json")
}

func underlyingsRows(m map[string]any) []map[string]any {
	var out []map[string]any
	raw, _ := m["rows"].([]any)
	for _, x := range raw {
		out = append(out, x.(map[string]any))
	}
	return out
}

// TestUnderlyingsV1Fixtures validates golden, confirm and readonly, shows bad-version fails at
// /schema_version, and rejects inline mutations.
func TestUnderlyingsV1Fixtures(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, "page.underlyings.v1.json")
	for _, name := range []string{"golden", "confirm", "readonly"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := s.Validate(loadUnderlyingsV1(t, name)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}
	t.Run("bad-version", func(t *testing.T) {
		t.Parallel()
		err := s.Validate(loadUnderlyingsV1(t, "bad-version"))
		ve, ok := err.(*jsonschema.ValidationError)
		if !ok {
			t.Fatalf("err = %v, want a validation error", err)
		}
		var got []leaf
		leaves(ve, &got)
		if len(got) != 1 || got[0] != (leaf{"/schema_version", "const"}) {
			t.Errorf("failures = %v, want one /schema_version const", got)
		}
	})
	setRow := func(m map[string]any, market, symbol, key string) {
		r := underlyingsRows(m)[0]
		r["market"], r["symbol"], r["key"] = market, symbol, key
	}
	accepted := map[string]func(m map[string]any){
		"us-space":       func(m map[string]any) { setRow(m, "US", "BRK B", "US:BRK B") },
		"us-dot":         func(m map[string]any) { setRow(m, "US", "BRK.B", "US:BRK.B") },
		"us-plain":       func(m map[string]any) { setRow(m, "US", "AAPL", "US:AAPL") },
		"jp-letter":      func(m map[string]any) { setRow(m, "JP", "278A", "JP:278A") },
		"jp-letter-130A": func(m map[string]any) { setRow(m, "JP", "130A", "JP:130A") },
		"jp-digits":      func(m map[string]any) { setRow(m, "JP", "7203", "JP:7203") },
		"two-rows": func(m map[string]any) {
			rows := underlyingsRows(m)
			if len(rows) < 2 {
				t.Fatal("golden has fewer than two rows")
			}
			setRow(m, "US", "BRK B", "US:BRK B")
			rows[1]["market"], rows[1]["symbol"], rows[1]["key"] = "JP", "278A", "JP:278A"
		},
	}
	for name, mutate := range accepted {
		t.Run("accepts-"+name, func(t *testing.T) {
			t.Parallel()
			m := loadUnderlyingsV1(t, "golden")
			mutate(m)
			if err := s.Validate(m); err != nil {
				t.Errorf("page is invalid: %v", err)
			}
		})
	}
	mutations := map[string]func(m map[string]any){
		"double-space":     func(m map[string]any) { setRow(m, "US", "BRK  B", "US:BRK  B") },
		"leading-space":    func(m map[string]any) { setRow(m, "US", " BRK B", "US: BRK B") },
		"trailing-space":   func(m map[string]any) { setRow(m, "US", "BRK B ", "US:BRK B ") },
		"three-parts":      func(m map[string]any) { setRow(m, "US", "A B C", "US:A B C") },
		"lower-us":         func(m map[string]any) { setRow(m, "US", "brk b", "US:brk b") },
		"lower-jp":         func(m map[string]any) { setRow(m, "JP", "278a", "JP:278a") },
		"symbol-too-long":  func(m map[string]any) { setRow(m, "US", "ABCDEFGHIJKLM", "US:ABCDEFGHIJKLM") },
		"key-other-market": func(m map[string]any) { setRow(m, "US", "AAPL", "HK:AAPL") },
		"unknown-status":   func(m map[string]any) { underlyingsRows(m)[0]["status"] = "gone" },
		"unknown-followed-by": func(m map[string]any) {
			underlyingsRows(m)[0]["followed_by"] = []any{"bars", "news"}
		},
		"extra-top-level": func(m map[string]any) { m["extra"] = true },
		"extra-row-field": func(m map[string]any) { underlyingsRows(m)[0]["extra"] = 1 },
	}
	for name, mutate := range mutations {
		t.Run("mutation-"+name, func(t *testing.T) {
			t.Parallel()
			m := loadUnderlyingsV1(t, "golden")
			mutate(m)
			if s.Validate(m) == nil {
				t.Error("mutated page is valid")
			}
		})
	}
}

// TestUnderlyingsV1Content pins the content of golden, confirm and readonly that the design names.
func TestUnderlyingsV1Content(t *testing.T) {
	t.Parallel()
	t.Run("golden-names", func(t *testing.T) {
		t.Parallel()
		got := map[string]map[string]any{}
		for _, r := range underlyingsRows(loadUnderlyingsV1(t, "golden")) {
			got[r["symbol"].(string)] = r
		}
		if len(got) != 6 {
			t.Fatalf("%d names, want 6", len(got))
		}
		for sym, want := range map[string]string{"AAA": "active", "BBB": "paused", "CCC": "active", "DDD": "pending", "EEE": "unresolved", "FFF": "active"} {
			if got[sym]["status"] != want {
				t.Errorf("%s status = %v, want %s", sym, got[sym]["status"], want)
			}
		}
		if got["AAA"]["held"] != true || got["CCC"]["options"] != false || got["DDD"]["market"] != "JP" ||
			got["EEE"]["reason"] != "not listed" || got["FFF"]["kind"] != "etf" || got["FFF"]["issuer"] != "" ||
			got["FFF"]["fundamentals"] != false {
			t.Errorf("golden names do not carry the pinned attributes")
		}
	})
	t.Run("confirm-stops", func(t *testing.T) {
		t.Parallel()
		c, _ := loadUnderlyingsV1(t, "confirm")["confirm"].(map[string]any)
		stops, _ := c["stops"].([]any)
		var bars, options string
		for _, x := range stops {
			switch l := x.(string); l[:min(len(l), 5)] {
			case "bars:":
				bars = l
			case "optio":
				options = l
			}
		}
		if bars != "bars: not stopped for AAA (held)" || options != "options: stopped for AAA" {
			t.Errorf("stops = %v", stops)
		}
	})
	t.Run("readonly", func(t *testing.T) {
		t.Parallel()
		m := loadUnderlyingsV1(t, "readonly")
		gaps, _ := m["gaps"].([]any)
		if m["form"] != nil || len(gaps) != 1 || gaps[0].(map[string]any)["reason"] != "no_reader" {
			t.Errorf("form %v gaps %v, want null form and one no_reader gap", m["form"], gaps)
		}
	})
}

// TestUnderlyingsFollowTable checks the follow table covers every status times both switches, issuer and held.
func TestUnderlyingsFollowTable(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join(optionPageFixturesDir, "underlyings.follow.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Status       string   `json:"status"`
		Options      bool     `json:"options"`
		Fundamentals bool     `json:"fundamentals"`
		IssuerSet    bool     `json:"issuer_set"`
		Held         bool     `json:"held"`
		FollowedBy   []string `json:"followed_by"`
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 24 {
		t.Fatalf("%d cases, want at least 24", len(cases))
	}
	type key struct {
		st         string
		o, f, i, h bool
	}
	seen := map[key]bool{}
	for _, c := range cases {
		seen[key{c.Status, c.Options, c.Fundamentals, c.IssuerSet, c.Held}] = true
		if c.FollowedBy == nil {
			t.Errorf("%+v: followed_by is null, want a list", c)
		}
		// Decision 6: only an active name is followed by options and fundamentals; a held paused or
		// retired name keeps bars; pending and unresolved are followed by nothing.
		want := []string{}
		bars := c.Status == "active" || ((c.Status == "paused" || c.Status == "retired") && c.Held)
		if bars {
			want = append(want, "bars")
			if c.IssuerSet {
				want = append(want, "dividends")
			}
		}
		if c.Status == "active" && c.Options {
			want = append(want, "options")
		}
		if c.Status == "active" && c.Fundamentals && c.IssuerSet {
			want = append(want, "fundamentals")
		}
		if !slices.Equal(c.FollowedBy, want) {
			t.Errorf("%+v: followed_by %v, want %v", c, c.FollowedBy, want)
		}
	}
	for _, st := range []string{"pending", "active", "paused", "retired", "unresolved"} {
		for n := 0; n < 16; n++ {
			k := key{st, n&1 != 0, n&2 != 0, n&4 != 0, n&8 != 0}
			if !seen[k] {
				t.Errorf("no case for %+v", k)
			}
		}
	}
}
