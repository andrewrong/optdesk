package optionresult

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// pos builds a closed position: qty is signed, premium the opening premium per share times
// multiplier 100 comes from price.
func pos(id, status, cur string, qty, price, realized, closedOn string) compute.LedgerPosition {
	return compute.LedgerPosition{
		PositionID: id, Status: status, Currency: cur, Quantity: qty, Multiplier: "100",
		ClosedOn: contracts.Day(closedOn), RealizedPnL: realized,
		Fills:         []compute.LedgerFill{{Action: "open", Quantity: qty, Price: price}},
		PremiumIncome: realized,
	}
}

func summarise(ps ...compute.LedgerPosition) compute.ResultSummary {
	return New().Summarise(compute.ResultInput{Alias: "a", Positions: ps, AsOf: "2026-03-15"})
}

func TestKeptShare(t *testing.T) {
	t.Parallel()
	got := summarise(pos("p1", "closed", "USD", "-1", "2", "150.0000", "2026-01-20"))
	s := got.Sections[0]
	if s.Premium != "200.0000" || s.Realized != "150.0000" || s.KeptShare != "0.750000" {
		t.Fatalf("section %+v", s)
	}
	if b := summarise(pos("p2", "closed", "USD", "1", "2", "150.0000", "2026-01-20")); b.Sections[0].KeptShare != "" {
		t.Fatalf("bought kept share %q", b.Sections[0].KeptShare)
	}
	if z := summarise(pos("p3", "closed", "USD", "-1", "0", "5", "2026-01-20")); z.Sections[0].KeptShare != "" {
		t.Fatalf("zero premium kept share %q", z.Sections[0].KeptShare)
	}
}

func TestRoundingHalfEven(t *testing.T) {
	t.Parallel()
	cases := map[string]string{"0.12345": "0.1234", "0.12355": "0.1236", "-0.12345": "-0.1234", "1": "1.0000", "-0.00001": "0.0000"}
	for in, want := range cases {
		got := summarise(pos("p", "closed", "USD", "-1", "1", in, "2026-01-20")).Sections[0].Realized
		if got != want {
			t.Errorf("%s: got %s want %s", in, got, want)
		}
	}
}

func TestCurrenciesNeverAdded(t *testing.T) {
	t.Parallel()
	got := summarise(
		pos("u", "closed", "USD", "-1", "2", "100", "2026-01-20"),
		pos("j", "closed", "JPY", "-1", "300", "20000", "2026-01-21"),
	)
	if len(got.Sections) != 2 || got.Sections[0].Currency != "JPY" || got.Sections[1].Currency != "USD" {
		t.Fatalf("sections %+v", got.Sections)
	}
	if got.Sections[0].Realized != "20000.0000" || got.Sections[1].Realized != "100.0000" {
		t.Fatalf("realized %q %q", got.Sections[0].Realized, got.Sections[1].Realized)
	}
	for _, s := range got.Sections {
		if strings.Contains(s.Realized, "20100") || len(s.Months) != 1 || s.Months[0].Running != s.Realized {
			t.Fatalf("section mixes currencies: %+v", s)
		}
	}
}

func TestSideSplit(t *testing.T) {
	t.Parallel()
	got := summarise(
		pos("b", "closed", "USD", "1", "2", "-50", "2026-01-20"),
		pos("s", "closed", "USD", "-2", "2", "100", "2026-01-20"),
	)
	if len(got.Sections) != 2 || got.Sections[0].Side != "sold" || got.Sections[1].Side != "bought" {
		t.Fatalf("sections %+v", got.Sections)
	}
	if got.Sections[0].Premium != "400.0000" || got.Sections[1].Realized != "-50.0000" || got.Sections[1].Premium != "200.0000" {
		t.Fatalf("figures %+v", got.Sections)
	}
}

