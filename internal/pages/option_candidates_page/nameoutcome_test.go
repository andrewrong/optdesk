package option_candidates_page

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optioncand"
	"github.com/andrewrong/optdesk/internal/compute/optioncandapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// fakeOpen holds the stored open rows of the day and the days it was asked for.
type fakeOpen struct {
	rows  []contracts.OptionOpen
	err   error
	asked []contracts.Day
}

func (f *fakeOpen) OptionOpen(_ context.Context, day contracts.Day) ([]contracts.OptionOpen, error) {
	f.asked = append(f.asked, day)
	return f.rows, f.err
}

func openRows(ul int, currency string, status int, code string, n int) []contracts.OptionOpen {
	out := make([]contracts.OptionOpen, n)
	for i := range out {
		out[i] = contracts.OptionOpen{UnderlyingID: ul, Currency: currency, Status: status}
		if code != "" {
			c := code
			out[i].ErrorCode = &c
		}
	}
	return out
}

// jpNamesConfig is the yen page of the three names with rows (7203, 6954, 6758), plus 2413 (11 NO_QUOTE rows),
// 2408 (28 status 7 rows with NO_DATA), 9999 (no row, and a USD row that must not count) on the watch list.
func jpNamesConfig(t *testing.T) (Config, *fakeOpen) {
	t.Helper()
	c, _ := jpConfig(t)
	c.Names = &namesFake{names: map[int]string{21: "7203", 22: "6954", 23: "6758", 31: "2413", 32: "2408", 34: "9999"}}
	var rows []contracts.OptionOpen
	rows = append(rows, openRows(21, "JPY", 1, "", 3)...)
	rows = append(rows, openRows(31, "JPY", 3, "NO_QUOTE", 11)...)
	rows = append(rows, openRows(32, "JPY", 7, "NO_DATA", 28)...)
	rows = append(rows, openRows(34, "USD", 1, "", 2)...)
	open := &fakeOpen{rows: rows}
	c.OpenRows = open
	c.NameOpen = optioncand.NewNameOpen
	c.JPNames = []string{"7203", "6954", "6758", "2413", "2408", "9999"}
	return c, open
}

type namesOut struct {
	Groups []struct {
		Kind         string          `json:"kind"`
		NameOutcomes json.RawMessage `json:"name_outcomes"`
	} `json:"groups"`
}

func TestJPNameOutcomes(t *testing.T) {
	t.Parallel()
	c, open := jpNamesConfig(t)
	_, b := buildJP(t, c)
	var p namesOut
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if len(open.asked) != 1 || open.asked[0] != jpDay {
		t.Errorf("open rows asked for %v, want the Tokyo day %s", open.asked, jpDay)
	}
	for i, g := range p.Groups {
		if i != 1 && g.NameOutcomes != nil {
			t.Errorf("group %d has name_outcomes", i)
		}
	}
	want, err := os.ReadFile("testdata/jp_names.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(compact(t, want), compact(t, p.Groups[1].NameOutcomes)) {
		t.Errorf("name_outcomes = %s, differs from testdata/jp_names.golden.json", p.Groups[1].NameOutcomes)
	}
	if strings.Contains(string(p.Groups[1].NameOutcomes), "出错\"") {
		t.Error("no_quote claims a failure")
	}
}

// TestJPNameOutcomesNoRowsGroup: a yen group with no candidate row at all still says why each name has none.
func TestJPNameOutcomesNoRowsGroup(t *testing.T) {
	t.Parallel()
	c, _ := jpNamesConfig(t)
	c.CandCur = &fakeCur{}
	p, b := buildJP(t, c)
	jp := p.Groups[1]
	if jp.Gap == nil || !strings.Contains(string(b), `"name":"2413"`) || !strings.Contains(string(b), `"name":"7203"`) {
		t.Errorf("yen group %+v in %s", jp, b)
	}
}

func TestJPNameOutcomesNilReader(t *testing.T) {
	t.Parallel()
	for name, mod := range map[string]func(*Config){
		"reader": func(c *Config) { c.OpenRows = nil },
		"rule":   func(c *Config) { c.NameOpen = nil },
		"names":  func(c *Config) { c.JPNames = nil },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, _ := jpNamesConfig(t)
			mod(&c)
			p, b := buildJP(t, c)
			if bytes.Contains(b, []byte("name_outcomes")) || p.Groups[1].Gap != nil || p.Groups[1].Main == nil {
				t.Errorf("%s missing: %s", name, b)
			}
		})
	}
}

func TestJPNameOutcomesReadError(t *testing.T) {
	t.Parallel()
	c, open := jpNamesConfig(t)
	open.err = errors.New("down")
	if _, err := Build(context.Background(), c, nil, usDay, jpNow); err == nil {
		t.Error("a failed open-row read is an error of Build")
	}
}

func TestOutcomeTextCodes(t *testing.T) {
	t.Parallel()
	facts := []optioncandapi.OpenFact{{Status: 7, ErrorCode: "NO_DATA"}, {Status: 7, ErrorCode: "TIMEOUT"}, {Status: 7, ErrorCode: "NO_DATA"}}
	if got := outcomeText(optioncand.NewNameOpen(), optioncandapi.OutcomeNoSpot, facts); got != "没读到标的现价（NO_DATA、TIMEOUT），所以没有询价" {
		t.Errorf("no_spot = %q", got)
	}
	other := []optioncandapi.OpenFact{{Status: 5}, {Status: 3, ErrorCode: "NO_QUOTE"}}
	if got := outcomeText(optioncand.NewNameOpen(), optioncandapi.OutcomeOther, other); got != "其他状态（5）" {
		t.Errorf("other = %q", got)
	}
}

func TestOutcomeTextNoSpotSkipsOutsideBand(t *testing.T) {
	t.Parallel()
	facts := []optioncandapi.OpenFact{{Status: 7, ErrorCode: "TIMEOUT"}, {Status: 7, ErrorCode: "NO_DATA", OutsideBand: true}}
	res := optioncand.NewNameOpen().Classify(optioncandapi.NameOpenFacts{Name: "x", Facts: facts})
	if res.Outcome != optioncandapi.OutcomeNoSpot || res.NoSpot != 1 || res.Deferred != 1 {
		t.Fatalf("rule %+v", res)
	}
	if got := outcomeText(optioncand.NewNameOpen(), res.Outcome, facts); got != "没读到标的现价（TIMEOUT），所以没有询价" {
		t.Errorf("no_spot = %q", got)
	}
}
