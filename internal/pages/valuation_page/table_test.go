package valuation_page

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/underlying"
	"github.com/andrewrong/optdesk/internal/store/valuation"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
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

// validate checks b against page.valuation_page.v1 and returns it decoded.
func validate(t *testing.T, b []byte) map[string]any {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile(schemaHost + "page.valuation_page.v1.json")
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
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func baseInput(calc *fakeCalc) Input {
	return Input{
		RunID: "run-1", Now: time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC), Day: testDay,
		StateOK: true, Calc: calc, State: madeUpState(),
	}
}

func rowsOf(m map[string]any) []map[string]any {
	var out []map[string]any
	for _, r := range m["rows"].([]any) {
		out = append(out, r.(map[string]any))
	}
	return out
}

func TestUnvaluedCompaniesListed(t *testing.T) {
	t.Parallel()
	in := baseInput(newFake())
	valued := entryWith("US:AAA", "AAA", "US", "100", "3")
	bare := entryWith("US:BBB", "BBB", "US", "", "")
	bare.Current = nil
	in.State.Entries = []valuation.Entry{valued, bare}
	b, err := BuildTable(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	rows := rowsOf(validate(t, b))
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0]["valued"] != true || rows[0]["not_entered"] != "" {
		t.Errorf("valued row = %v", rows[0])
	}
	if rows[1]["valued"] != false || rows[1]["not_entered"] != "not entered" || !strings.Contains(rows[1]["entry_link"].(string), "US%3ABBB") {
		t.Errorf("unvalued row = %v", rows[1])
	}
}

func TestNamesWithoutProfileListed(t *testing.T) {
	t.Parallel()
	in := baseInput(newFake())
	in.State.Entries = []valuation.Entry{entryWith("US:AAA", "AAA", "US", "100", "3"), entryWith("HK:ONLY", "", "HKCN", "50", "3")}
	in.Underlying = []underlying.Row{
		{Key: "JP:CCC", Symbol: "CCC", Market: "JP", Currency: "JPY", Name: "Made Up C"},
		{Key: "US:AAA", Symbol: "AAA", Market: "US", Currency: "USD", Name: "Made Up A"},
	}
	b, err := BuildTable(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	m := validate(t, b)
	rows := rowsOf(m)
	keys := []string{}
	for _, r := range rows {
		keys = append(keys, r["company_key"].(string))
	}
	if strings.Join(keys, ",") != "HK:ONLY,JP:CCC,US:AAA" {
		t.Fatalf("keys = %v, want the list names and the profile with no list row, by key", keys)
	}
	c := rows[1]
	if c["valued"] != false || c["not_entered"] != "not entered" || c["name"] != "Made Up C" || !strings.Contains(c["entry_link"].(string), "JP%3ACCC") {
		t.Errorf("defaulted row = %v", c)
	}
	if m["unvalued_count"] != float64(1) {
		t.Errorf("unvalued_count = %v", m["unvalued_count"])
	}
}

func TestRowsUseOneResolvePerCompany(t *testing.T) {
	t.Parallel()
	calc := newFake()
	in := baseInput(calc)
	unvalued := entryWith("US:CCC", "CCC", "US", "", "")
	unvalued.Current = nil
	in.State.Entries = []valuation.Entry{entryWith("US:AAA", "AAA", "US", "100", "3"), entryWith("US:BBB", "BBB", "US", "80", "2"), unvalued}
	b, err := BuildTable(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
	if calc.callCount() != 2 {
		t.Errorf("Lines called %d times, want 2 (one per valued company)", calc.callCount())
	}
	r := rowsOf(validate(t, b))[0]
	if r["c"] != "90.00" || r["buy_zone"] != "69.3 - 73.8" || r["zone"] != "buy" || r["flags"] == nil {
		t.Errorf("row = %v", r)
	}
}

func TestHandBuyPriceNextToBuyHigh(t *testing.T) {
	t.Parallel()
	calc := newFake()
	in := baseInput(calc)
	in.State.Entries = []valuation.Entry{entryWith("US:AAA", "AAA", "US", "100", "3")}
	in.Underlying = []underlying.Row{{Key: "US:AAA", Symbol: "AAA", Market: "US", BuyPrice: "71.25"}}
	b, err := BuildTable(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	r := rowsOf(validate(t, b))[0]
	if r["buy_price"] != "71.25" || !strings.HasSuffix(r["buy_zone"].(string), "73.8") {
		t.Errorf("row = %v", r)
	}
}

func TestUnvaluedCount(t *testing.T) {
	t.Parallel()
	in := baseInput(newFake())
	a := entryWith("US:AAA", "AAA", "US", "", "")
	a.Current = nil
	in.State.Entries = []valuation.Entry{a, entryWith("US:BBB", "BBB", "US", "100", "3")}
	in.Underlying = []underlying.Row{{Key: "US:DDD", Symbol: "DDD", Market: "US"}, {Key: "JP:EEE", Symbol: "EEE", Market: "JP"}}
	b, err := BuildTable(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if got := validate(t, b)["unvalued_count"]; got != float64(3) {
		t.Errorf("unvalued_count = %v, want 3 (one profile without review, two names without profile)", got)
	}
}

func TestTableStoreUnavailableIsGap(t *testing.T) {
	t.Parallel()
	in := baseInput(newFake())
	in.StateOK = false
	b, err := BuildTable(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	m := validate(t, b)
	if len(m["gaps"].([]any)) != 1 || len(m["rows"].([]any)) != 0 {
		t.Errorf("doc = %v", m)
	}
}
