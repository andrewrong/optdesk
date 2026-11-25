package purity

import (
	"go/ast"
	"io/fs"
	"path/filepath"
	"strings"
)

const (
	contractsPath = "github.com/andrewrong/optdesk/internal/contracts"
	sortOnly      = "Sort" // the slices.Sort* family
)

// pageDefault is the common page builder rule: the six imports and the float text.
var pageDefault = rule{
	Banned: []string{"math/big", "sort", "database/sql", "os", "internal/notify", "internal/jobs"},
	Float:  floatText,
}

// pageWith is pageDefault with more banned imports.
func pageWith(more ...string) rule {
	r := pageDefault
	r.Banned = append(append([]string{}, r.Banned...), more...)
	return r
}

var (
	slicesSort    = selBan{Pkg: "slices", Name: sortOnly, Prefix: true}
	noMathCompute = []string{"math.Sqrt", "math.Log"}
)

// table has one row for each package that held a purity_test.go. A new package is one line.
var table = []row{
	// compute
	{"internal/compute/optioncand", rule{
		Banned: []string{"os", "net/http", "database/sql", "internal/store", "internal/notify", "internal/jobs"},
		Float:  floatIdent, Clock: clockNowSleep}},
	{"internal/compute/optionfig", rule{
		Banned: []string{"os", "internal/store", "internal/jobs", "internal/notify"}, Clock: clockNow}},
	{"internal/compute/optionnotice", rule{
		Banned: []string{"os", "net/http", "database/sql", "time", "internal/store", "internal/notify"},
		Float:  floatIdent, Clock: clockNow}},
	{"internal/compute/optionshortlist", rule{
		Banned: []string{"time", "os", "net", "database/sql", "math/rand"}}},
	{"internal/compute/optiontier", rule{
		Match: matchEnds,
		Banned: []string{"math", "os", "net/http", "database/sql", "time", "internal/strategy",
			"internal/compute/optionevidence"},
		BannedSubstr: []string{"/bsm"}, Float: floatText64}},
	{"internal/compute/optiontrack", rule{
		Banned: []string{"os", "net/http", "database/sql", "internal/store", "internal/notify", "internal/jobs",
			"internal/pages"},
		Float: floatIdent, Clock: clockNowSleep}},

	// notify
	{"internal/notify", rule{
		Select:       walkTree,
		BannedSubstr: []string{"internal/pages", "internal/jobs", "optionnotice"},
		Idents: []string{"Position", "Candidate", "Cushion", "ExcessLiquidity", "AccountReader", "AccountValue",
			"OptionLedger", "NoticeResult", "CashByDate"}}},

	// pages
	{"internal/pages/advice_page", pageDefault},
	{"internal/pages/household_page", pageDefault},
	{"internal/pages/positions_page", pageDefault},
	{"internal/pages/risk_page", pageDefault},
	{"internal/pages/home_companies", withClock(pageWith("internal/pages/companies_page"), clockNow, "time.Now")},
	{"internal/pages/home_household", withClock(rule{
		Banned: []string{"math", "sort", "database/sql", "os", "internal/notify", "internal/jobs"},
		Float:  floatText}, clockNow, "time.Now")},
	{"internal/pages/base_data_page", rule{
		Banned: []string{"database/sql", "os", "net/http", "github.com/jackc/pgx"}, Clock: clockNowSinceUntil}},
	{"internal/pages/data_health_page", rule{
		Banned: []string{"database/sql", "os", "net/http", "internal/store/ops", "github.com/jackc/pgx"},
		Clock:  clockNowSinceUntil}},
	{"internal/pages/home_macro", rule{
		Banned: []string{"internal/compute/ease", "internal/pages/macro_page", "database/sql", "os", "sort",
			"internal/jobs", "internal/notify"}}},
	{"internal/pages/home_market", withClock(rule{
		Banned: []string{"internal/pages/market_page", "database/sql", "os", "internal/notify", "internal/jobs"},
		Text:   noMathCompute}, clockNow, "time.Now")},
	{"internal/pages/home_options", rule{
		Banned: []string{"math/big", "math", "strconv", "internal/pages/option_tracking_page", "database/sql", "os",
			"sort", "internal/jobs", "internal/notify", "internal/compute/optionledger"},
		Float: floatIdent, Clock: clockNowSinceUntil}},
	{"internal/pages/market_page", rule{
		Banned: []string{"internal/store/bar", "internal/jobs", "internal/compute/figures", "database/sql", "os"},
		Text:   noMathCompute}},
	{"internal/pages/market_state_page", rule{
		Match: matchEnds,
		Banned: []string{"internal/compute/marketstate", "internal/compute/liquidity", "internal/compute/figures",
			"internal/jobs", "database/sql", "os"}}},
	{"internal/pages/option_candidates_page", rule{
		Banned: []string{"math/big", "sort", "os", "net/http", "database/sql", "internal/notify", "internal/jobs"},
		Float:  floatIdent, Clock: clockNowSleep, Sels: []selBan{slicesSort},
		Require: []string{"tier"}, Extra: tierNoArithmetic}},
	{"internal/pages/option_notices_page", rule{
		Banned: []string{"sort", "os", "net/http", "database/sql", "internal/notify", "internal/jobs"},
		Float:  floatIdent, Clock: clockNowSleep, Sels: []selBan{slicesSort}}},
	{"internal/pages/option_pnl_page", rule{
		Banned: []string{"math/big", "math", "sort", "os", "net/http", "database/sql"},
		Float:  floatIdent,
		Sels:   []selBan{{Name: "Add"}, {Name: "Sub"}, {Name: "Mul"}, {Name: "Div"}},
		Extra:  marksSignOnly}},
	{"internal/pages/option_tracking_page", rule{
		Banned: []string{"math/big", "sort", "os", "net/http", "database/sql", "internal/notify", "internal/jobs"},
		Float:  floatIdent, Clock: clockNowSleep, Sels: []selBan{slicesSort},
		ClockOnly: []string{"context.go"}}},
	{"internal/pages/strategy_replay_page", rule{
		Banned: []string{"os", "time", "math", "net/http", "database/sql"}, Extra: contractsOnly}},
	{"internal/pages/valuation_import_page", rule{
		Match: matchEnds,
		Banned: []string{"database/sql", "os", "net/http", "time", "math", "internal/clock",
			"internal/store/valuationpg", "internal/store/underlyingpg", "github.com/jackc/pgx",
			"internal/compute/valueimport"},
		BannedSubstr: []string{"decimal"}}},
	{"internal/pages/valuation_page", rule{
		Match: matchEnds,
		Banned: []string{"database/sql", "os", "net/http", "internal/clock", "internal/store/valuationpg",
			"internal/store/underlyingpg", "github.com/jackc/pgx", "internal/compute/valuelines", "math"},
		BannedSubstr: []string{"decimal"},
		Sels:         []selBan{{Pkg: "strconv", Name: "ParseFloat", Prefix: true}}}},
	{"internal/pages/company_data_page", rule{
		Match:        matchEnds,
		Files:        []string{"builder.go", "rows.go", "strings.go"},
		Banned:       []string{"math", "os", "database/sql", "internal/compute/companydata"},
		BannedSubstr: []string{"decimal"}, Float: floatConv, Clock: clockNowSinceUntil}},
	{"internal/pages/company_quarters_page", rule{
		Match:        matchEnds,
		Files:        []string{"format.go", "builder.go"},
		Banned:       []string{"math", "internal/compute/companydata"},
		BannedSubstr: []string{"decimal"}, Float: floatConv,
		Sels: []selBan{{Pkg: "big"}, {Pkg: "decimal"}, {Pkg: "math"}}}},
}

