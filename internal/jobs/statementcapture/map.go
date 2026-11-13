package statementcapture

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	levelSummary = "SUMMARY"
	levelLot     = "LOT"
	costPlaces   = 6
)

// typedError is an error with a kind; its text names a day and an attribute, never a value.
type typedError struct {
	kind contracts.ErrorKind
	text string
}

func (e *typedError) Error() string             { return e.text }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func refuse(day, attr, why string) error {
	return &typedError{kind: contracts.Permanent, text: "statementcapture: day " + day + ": attribute " + attr + " " + why}
}

// row is a stored section row with its attributes decoded.
type row struct {
	key string
	at  time.Time
	a   map[string]string
}

// decodeAttrs reads the JSON object of a row; a number is kept as its text.
func decodeAttrs(text string) (map[string]string, error) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		switch t := v.(type) {
		case string:
			out[k] = t
		case json.Number:
			out[k] = t.String()
		}
	}
	return out, nil
}

// isoDate reads YYYYMMDD or YYYY-MM-DD and returns YYYY-MM-DD; any other text is refused.
func isoDate(s string) (string, bool) {
	layout := "20060102"
	if len(s) == len(time.DateOnly) {
		layout = time.DateOnly
	}
	t, err := time.Parse(layout, s)
	if err != nil {
		return "", false
	}
	return t.Format(time.DateOnly), true
}

func (r row) text(day, name string) (string, error) {
	v := r.a[name]
	if v == "" {
		return "", refuse(day, name, "is missing")
	}
	return v, nil
}

func (r row) dec(day, name string) (decimal.Decimal, error) {
	v, err := r.text(day, name)
	if err != nil {
		return decimal.Decimal{}, err
	}
	d, err := decimal.NewFromString(v)
	if err != nil {
		return decimal.Decimal{}, refuse(day, name, "is not a decimal")
	}
	return d, nil
}

func (r row) date(day, name string) (string, error) {
	v, err := r.text(day, name)
	if err != nil {
		return "", err
	}
	d, ok := isoDate(v)
	if !ok {
		return "", refuse(day, name, "is not a date of the form YYYYMMDD or YYYY-MM-DD")
	}
	return d, nil
}

// mappedPosition is a position of the capture with the mark price of the statement.
type mappedPosition struct {
	pos  store.Position
	mark string
}

// counts of what the mapping left out.
type skipped struct {
	category, level, zero int
}

// mappedDay is what one report day gives.
type mappedDay struct {
	positions []mappedPosition
	values    []store.AccountValue
	skipped   skipped
}

// conidRows holds the rows of one conid by level.
type conidRows struct {
	conid   int64
	summary []row
	lots    []row
}

// groupByConid splits the position rows of a day by conid and level, in conid order; rows of another level are counted.
func groupByConid(day string, rows []row, sk *skipped) ([]conidRows, error) {
	sorted := append([]row(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].key < sorted[j].key })
	byID := map[int64]*conidRows{}
	for _, r := range sorted {
		level, err := r.text(day, "levelOfDetail")
		if err != nil {
			return nil, err
		}
		raw, err := r.text(day, "conid")
		if err != nil {
			return nil, err
		}
		id, perr := strconv.ParseInt(raw, 10, 64)
		if perr != nil {
			return nil, refuse(day, "conid", "is not an integer")
		}
		g := byID[id]
		if g == nil {
			g = &conidRows{conid: id}
			byID[id] = g
		}
		switch level {
		case levelSummary:
			g.summary = append(g.summary, r)
		case levelLot:
			g.lots = append(g.lots, r)
		default:
			sk.level++
		}
	}
	out := make([]conidRows, 0, len(byID))
	for _, g := range byID {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].conid < out[j].conid })
	return out, nil
}

// unit is the quantity and unit cost of a conid after its rows are combined.
type unit struct {
	first    row
	quantity decimal.Decimal
	cost     decimal.Decimal
	zero     bool
}

// combine applies the level rule: a SUMMARY row alone, else the lots combined.
func combine(day string, g conidRows) (unit, error) {
	if len(g.summary) > 0 {
		r := g.summary[0]
		q, err := r.dec(day, "position")
		if err != nil {
			return unit{}, err
		}
		c, err := r.dec(day, "costBasisPrice")
		if err != nil {
			return unit{}, err
		}
		return unit{first: r, quantity: q, cost: c, zero: q.IsZero()}, nil
	}
	if len(g.lots) == 0 {
		return unit{zero: true}, nil
	}
	first := g.lots[0]
	sum, weighted := decimal.Zero, decimal.Zero
	for _, l := range g.lots {
		q, err := l.dec(day, "position")
		if err != nil {
			return unit{}, err
		}
		c, err := l.dec(day, "costBasisPrice")
		if err != nil {
			return unit{}, err
		}
		if err := sameLot(day, first, l); err != nil {
			return unit{}, err
		}
		sum, weighted = sum.Add(q), weighted.Add(q.Mul(c))
	}
	if sum.IsZero() {
		return unit{first: first, zero: true}, nil
	}
	return unit{first: first, quantity: sum, cost: weighted.DivRound(sum, costPlaces)}, nil
}

