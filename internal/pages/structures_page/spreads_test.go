package structures_page

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/spreadlegs"
	"github.com/andrewrong/optdesk/internal/compute/spreadlegsapi"
)

var updateSpreads = flag.Bool("update-spreads", false, "rewrite testdata/spreads.golden.json")

const spreadsSchemaHost = "https://optdesk.invalid/schemas/"

type spreadsSchemaLoader struct{}

func (spreadsSchemaLoader) Load(url string) (any, error) {
	file, ok := strings.CutPrefix(url, spreadsSchemaHost)
	if !ok || strings.ContainsAny(file, "/\\") {
		return nil, fmt.Errorf("unexpected schema url %q", url)
	}
	b, err := os.ReadFile(filepath.Join("../../../contracts/schemas", file))
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func validateSpreads(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(spreadsSchemaLoader{})
	sch, err := c.Compile(spreadsSchemaHost + "page.spread_legs_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Fatalf("schema: %v\n%s", err, b)
	}
}

func fv(v string) compute.FigureValue { return compute.FigureValue{Value: v} }
func fg(g string) compute.FigureValue { return compute.FigureValue{Gap: g} }

func sl(right, strike, expiry, qty, open string) compute.StructureLeg {
	return compute.StructureLeg{PositionID: "p", Right: right, Strike: strike, Expiry: expiry, Quantity: qty,
		Multiplier: "100", OpenPrice: open}
}

func st(id, under, kind, cur string, loss, gain compute.FigureValue, legs ...compute.StructureLeg) compute.Structure {
	return compute.Structure{ID: id, Alias: "main", Underlying: under, Currency: cur, Kind: kind, Legs: legs,
		Figures: compute.StructureFigures{MaxLoss: loss, MaxGain: gain, BreakEven: fg("no_data")}}
}

// spreadsResult is what Evaluate gave: a put spread, an iron condor, a covered call, a single short put, a yen
// spread, a spread with a leg that has no open price, and a unit of kind other.
func spreadsResult() compute.StructureResult {
	return compute.StructureResult{
		Structures: []compute.Structure{
			st("put_spread-aaa", "AAA", "put_spread", "USD", fv("-740.0000"), fv("260.0000"),
				sl("P", "95", "2026-12-18", "-2", "2.4"), sl("P", "90", "2026-12-18", "2", "1.1")),
			st("iron_condor-bbb", "BBB", "iron_condor", "USD", fv("-190.0000"), fv("310.0000"),
				sl("P", "90", "2026-12-18", "-1", "2.5"), sl("P", "85", "2026-12-18", "1", "0.9"),
				sl("C", "110", "2026-12-18", "-1", "2.1"), sl("C", "115", "2026-12-18", "1", "0.6")),
			st("covered_call-ccc", "CCC", "covered_call", "USD", fg("no_rule"), fv("1420.0000"),
				sl("C", "120", "2026-12-18", "-1", "1.8")),
			st("short_put-eee", "EEE", "short_put", "USD", fv("-4000.0000"), fv("100.0000"),
				sl("P", "40", "2026-12-18", "-1", "1")),
			st("put_spread-jjj", "JJJ", "put_spread", "JPY", fg("unsupported_currency"), fg("unsupported_currency"),
				sl("P", "3000", "2026-12-04", "-1", "50"), sl("P", "2900", "2026-12-04", "1", "20")),
			st("put_spread-fff", "FFF", "put_spread", "USD", fg("missing_input"), fg("missing_input"),
				sl("P", "50", "2026-12-11", "-1", ""), sl("P", "45", "2026-12-11", "1", "0.5")),
		},
		Other: []compute.Structure{
			st("other-ddd", "DDD", "other", "USD", fg("no_rule"), fg("no_rule"),
				sl("P", "50", "2026-12-18", "-1", "1"), sl("C", "80", "2026-12-18", "-3", "0.5")),
		},
	}
}

func spreadsRig(res compute.StructureResult) *rig {
	r := newRig(res)
	r.cfg.Spreads = spreadlegs.New
	return r
}

func (r *rig) buildSpreadsPage(t *testing.T, alias string) []byte {
	t.Helper()
	b, err := buildSpreads(context.Background(), r.cfg, alias, "2026-11-20", testAt)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBuildSpreadsGolden(t *testing.T) {
	t.Parallel()
	got := spreadsRig(spreadsResult()).buildSpreadsPage(t, "main")
	validateSpreads(t, got)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, got, "", "  "); err != nil {
		t.Fatal(err)
	}
	pretty.WriteByte('\n')
	const path = "testdata/spreads.golden.json"
	if *updateSpreads {
		if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, pretty.Bytes()) {
		t.Fatalf("golden differs:\n%s", pretty.Bytes())
	}
}

func spreadsModelOf(t *testing.T, b []byte) spreadsModel {
	t.Helper()
	var m spreadsModel
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func blockSymbols(g spreadGroup) string {
	var s []string
	for _, b := range g.Structures {
		s = append(s, b.Symbol)
	}
	return strings.Join(s, ",")
}

func TestSpreadsGroupsAndCounts(t *testing.T) {
	t.Parallel()
	m := spreadsModelOf(t, spreadsRig(spreadsResult()).buildSpreadsPage(t, "main"))
	// the single short put is not listed; the figure orders by days to expiry: JJJ 14, FFF 21, then the 27-day ones
	if got := blockSymbols(m.Groups[0]); got != "JJJ,FFF,AAA,BBB,CCC" {
		t.Fatalf("spreads = %s", got)
	}
	if got := blockSymbols(m.Groups[1]); got != "DDD" {
		t.Fatalf("other = %s", got)
	}
	if want := spreadsSentenceHead + "5" + spreadsSentenceMid + "1" + spreadsSentenceTail; m.Sentence != want {
		t.Fatalf("sentence = %q, want %q", m.Sentence, want)
	}
	if m.Groups[1].Structures[0].Reason == nil || !strings.Contains(*m.Groups[1].Structures[0].Reason, reasonPutCallOneExpiry) {
		t.Fatalf("other reason = %v", m.Groups[1].Structures[0].Reason)
	}
	if m.Note == nil || !strings.Contains(*m.Note, spreadsNote) || m.RunStatus != "partial" {
		t.Fatalf("note %v status %s", m.Note, m.RunStatus)
	}
}

func spreadsBySymbol(t *testing.T) map[string]spreadBlock {
	t.Helper()
	m := spreadsModelOf(t, spreadsRig(spreadsResult()).buildSpreadsPage(t, "main"))
	by := map[string]spreadBlock{}
	for _, b := range m.Groups[0].Structures {
		by[b.Symbol] = b
	}
	return by
}

func TestSpreadsFiguresCopiedPutSpread(t *testing.T) {
	t.Parallel()
	aaa := spreadsBySymbol(t)["AAA"]
	if aaa.NetCredit.Text != "+260.00"+spreadsCreditUnit || aaa.MaxLoss.Text != "-740.00" || aaa.MaxGain.Text != "+260.00" ||
		aaa.BreakEven.Text != "93.70" || aaa.ExpiryText != spreadsExpiryHead+"2026-12-18"+spreadsDaysOpen+"28"+spreadsDaysClose {
		t.Errorf("put spread %+v", aaa)
	}
}

func TestSpreadsFiguresCopiedIronCondor(t *testing.T) {
	t.Parallel()
	if bbb := spreadsBySymbol(t)["BBB"]; bbb.BreakEven.Text != "86.90"+spreadsBreakJoin+"113.10" || len(bbb.BreakEven.Values) != 2 {
		t.Errorf("iron condor break-even %+v", bbb.BreakEven)
	}
}

func TestSpreadsFiguresCopiedCoveredCall(t *testing.T) {
	t.Parallel()
	ccc := spreadsBySymbol(t)["CCC"]
	if len(ccc.Legs) != 1 || ccc.MaxLoss.Kind != "gap" || ccc.MaxLoss.Text != gapTextCallMaxLoss ||
		ccc.BreakEven.Kind != "gap" || ccc.BreakEven.Text != gapTextCallBreak || ccc.MaxGain.Text != "+1,420.00" {
		t.Errorf("covered call %+v", ccc)
	}
}

func TestSpreadsFiguresCopiedGaps(t *testing.T) {
	t.Parallel()
	by := spreadsBySymbol(t)
	if jjj := by["JJJ"]; jjj.NetCredit.Reason != "unsupported_currency" || jjj.NetCredit.Text != gapTextUnsupported ||
		jjj.BreakEven.Reason != "unsupported_currency" || jjj.MaxLoss.Reason != "unsupported_currency" {
		t.Errorf("yen %+v", jjj)
	}
	if fff := by["FFF"]; fff.NetCredit.Reason != "missing_input" || fff.NetCredit.Text != gapTextMissingInput ||
		fff.Legs[0].OpenPrice != spreadsNoOpenPrice || fff.BreakEven.Reason != "missing_input" {
		t.Errorf("no open price %+v", fff)
	}
}

// spyFigure gives fixed rows and keeps its input.
type spyFigure struct {
	in   *spreadlegsapi.SpreadIn
	rows []spreadlegsapi.SpreadFigures
}

func (s spyFigure) Spreads(in spreadlegsapi.SpreadIn) spreadlegsapi.SpreadResult {
	*s.in = in
	return spreadlegsapi.SpreadResult{Rows: s.rows}
}

func TestSpreadsOrderAndDaysFromFigure(t *testing.T) {
	t.Parallel()
	big := st("big", "BIG", "put_spread", "USD", fv("-9000.0000"), fv("1000.0000"),
		sl("P", "95", "2027-01-15", "-1", "2"), sl("P", "85", "2027-01-15", "1", "1"))
	small := st("small", "SML", "put_spread", "USD", fv("-100.0000"), fv("50.0000"),
		sl("P", "95", "2026-11-27", "-1", "0.5"), sl("P", "94", "2026-11-27", "1", "0.4"))
	// Evaluate gives the largest loss first, as the structures page orders it
	r := newRig(compute.StructureResult{Structures: []compute.Structure{big, small}})
	var in spreadlegsapi.SpreadIn
	// the figure puts the near one first and says its own day counts
	r.cfg.Spreads = func() spreadlegsapi.SpreadFigure {
		return spyFigure{in: &in, rows: []spreadlegsapi.SpreadFigures{
			{ID: "small", Expiry: "2026-11-27", DaysToExpiry: 99, NetCredit: fv("10.00"), BreakEvens: []string{"94.9000"}},
			{ID: "big", Expiry: "2027-01-15", DaysToExpiry: 56, NetCredit: fv("100.00"), BreakEvens: []string{"94.0000"}},
		}}
	}
	m := spreadsModelOf(t, r.buildSpreadsPage(t, "main"))
	if got := blockSymbols(m.Groups[0]); got != "SML,BIG" {
		t.Fatalf("order = %s, want the order of the figure rows", got)
	}
	if want := spreadsExpiryHead + "2026-11-27" + spreadsDaysOpen + "99" + spreadsDaysClose; m.Groups[0].Structures[0].ExpiryText != want {
		t.Fatalf("expiry text = %q, want %q", m.Groups[0].Structures[0].ExpiryText, want)
	}
	if in.AsOf != "2026-11-20" || len(in.Structures) != 2 {
		t.Fatalf("figure input = %+v", in)
	}
}

func TestSpreadsNoReaderAndUnknownAlias(t *testing.T) {
	t.Parallel()
	r := newRig(spreadsResult())
	m := spreadsModelOf(t, r.buildSpreadsPage(t, "main")) // Spreads is nil
	if m.RunStatus != "failed" || m.Sentence != spreadsNoReader || len(m.Groups) != 2 || len(m.Groups[0].Structures) != 0 {
		t.Fatalf("nil Spreads: %+v", m)
	}
	raw := spreadsRig(spreadsResult()).buildSpreadsPage(t, "nobody")
	validateSpreads(t, raw)
	m = spreadsModelOf(t, raw)
	if m.RunStatus != "failed" || m.Sentence != spreadsUnknownAcct || len(m.Groups[1].Structures) != 0 {
		t.Fatalf("unknown alias: %+v", m)
	}
	validateSpreads(t, r.buildSpreadsPage(t, "main"))
}

func TestSpreadsEmptyAccount(t *testing.T) {
	t.Parallel()
	raw := spreadsRig(compute.StructureResult{}).buildSpreadsPage(t, "main")
	validateSpreads(t, raw)
	m := spreadsModelOf(t, raw)
	if m.Sentence != spreadsEmptyHead+"main"+spreadsEmptyTail || m.RunStatus != "ok" {
		t.Fatalf("empty: %+v", m)
	}
}

func TestSpreadsHrefIsStructureDetail(t *testing.T) {
	t.Parallel()
	m := spreadsModelOf(t, spreadsRig(spreadsResult()).buildSpreadsPage(t, "main"))
	for _, b := range m.Groups[0].Structures {
		if b.Symbol == "AAA" && b.Href != hrefOf("put_spread-aaa") {
			t.Fatalf("href = %s", b.Href)
		}
	}
}
