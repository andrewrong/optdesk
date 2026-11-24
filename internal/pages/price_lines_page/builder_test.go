package price_lines_page

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

var fixedNow = time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)

func base() Input {
	return Input{RunID: "run1", BuildID: "b1", Scope: "price_lines", Reader: true, Now: fixedNow}
}

func row(market, symbol, price, status, note string) underlying.Row {
	return underlying.Row{Key: market + ":" + symbol, Market: market, Symbol: symbol, BuyPrice: price, Status: status, Note: note}
}

type outRow struct {
	Symbol, Market string
	Price, Note    *string
}

type out struct {
	Schema        string
	SchemaVersion int    `json:"schema_version"`
	GeneratedAt   string `json:"generated_at"`
	RunStatus     string `json:"run_status"`
	Stale         *string
	Notes         []string
	Gaps          []struct{ Kind, Reason, Text string }
	Saved, Error  *string
	Form          struct {
		PostPath string `json:"post_path"`
		Expires  string
		MAC      string
	}
	Rows  []outRow
	Unset []outRow
	Add   struct {
		Markets []string
		Hint    string
	}
	Title, Sentence, Intro, Footnote string
}

func build(t *testing.T, in Input) (out, []byte) {
	t.Helper()
	b, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	var o out
	if err := json.Unmarshal(b, &o); err != nil {
		t.Fatal(err)
	}
	return o, b
}

func TestBuildRowsSorted(t *testing.T) {
	t.Parallel()
	in := base()
	in.Rows = []underlying.Row{
		row("US", "MSFT", "300", "active", "core"),
		row("JP", "7203", "2500", "paused", ""),
		row("US", "AAPL", "150.5", "retired", ""),
		row("US", "NVDA", "", "active", ""),
	}
	o, _ := build(t, in)
	if len(o.Rows) != 3 {
		t.Fatalf("rows = %+v", o.Rows)
	}
	want := []string{"JP:7203", "US:AAPL", "US:MSFT"}
	for i, r := range o.Rows {
		if r.Market+":"+r.Symbol != want[i] {
			t.Fatalf("row %d = %s:%s", i, r.Market, r.Symbol)
		}
	}
	if o.Rows[0].Note != nil || *o.Rows[0].Price != "2500" {
		t.Fatalf("row 0 = %+v", o.Rows[0])
	}
	if o.Rows[2].Note == nil || *o.Rows[2].Note != "core" {
		t.Fatalf("row 2 = %+v", o.Rows[2])
	}
}

func TestBuildUnsetSkipsPriced(t *testing.T) {
	t.Parallel()
	in := base()
	in.Held = map[string]bool{"AAPL": true, "MSFT": true, "NVDA": true}
	in.Rows = []underlying.Row{
		row("US", "NVDA", "", "active", ""),
		row("US", "AAPL", "150", "active", ""),
		row("US", "AMD", "", "active", ""), // not held
		row("US", "MSFT", "", "pending", ""),
	}
	o, _ := build(t, in)
	if len(o.Unset) != 2 || o.Unset[0].Symbol != "NVDA" || o.Unset[1].Symbol != "MSFT" {
		t.Fatalf("unset = %+v", o.Unset)
	}
}

func TestBuildUnsetSkipsRetired(t *testing.T) {
	t.Parallel()
	in := base()
	in.Held = map[string]bool{"AAPL": true}
	in.Rows = []underlying.Row{row("US", "AAPL", "", underlying.StatusRetired, "")}
	o, _ := build(t, in)
	if len(o.Unset) != 0 {
		t.Fatalf("unset = %+v", o.Unset)
	}
}

func TestBuildNoReaderGap(t *testing.T) {
	t.Parallel()
	in := base()
	in.Reader = false
	in.Held = map[string]bool{"AAPL": true}
	in.Rows = []underlying.Row{row("US", "AAPL", "1", "active", ""), row("US", "MSFT", "", "active", "")}
	o, _ := build(t, in)
	if o.RunStatus != "partial" || len(o.Gaps) != 1 || o.Gaps[0].Reason != "no_reader" || o.Gaps[0].Kind != "gap" {
		t.Fatalf("status %s gaps %+v", o.RunStatus, o.Gaps)
	}
	if len(o.Rows) != 0 || len(o.Unset) != 0 {
		t.Fatalf("rows %+v unset %+v", o.Rows, o.Unset)
	}
}