// sameLot refuses lots of one conid that differ in currency or mark price.
func sameLot(day string, a, b row) error {
	if a.a["currency"] != b.a["currency"] {
		return refuse(day, "currency", "differs between the lots of one conid")
	}
	pa, err := a.dec(day, "markPrice")
	if err != nil {
		return err
	}
	pb, err := b.dec(day, "markPrice")
	if err != nil {
		return err
	}
	if !pa.Equal(pb) {
		return refuse(day, "markPrice", "differs between the lots of one conid")
	}
	return nil
}

// mapPosition maps one conid; the bool is false when it is left out (counted in sk).
func mapPosition(day string, g conidRows, sk *skipped) (mappedPosition, bool, error) {
	u, err := combine(day, g)
	if err != nil {
		return mappedPosition{}, false, err
	}
	if u.zero && u.first.a == nil {
		return mappedPosition{}, false, nil
	}
	cat, err := u.first.text(day, "assetCategory")
	if err != nil {
		return mappedPosition{}, false, err
	}
	if cat != "STK" && cat != "OPT" {
		sk.category++
		return mappedPosition{}, false, nil
	}
	if u.zero {
		sk.zero++
		return mappedPosition{}, false, nil
	}
	p := store.Position{ConID: g.conid, Quantity: u.quantity.String(), SecType: "stock", Multiplier: "1"}
	if p.Currency, err = u.first.text(day, "currency"); err != nil {
		return mappedPosition{}, false, err
	}
	mark, err := u.first.dec(day, "markPrice")
	if err != nil {
		return mappedPosition{}, false, err
	}
	avg := u.cost
	if cat == "STK" {
		p.Symbol, err = u.first.text(day, "symbol")
	} else {
		avg, err = optionFields(day, u, &p)
	}
	if err != nil {
		return mappedPosition{}, false, err
	}
	p.AvgCost = avg.String()
	return mappedPosition{pos: p, mark: mark.String()}, true, nil
}

// optionFields fills the option attributes of p and returns the cost per contract.
func optionFields(day string, u unit, p *store.Position) (decimal.Decimal, error) {
	var err error
	p.SecType = "option"
	if p.Symbol, err = u.first.text(day, "underlyingSymbol"); err != nil {
		return decimal.Zero, err
	}
	mult, err := u.first.dec(day, "multiplier")
	if err != nil {
		return decimal.Zero, err
	}
	strike, err := u.first.dec(day, "strike")
	if err != nil {
		return decimal.Zero, err
	}
	if p.Expiry, err = u.first.date(day, "expiry"); err != nil {
		return decimal.Zero, err
	}
	pc, err := u.first.text(day, "putCall")
	if err != nil {
		return decimal.Zero, err
	}
	switch pc {
	case "P":
		p.Right = "put"
	case "C":
		p.Right = "call"
	default:
		return decimal.Zero, refuse(day, "putCall", "is not P or C")
	}
	p.Multiplier, p.Strike = mult.String(), strike.String()
	return u.cost.Mul(mult), nil
}

// mapDay maps the position rows and the EquitySummary row of one report day; base is the base currency.
func mapDay(day string, positions []row, equity row, base string) (mappedDay, error) {
	var out mappedDay
	groups, err := groupByConid(day, positions, &out.skipped)
	if err != nil {
		return mappedDay{}, err
	}
	for _, g := range groups {
		mp, ok, err := mapPosition(day, g, &out.skipped)
		if err != nil {
			return mappedDay{}, err
		}
		if ok {
			out.positions = append(out.positions, mp)
		}
	}
	total, err := equity.dec(day, "total")
	if err != nil {
		return mappedDay{}, err
	}
	cash, err := equity.dec(day, "cash")
	if err != nil {
		return mappedDay{}, err
	}
	out.values = []store.AccountValue{
		{Tag: "NetLiquidation", Value: total.String(), Currency: base},
		{Tag: "TotalCashValue", Value: cash.String(), Currency: base},
	}
	return out, nil
}

// baseCurrency is the one currency of the ChangeInNAV rows; none or two different ones is a refusal of the day.
func baseCurrency(day string, rows []row) (string, error) {
	cur := ""
	for _, r := range rows {
		c, err := r.text(day, "currency")
		if err != nil {
			return "", err
		}
		if cur != "" && c != cur {
			return "", refuse(day, "currency", "of ChangeInNAV is not one base currency")
		}
		cur = c
	}
	if cur == "" {
		return "", refuse(day, "currency", "of ChangeInNAV is missing: no base currency")
	}
	return cur, nil
}
