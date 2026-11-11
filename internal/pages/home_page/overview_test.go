package home_page_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/pages/home_page"
)

type overviewOut struct {
	GapLine  string `json:"gap_line"`
	Overview struct {
		Items []struct {
			Label, Text, Href string
		} `json:"items"`
		Gaps []struct {
			Card, Text string
		} `json:"gaps"`
	} `json:"overview"`
}

func composeOverview(t *testing.T, in home_page.Input) (overviewOut, []byte) {
	t.Helper()
	b, err := home_page.Compose(in, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	var o overviewOut
	if err := json.Unmarshal(b, &o); err != nil {
		t.Fatal(err)
	}
	return o, b
}

func validateV3(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile(schemaHost + "page.home_page.v3.json")
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

// cardText returns card.<key> of a member document.
func cardText(t *testing.T, doc []byte, key string) string {
	t.Helper()
	var m struct {
		Card map[string]any `json:"card"`
	}
	if err := json.Unmarshal(doc, &m); err != nil {
		t.Fatal(err)
	}
	s, _ := m.Card[key].(string)
	return s
}

func TestOverviewLines(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"overview_ok", "overview_options_nosay"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			in := inputFrom(t, file)
			o, b := composeOverview(t, in)
			validateV3(t, b)
			order := []struct {
				slot, href string
				doc        []byte
			}{
				{"macro", "/macro", in.Macro}, {"market", "/market", in.Market},
				{"household", "/household", in.Household},
				{"companies", "/companies", in.Companies}, {"options", "/options", in.Options},
			}
			if len(o.Overview.Items) != 5 {
				t.Fatalf("%d items, want 5", len(o.Overview.Items))
			}
			for i, w := range order {
				want := cardText(t, w.doc, "say")
				if want == "" {
					want = cardText(t, w.doc, "subtitle")
				}
				got := o.Overview.Items[i]
				if got.Text != want || want == "" {
					t.Errorf("%s: text %q, want %q", w.slot, got.Text, want)
				}
				if got.Label != cardText(t, w.doc, "title") {
					t.Errorf("%s: label %q", w.slot, got.Label)
				}
				if got.Href != w.href {
					t.Errorf("%s: href %q, want %q", w.slot, got.Href, w.href)
				}
			}
			if file == "overview_options_nosay" && o.Overview.Items[4].Text != cardText(t, in.Options, "subtitle") {
				t.Errorf("options line is not its subtitle: %q", o.Overview.Items[4].Text)
			}
			if len(o.Overview.Gaps) != 0 {
				t.Errorf("gaps = %v, want none", o.Overview.Gaps)
			}
			if o.GapLine != "No gaps today" {
				t.Errorf("gap_line = %q", o.GapLine)
			}
		})
	}
}

func TestOverviewHouseholdUsesSubtitle(t *testing.T) {
	t.Parallel()
	in := inputFrom(t, "overview_ok")
	if cardText(t, in.Household, "say") != "" {
		t.Fatal("fixture household has a say")
	}
	o, _ := composeOverview(t, in)
	if got := o.Overview.Items[2].Text; got != "Two households combined · data day 10-21" {
		t.Errorf("household text = %q", got)
	}
}

func TestOverviewGaps(t *testing.T) {
	t.Parallel()
	o, b := composeOverview(t, inputFrom(t, "overview_gaps"))
	validateV3(t, b)
	if len(o.Overview.Gaps) != 0 {
		t.Fatalf("gaps = %v, want none", o.Overview.Gaps)
	}
	if o.GapLine != "2 gaps, shown in their cards" {
		t.Errorf("gap_line = %q", o.GapLine)
	}
}

func TestOverviewManyGaps(t *testing.T) {
	t.Parallel()
	o, b := composeOverview(t, inputFrom(t, "overview_many_gaps"))
	validateV3(t, b)
	if len(o.Overview.Gaps) != 0 {
		t.Fatalf("gaps = %v, want none", o.Overview.Gaps)
	}
	if o.GapLine != "22 gaps, shown in their cards" {
		t.Errorf("gap_line = %q", o.GapLine)
	}
}

func TestOverviewHomeGap(t *testing.T) {
	t.Parallel()
	_, b := composeOverview(t, inputFrom(t, "overview_ok"))
	var raw struct {
		GapLine  string `json:"gap_line"`
		Overview struct {
			Gaps json.RawMessage `json:"gaps"`
		} `json:"overview"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw.Overview.Gaps) != "[]" {
		t.Errorf("overview.gaps = %s, want []", raw.Overview.Gaps)
	}
	if raw.GapLine != "No gaps today" {
		t.Errorf("gap_line = %q", raw.GapLine)
	}
}

func TestOverviewNoSayNoSubtitle(t *testing.T) {
	t.Parallel()
	for _, slot := range slots {
		t.Run(slot, func(t *testing.T) {
			t.Parallel()
			in := inputFrom(t, "overview_ok")
			setSlot(&in, slot, edit(t, getSlot(in, slot), func(m map[string]any) {
				card := m["card"].(map[string]any)
				delete(card, "say")
				delete(card, "subtitle")
			}))
			_, err := home_page.Compose(in, fixedNow)
			if err == nil || !strings.Contains(err.Error(), slot) {
				t.Errorf("error %v does not name %q", err, slot)
			}
		})
	}
}

func TestCardLinks(t *testing.T) {
	t.Parallel()
	t.Run("no block_links key", func(t *testing.T) {
		t.Parallel()
		_, b := composeOverview(t, input(t))
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		if _, ok := m["block_links"]; ok {
			t.Error("block_links is present")
		}
	})
	for _, bad := range []string{"", "macro", "https://x/y"} {
		t.Run("bad href "+bad, func(t *testing.T) {
			t.Parallel()
			in := input(t)
			in.CardLinks.Market = bad
			_, err := home_page.Compose(in, fixedNow)
			if err == nil || !strings.Contains(err.Error(), "market") {
				t.Errorf("error %v, want one naming market", err)
			}
		})
	}
}

func TestStaleAfterFallbackValidates(t *testing.T) {
	t.Parallel()
	in := input(t)
	for _, s := range slots {
		setSlot(&in, s, edit(t, getSlot(in, s), func(m map[string]any) { delete(m, "stale_after") }))
	}
	_, b := composeOverview(t, in)
	// The members of this input lack the stale_after their own schemas require,
	// so check the envelope with the members of the valid input put back.
	valid := input(t)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, s := range slots {
		var doc any
		if err := json.Unmarshal(getSlot(valid, s), &doc); err != nil {
			t.Fatal(err)
		}
		m[s] = doc
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	validateV3(t, b)
	if m["stale_after"] == "" {
		t.Error("stale_after empty")
	}
}
