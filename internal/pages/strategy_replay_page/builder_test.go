package strategy_replay_page

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
	generated  = "2026-11-19T08:00:00Z"
)

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	file, ok := strings.CutPrefix(url, schemaHost)
	if !ok || strings.ContainsAny(file, "/\\") {
		return nil, fmt.Errorf("unexpected schema url %q", url)
	}
	b, err := os.ReadFile(schemaDir + file)
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
	s, err := c.Compile(schemaHost + "page.strategy_replay_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(inst); err != nil {
		t.Fatalf("schema: %v\n%s", err, b)
	}
}

func decode(t *testing.T, b []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func sampleReport() *ReportInput {
	return &ReportInput{
		AsOf: "2026-11-19", Lots: LotsShortPuts, FirstQuoteDay: "2026-11-07", LastQuoteDay: "2026-11-18",
		Variants: []Variant{{"hold", "a1b2c3d4"}, {"early", "e5f6a7b8"}},
		LotCount: 42, LotsWithQuote: 38, StillOpen: 5, LotDays: 610, QuoteDays: 8,
		Excluded: []Excluded{{Reason: "no_quote", Text: "没有任何报价", Lots: 4}},
		MinLots:  20,
		Lines: []Line{
			{Set: "all", Variant: "hold", Currency: "USD", Lots: 38, Cycles: 41, ClosedEarly: 12, Held: 26,
				Assigned: 1, Rolled: 2, InMoney: 3, KeptShare: "0.71", MeanGross: "120.50", WorstGross: "-340.00",
				HoldKeptShare: "0.71", ActualKeptShare: "0.64"},
			{Set: "all", Variant: "early", Currency: "USD", Lots: 38, Cycles: 41, ClosedEarly: 12, Held: 26,
				Assigned: 1, Rolled: 2, InMoney: 3, KeptShare: "0.78", MeanGross: "98.25", WorstGross: "-280.00",
				HoldKeptShare: "0.71", ActualKeptShare: "0.64"},
			{Set: "alias_x", Variant: "early", Currency: "USD", Lots: 7, Cycles: 10, ClosedEarly: 2, Held: 5,
				Assigned: 1, Rolled: 2, InMoney: 3, KeptShare: "0.55", WorstGross: "-90.00",
				HoldKeptShare: "0.60", UnderMinimum: true},
		},
	}
}

func emptyReport() *ReportInput {
	return &ReportInput{
		AsOf: "2026-11-19", Lots: LotsShortPuts, MinLots: 20,
		Variants: []Variant{{"hold", "a1b2c3d4"}, {"early", "e5f6a7b8"}},
	}
}

func TestBuildMatchesDocuments(t *testing.T) {
	t.Parallel()
	form := DefaultForm()
	cases := map[string]Input{
		"form":   {GeneratedAt: generated, View: ViewForm, Form: &form},
		"report": {GeneratedAt: generated, View: ViewReport, Form: &form, Report: sampleReport()},
		"empty":  {GeneratedAt: generated, View: ViewReport, Form: &form, Report: emptyReport()},
		"off":    {GeneratedAt: generated, View: ViewForm},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := Build(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			validate(t, got)
			want, err := os.ReadFile("testdata/" + name + ".json")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decode(t, got), decode(t, want)) {
				t.Errorf("document differs from testdata/%s.json:\n%s", name, got)
			}
		})
	}
}

func TestBuildNoticeKeys(t *testing.T) {
	t.Parallel()
	form := DefaultForm()
	keys := []string{NoticeFormOff, NoticeTimedOut, NoticeBusy, NoticeReadFailed, NoticeBadLots, NoticeBadAsOf,
		NoticeBadEarlyKeep, NoticeBadEarlyDTE, NoticeBadMinLots}
	seen := map[string]string{}
	for _, k := range keys {
		b, err := Build(context.Background(), Input{View: ViewForm, Form: &form, Notices: []string{k}})
		if err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		var d struct{ Notices []noticeJSON }
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatal(err)
		}
		if len(d.Notices) != 1 || d.Notices[0].Key != k || d.Notices[0].Text == "" {
			t.Fatalf("%s: notices %+v", k, d.Notices)
		}
		if other, dup := seen[d.Notices[0].Text]; dup {
			t.Errorf("%s and %s share a text", k, other)
		}
		seen[d.Notices[0].Text] = k
	}
	if _, err := Build(context.Background(), Input{View: ViewForm, Form: &form, Notices: []string{"nope"}}); err == nil {
		t.Error("unknown notice key built")
	}
}

func TestBuildRejectsBadInput(t *testing.T) {
	t.Parallel()
	form := DefaultForm()
	bad := map[string]Input{
		"view":       {View: "x", Form: &form},
		"no report":  {View: ViewReport, Form: &form},
		"lots value": {View: ViewReport, Form: &form, Report: &ReportInput{Lots: "x"}},
	}
	for name, in := range bad {
		if _, err := Build(context.Background(), in); err == nil {
			t.Errorf("%s: built", name)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Build(ctx, Input{View: ViewForm, Form: &form}); err == nil {
		t.Error("cancelled context built")
	}
}

func TestOffPageHoldsOneFormOffNotice(t *testing.T) {
	t.Parallel()
	b, err := Build(context.Background(), Input{View: ViewReport, Report: sampleReport(),
		Notices: []string{NoticeFormOff, NoticeBusy}})
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		View    string
		Form    *formJSON
		Report  *reportJSON
		Notices []noticeJSON
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	if d.View != ViewForm || d.Form != nil || d.Report != nil || len(d.Notices) != 2 || d.Notices[0].Key != NoticeFormOff {
		t.Errorf("off page: %+v", d)
	}
}