func TestBuildFixedFields(t *testing.T) {
	t.Parallel()
	in := base()
	o, _ := build(t, in)
	in.Saved, in.Error = "Saved 1 row(s).", ""
	if o.RunStatus != "ok" || len(o.Gaps) != 0 || o.Stale != nil || len(o.Notes) != 0 {
		t.Fatalf("%+v", o)
	}
	o2, _ := build(t, in)
	if o2.Saved == nil || *o2.Saved != "Saved 1 row(s)." || o2.Error != nil {
		t.Fatalf("saved %v error %v", o2.Saved, o2.Error)
	}
	if o.Form.PostPath != "/plan/price-lines" || o.Form.Expires != "0" || o.Form.MAC != string(bytes.Repeat([]byte("0"), 64)) {
		t.Fatalf("form %+v", o.Form)
	}
	if o.GeneratedAt != "2026-11-14T09:00:00Z" {
		t.Fatalf("generated_at = %s", o.GeneratedAt)
	}
	in.Form = &Form{Expires: "17", MAC: string(bytes.Repeat([]byte("a"), 64))}
	o, _ = build(t, in)
	if o.Form.Expires != "17" || o.Form.MAC[0] != 'a' {
		t.Fatalf("form %+v", o.Form)
	}
}

const (
	sentenceWant = "为每个持有的标的手填买入价。只有没有估值买入区间的标的，卖出认沽的闸门才会读取这个价格。"
	introWant    = "保存的价格在下一次期权候选运行时生效；清除手填价格，只会取消没有估值的标的的检查。"
)

func TestSentenceNamesValuationException(t *testing.T) {
	t.Parallel()
	o, _ := build(t, base())
	if !strings.Contains(o.Sentence, "只有没有估值买入区间的标的") {
		t.Errorf("sentence = %q", o.Sentence)
	}
	if !strings.Contains(o.Intro, "只会取消没有估值的标的的检查") {
		t.Errorf("intro = %q", o.Intro)
	}
}

func TestPriceLinesTextsAreChinese(t *testing.T) {
	t.Parallel()
	o, _ := build(t, base())
	old := []string{
		"Your hand buy price", "A saved price applies", "Type the code and market", "The price is the buy_price",
	}
	for name, text := range map[string]string{"sentence": o.Sentence, "intro": o.Intro, "add.hint": o.Add.Hint, "footnote": o.Footnote} {
		if !strings.ContainsFunc(text, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
			t.Errorf("%s holds no Chinese character: %q", name, text)
		}
		for _, e := range old {
			if strings.Contains(text, e) {
				t.Errorf("%s holds the old English sentence %q", name, e)
			}
		}
	}
	if !strings.Contains(o.Footnote, "buy_price") {
		t.Errorf("footnote lost the field name: %q", o.Footnote)
	}
}

func TestBuildFixedTexts(t *testing.T) {
	t.Parallel()
	o, _ := build(t, base())
	for name, p := range map[string][2]string{
		"title":    {o.Title, "Price lines"},
		"sentence": {o.Sentence, sentenceWant},
		"intro":    {o.Intro, introWant},
		"add.hint": {o.Add.Hint, "请输入标的清单中某个标的的代码和市场。"},
		"footnote": {o.Footnote, "该价格即标的页面上该标的的 buy_price；备注与标的页面共用。"},
	} {
		if p[0] != p[1] {
			t.Errorf("%s = %q, want %q", name, p[0], p[1])
		}
	}
	if len(o.Add.Markets) != 2 || o.Add.Markets[0] != "US" || o.Add.Markets[1] != "JP" {
		t.Fatalf("markets = %v", o.Add.Markets)
	}
	if o.Saved != nil || o.Error != nil {
		t.Fatalf("empty saved/error not null: %v %v", o.Saved, o.Error)
	}
}

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	for _, name := range []string{"common.v2.json", "page.price_lines_page.v1.json"} {
		f, err := os.Open(filepath.Join("..", "..", "..", "contracts", "schemas", name))
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
	s, err := c.Compile("https://optdesk.invalid/schemas/page.price_lines_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestBuildValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	s := compileSchema(t)
	full := base()
	full.Held = map[string]bool{"NVDA": true}
	full.Rows = []underlying.Row{row("US", "AAPL", "150", "active", "n"), row("US", "NVDA", "", "active", "")}
	full.Saved, full.Error = "Saved 1 row(s).", "bad"
	full.Form = &Form{Expires: "1789000000", MAC: string(bytes.Repeat([]byte("f"), 64))}
	noReader := base()
	noReader.Reader = false
	for name, in := range map[string]Input{"full": full, "no reader": noReader, "empty": base()} {
		_, b := build(t, in)
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Validate(doc); err != nil {
			t.Fatalf("%s: %v\n%s", name, err, b)
		}
	}
}
