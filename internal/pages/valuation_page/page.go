// Package valuation_page builds the page.valuation_page.v1 view model in its three views, table, company and
// form, from the stored rows and the valuelinesapi.Calculator. The builders compute nothing: every figure is a text
// the calculator wrote, shown through its RoundText. They read no store implementation and no clock.
// Design: docs/design/valuation-structure.md#6 valuation-pages.
package valuation_page

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/underlying"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// The views of the document.
const (
	ViewTable   = "table"
	ViewCompany = "company"
	ViewForm    = "form"
)

// The owner post path of the three views.
const postPath = "/valuation-post"

// Decimal places of a displayed figure: a price and an R, a rate of the margin row.
const (
	placesPrice = 2
	placesRate  = 4
)

const maxText = 200

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Form is the owner token of the form, made by the caller; nil in Input is read-only (form null).
type Form struct{ Expires, MAC string }

// Typed is the review and profile fields as the owner typed them on the form; every field text, "" when empty.
type Typed struct {
	Type, VersionDate, V, P              string
	Key, Name, Symbol, Currency, RMarket string
}

// Input is everything the three builders read; each view reads the fields it names.
type Input struct {
	RunID string
	Now   time.Time     // built_at; the page reads no clock
	Day   contracts.Day // the day of the close price window

	State   valuation.State
	StateOK bool // false: the store is unavailable
	Calc    valuelinesapi.Calculator
	Closes  CloseReader // nil: no close price

	Underlying []underlying.Row // table, company and form: underlying.Reader.Current

	Company string             // company, form: the company key
	Reviews []valuation.Review // company: every row of the company, newest first
	Typed   Typed              // form: the typed fields

	Form         *Form  // nil: form null
	Saved, Error string // empty: none
}

type gapJSON struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type formJSON struct {
	PostPath string `json:"post_path"`
	Expires  string `json:"expires"`
	MAC      string `json:"mac"`
}

type problemJSON struct {
	Field string `json:"field"`
	Text  string `json:"text"`
}

type flagsJSON struct {
	AtHeavy bool `json:"at_heavy"`
	InBuy   bool `json:"in_buy"`
	InFair  bool `json:"in_fair"`
	AtTrim  bool `json:"at_trim"`
	AtClear bool `json:"at_clear"`
	AtCore  bool `json:"at_core"`
}

type linesJSON struct {
	C        string     `json:"c"`
	R        string     `json:"r"`
	Heavy    string     `json:"heavy"`
	BuyLow   string     `json:"buy_low"`
	BuyHigh  string     `json:"buy_high"`
	FairLow  string     `json:"fair_low"`
	FairHigh string     `json:"fair_high"`
	Trim     string     `json:"trim"`
	Clear    string     `json:"clear"`
	Core     string     `json:"core"`
	M        string     `json:"m"`
	W        string     `json:"w"`
	H        string     `json:"h"`
	S        string     `json:"s"`
	Zone     string     `json:"zone"`
	ZoneNote string     `json:"zone_note"`
	Flags    *flagsJSON `json:"flags"`
}

type markJSON struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Pos   string `json:"pos"`
}

type bandJSON struct {
	Min   string     `json:"min"`
	Max   string     `json:"max"`
	Marks []markJSON `json:"marks"`
}

type closeJSON struct {
	Value string `json:"value"`
	Day   string `json:"day"`
}

type companyJSON struct {
	CompanyKey string `json:"company_key"`
	Name       string `json:"name"`
	Symbol     string `json:"symbol"`
	Currency   string `json:"currency"`
	RMarket    string `json:"r_market"`
	Pool       string `json:"pool"`
	PoolDate   string `json:"pool_date"`
	Note       string `json:"note"`
}

type pageJSON struct {
	Schema        string        `json:"schema"`
	SchemaVersion int           `json:"schema_version"`
	RunID         string        `json:"run_id"`
	BuiltAt       string        `json:"built_at"`
	View          string        `json:"view"`
	Rows          []rowJSON     `json:"rows"`
	UnvaluedCount int           `json:"unvalued_count"`
	Company       *companyJSON  `json:"company"`
	Current       *currentJSON  `json:"current"`
	Lines         *linesJSON    `json:"lines"`
	Band          *bandJSON     `json:"band"`
	Close         *closeJSON    `json:"close"`
	History       []historyJSON `json:"history"`
	Required      []string      `json:"required"`
	Problems      []problemJSON `json:"problems"`
	Error         string        `json:"error"`
	Saved         string        `json:"saved"`
	Gaps          []gapJSON     `json:"gaps"`
	Form          *formJSON     `json:"form"`
}