func TestEndingsByStatus(t *testing.T) {
	t.Parallel()
	got := summarise(
		pos("1", "closed", "USD", "-1", "2", "10", "2026-01-20"),
		pos("2", "closed", "USD", "-1", "2", "5.5", "2026-01-21"),
		pos("3", "expired", "USD", "-1", "2", "200", "2026-02-20"),
		pos("4", "assigned", "USD", "-1", "2", "150", "2026-02-20"),
	)
	want := []compute.ResultEnding{
		{Status: "closed", Count: 2, Realized: "15.5000"},
		{Status: "expired", Count: 1, Realized: "200.0000"},
		{Status: "assigned", Count: 1, Realized: "150.0000"},
		{Status: "exercised", Count: 0, Realized: "0.0000"},
	}
	if !reflect.DeepEqual(got.Sections[0].Endings, want) {
		t.Fatalf("endings %+v", got.Sections[0].Endings)
	}
}

func TestAssignedPremiumApart(t *testing.T) {
	t.Parallel()
	put := pos("p", "assigned", "USD", "-2", "2.5", "0", "2026-02-20")
	put.Right, put.Strike, put.PremiumIncome = "P", "50", "500.0000"
	got := summarise(put, pos("c", "closed", "USD", "-1", "1", "30", "2026-02-01"))
	s := got.Sections[0]
	if s.AssignedPremium != "500.0000" || s.AssignedShares != "200.0000" {
		t.Fatalf("assigned %q %q", s.AssignedPremium, s.AssignedShares)
	}
	for _, f := range reflect.VisibleFields(reflect.TypeOf(s)) {
		n := strings.ToLower(f.Name)
		if strings.Contains(n, "net") || strings.Contains(n, "basis") || strings.Contains(n, "minus") {
			t.Fatalf("field %s", f.Name)
		}
	}
}

func TestBasisNotReducedByPremium(t *testing.T) {
	t.Parallel()
	put := pos("p", "assigned", "USD", "-1", "2", "0", "2026-02-20")
	put.Right, put.Strike, put.PremiumIncome = "P", "50", "200.0000"
	s := summarise(put).Sections[0]
	// strike 50 less premium 2 per share would be 48: no figure may carry it, nor 4800.
	for _, v := range []string{s.Realized, s.Premium, s.KeptShare, s.AssignedPremium, s.AssignedShares} {
		if v == "48.0000" || v == "4800.0000" || v == "48" {
			t.Fatalf("figure %q is premium minus strike", v)
		}
	}
	if s.AssignedShares != "100.0000" {
		t.Fatalf("shares %q", s.AssignedShares)
	}
}

func TestOpenPositionsIgnored(t *testing.T) {
	t.Parallel()
	open := pos("o", "open", "USD", "-1", "2", "", "")
	noFigure := pos("n", "closed", "USD", "-1", "2", "", "2026-01-20")
	got := summarise(open, noFigure)
	if len(got.Sections) != 0 {
		t.Fatalf("sections %+v", got.Sections)
	}
	got = summarise(open, pos("c", "closed", "USD", "-1", "2", "10", "2026-01-20"))
	if len(got.Sections) != 1 || got.Sections[0].Reliability.NoBroker != 1 {
		t.Fatalf("sections %+v", got.Sections)
	}
}

func TestEmptyInputNoSection(t *testing.T) {
	t.Parallel()
	got := New().Summarise(compute.ResultInput{Alias: "a", Unresolved: []string{"x"}})
	if got.Sections != nil {
		t.Fatalf("sections %+v", got.Sections)
	}
}

type mixedFixture struct {
	Input    compute.ResultInput
	Expected compute.ResultSummary
}

func TestMixedEndingsFixture(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/mixed_endings.json")
	if err != nil {
		t.Fatal(err)
	}
	var f mixedFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if got := New().Summarise(f.Input); !reflect.DeepEqual(got, f.Expected) {
		g, _ := json.MarshalIndent(got, "", " ")
		t.Fatalf("got %s", g)
	}
}

func TestNoFloatNoClockNoIO(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{"float32", "float64", "time.Now", "time.Sleep"}
	imports := []string{"os", "net/http", "database/sql", "internal/store", "internal/notify", "internal/pages", "internal/jobs"}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range banned {
			if strings.Contains(string(src), b) {
				t.Errorf("%s uses %s", name, b)
			}
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range f.Imports {
			p := strings.Trim(im.Path.Value, `"`)
			for _, bad := range imports {
				if p == bad || strings.HasSuffix(p, "/"+bad) {
					t.Errorf("%s imports %s", name, p)
				}
			}
		}
	}
}