// withClock sets the clock form and the text that names the clock call, comments included.
func withClock(r rule, c clockForm, text string) rule {
	r.Clock = c
	r.Text = append(append([]string{}, r.Text...), text)
	return r
}

// walkTree lists the non-test .go files of dir and of every sub-directory, relative to dir.
func walkTree(root, dir string) ([]string, error) {
	var out []string
	base := filepath.Join(root, dir)
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			rel, _ := filepath.Rel(base, p)
			out = append(out, rel)
		}
		return nil
	})
	return out, err
}

// marksSignOnly: marks.go does only sign tests on figures.
func marksSignOnly(name string, f *ast.File) []string {
	if name != "marks.go" {
		return nil
	}
	allowed := map[string]bool{"Sign": true, "IsPositive": true, "NewFromString": true}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && id.Name == "decimal" && !allowed[x.Sel.Name] {
				out = append(out, "uses decimal."+x.Sel.Name)
			}
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && !allowed[sel.Sel.Name] {
				out = append(out, "calls ."+sel.Sel.Name)
			}
		}
		return true
	})
	return out
}

// tierNoArithmetic: the tier files import no number library and call neither percentText nor roundDigits.
func tierNoArithmetic(name string, f *ast.File) []string {
	if !strings.HasPrefix(name, "tier") {
		return nil
	}
	var out []string
	for _, imp := range f.Imports {
		p := importPath(imp)
		if p == "math" || p == "math/big" || p == "sort" || strings.Contains(p, "decimal") {
			out = append(out, "imports "+p)
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && (id.Name == "percentText" || id.Name == "roundDigits") {
				out = append(out, "calls "+id.Name)
			}
		}
		return true
	})
	return out
}

// contractsOnly: an import whose first path element holds a dot must be the contracts package.
func contractsOnly(_ string, f *ast.File) []string {
	var out []string
	for _, imp := range f.Imports {
		p := importPath(imp)
		first, _, _ := strings.Cut(p, "/")
		if strings.Contains(first, ".") && p != contractsPath {
			out = append(out, "imports "+p)
		}
	}
	return out
}