// newPage is the empty document of a view: every list empty, every nullable null.
func newPage(in Input, view string) (*pageJSON, error) {
	if !idPattern.MatchString(in.RunID) {
		return nil, fmt.Errorf("valuation_page: bad run_id %q", in.RunID)
	}
	if in.Calc == nil {
		return nil, fmt.Errorf("valuation_page: no calculator")
	}
	p := &pageJSON{
		Schema: "valuation_page", SchemaVersion: 1, RunID: in.RunID,
		BuiltAt: in.Now.UTC().Format(time.RFC3339), View: view,
		Required: []string{}, Problems: []problemJSON{}, Gaps: []gapJSON{},
		Error: cut(in.Error), Saved: cut(in.Saved),
	}
	if in.Form != nil {
		p.Form = &formJSON{PostPath: postPath, Expires: in.Form.Expires, MAC: in.Form.MAC}
	}
	if !in.StateOK {
		p.addGap("store_unavailable", "The valuation store is not readable here.")
	}
	return p, nil
}

func (p *pageJSON) addGap(reason, text string) {
	for _, g := range p.Gaps {
		if g.Reason == reason {
			return
		}
	}
	p.Gaps = append(p.Gaps, gapJSON{Kind: "gap", Reason: reason, Text: text})
}

func (p *pageJSON) marshal() ([]byte, error) {
	out, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("valuation_page: %w", err)
	}
	return out, nil
}

// cut is s cut to the 200 characters of the schema's short texts.
func cut(s string) string {
	if r := []rune(s); len(r) > maxText {
		return string(r[:maxText])
	}
	return s
}

// round writes a figure through the calculator, "" stays "".
func round(c valuelinesapi.Calculator, s string, places int) string {
	if s == "" {
		return ""
	}
	return c.RoundText(s, places)
}

func day(d contracts.Day) string { return string(d) }

func linesDoc(c valuelinesapi.Calculator, l valuelinesapi.Lines) *linesJSON {
	r := func(s string) string { return round(c, s, placesPrice) }
	f := func(s string) string { return round(c, s, placesRate) }
	out := &linesJSON{
		C: r(l.C), R: r(l.R), Heavy: r(l.Heavy), BuyLow: r(l.BuyLow), BuyHigh: r(l.BuyHigh),
		FairLow: r(l.FairLow), FairHigh: r(l.FairHigh), Trim: r(l.Trim), Clear: r(l.Clear), Core: r(l.Core),
		M: f(l.M), W: f(l.W), H: f(l.H), S: f(l.S), Zone: l.Zone, ZoneNote: l.ZoneNote,
	}
	if l.Flags != nil {
		f := flagsJSON(*l.Flags)
		out.Flags = &f
	}
	return out
}

func bandDoc(c valuelinesapi.Calculator, b valuelinesapi.Band) *bandJSON {
	out := &bandJSON{Min: round(c, b.Min, placesPrice), Max: round(c, b.Max, placesPrice), Marks: []markJSON{}}
	for _, m := range b.Marks {
		out.Marks = append(out.Marks, markJSON{Name: m.Name, Value: round(c, m.Value, placesPrice), Pos: m.Pos})
	}
	return out
}

func companyDoc(p valuation.Profile) *companyJSON {
	name := p.Name
	if name == "" {
		name = p.CompanyKey
	}
	return &companyJSON{
		CompanyKey: p.CompanyKey, Name: name, Symbol: p.Symbol, Currency: p.Currency, RMarket: p.RMarket,
		Pool: p.Pool, PoolDate: day(p.PoolDate), Note: p.Note,
	}
}

// profileFromRow is the profile defaulted from a name of settings.underlying (Decision 10).
func profileFromRow(r underlying.Row) valuation.Profile {
	name := r.Name
	if name == "" {
		name = r.Symbol
	}
	market := valuelinesapi.MarketUS
	if m, ok := MarketDefault(r.Key); ok {
		market = m
	}
	return valuation.Profile{
		CompanyKey: r.Key, Name: name, Symbol: r.Symbol, Currency: r.Currency, RMarket: market, Pool: "in_pool",
	}
}

// MarketDefault is the R market of a company key by its market prefix: US to US, JP to JP. Any other key (a Hong
// Kong or A-share name typed on the form) has no default and the owner must choose (ok false).
func MarketDefault(key string) (market string, ok bool) {
	switch {
	case strings.HasPrefix(key, "US:"):
		return valuelinesapi.MarketUS, true
	case strings.HasPrefix(key, "JP:"):
		return valuelinesapi.MarketJP, true
	}
	return "", false
}
