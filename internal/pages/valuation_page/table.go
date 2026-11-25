package valuation_page

import (
	"cmp"
	"context"
	"errors"
	"net/url"
	"slices"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

const (
	notEnteredText = "not entered"
	zoneSep        = " - "
	entryRoute     = "/valuation-form?company="
)

type rowJSON struct {
	CompanyKey  string     `json:"company_key"`
	Name        string     `json:"name"`
	Valued      bool       `json:"valued"`
	NotEntered  string     `json:"not_entered"`
	EntryLink   string     `json:"entry_link"`
	V           string     `json:"v"`
	VersionDate string     `json:"version_date"`
	R           string     `json:"r"`
	C           string     `json:"c"`
	Heavy       string     `json:"heavy"`
	BuyZone     string     `json:"buy_zone"`
	FairZone    string     `json:"fair_zone"`
	Trim        string     `json:"trim"`
	Clear       string     `json:"clear"`
	Core        string     `json:"core"`
	Close       string     `json:"close"`
	Zone        string     `json:"zone"`
	Flags       *flagsJSON `json:"flags"`
	P           string     `json:"p"`
	BuyPrice    string     `json:"buy_price"`
	Status      string     `json:"status"`
}

// BuildTable returns the JSON of page.valuation_page.v1, view table: one row per name of the union of every
// current settings.underlying row and every profile, ascending by company key. A name with no current review is a
// row with valued false, "not entered" and an entry link, and counts in unvalued_count. Each valued company is
// resolved once through Resolve. The hand-typed buy price of settings.underlying is printed beside the buy zone and
// nothing is computed from the two.
func BuildTable(ctx context.Context, in Input) ([]byte, error) {
	p, err := newPage(in, ViewTable)
	if err != nil {
		return nil, err
	}
	p.Rows = []rowJSON{}
	if in.StateOK {
		for _, e := range union(in) {
			row, count := tableRow(ctx, p, in, e)
			p.Rows = append(p.Rows, row)
			p.UnvaluedCount += count
		}
	}
	return p.marshal()
}

// namedEntry is an entry and the hand-typed buy price of its row on settings.underlying, "" when none.
type namedEntry struct {
	valuation.Entry
	buyPrice string
}

// union is every profile and every underlying name without one (a defaulted profile), ascending by key.
func union(in Input) []namedEntry {
	byKey := map[string]*namedEntry{}
	for _, e := range in.State.Entries {
		byKey[e.Profile.CompanyKey] = &namedEntry{Entry: e}
	}
	for _, r := range in.Underlying {
		if n, ok := byKey[r.Key]; ok {
			n.buyPrice = r.BuyPrice
			continue
		}
		byKey[r.Key] = &namedEntry{Profile: profileFromRow(r), buyPrice: r.BuyPrice}
	}
	out := make([]namedEntry, 0, len(byKey))
	for _, n := range byKey {
		out = append(out, *n)
	}
	slices.SortFunc(out, func(a, b namedEntry) int {
		return cmp.Compare(a.Profile.CompanyKey, b.Profile.CompanyKey)
	})
	return out
}

// tableRow is the row of one entry and 1 when it counts as unvalued.
func tableRow(ctx context.Context, p *pageJSON, in Input, e namedEntry) (rowJSON, int) {
	key := e.Profile.CompanyKey
	name := e.Profile.Name
	if name == "" {
		name = key
	}
	row := rowJSON{CompanyKey: key, Name: name, BuyPrice: e.buyPrice}
	if e.Current == nil {
		row.NotEntered = notEnteredText
		row.EntryLink = entryRoute + url.QueryEscape(key)
		return row, 1
	}
	row.Valued, row.Status = true, e.Current.Status
	row.V, row.P, row.VersionDate = e.Current.V, e.Current.P, day(e.Current.VersionDate)
	res, err := Resolve(ctx, e.Entry, in.State, in.Closes, in.Calc, in.Day)
	switch {
	case errors.Is(err, ErrNoTables):
		p.addGap("tables_missing", tablesMissingText)
		return row, 0
	case err != nil:
		return row, 0 // a stored review the calculator refuses shows its V and P and no line
	}
	if res.CloseErr != nil {
		p.addGap("close_unavailable", closeUnavailableTableText)
	}
	l := res.Lines
	c := in.Calc
	row.R, row.C = round(c, l.R, placesPrice), round(c, l.C, placesPrice)
	row.Heavy = round(c, l.Heavy, placesPrice)
	row.BuyZone = zone(c, l.BuyLow, l.BuyHigh)
	row.FairZone = zone(c, l.FairLow, l.FairHigh)
	row.Trim, row.Clear, row.Core = round(c, l.Trim, placesPrice), round(c, l.Clear, placesPrice), round(c, l.Core, placesPrice)
	row.Close = round(c, res.Close, placesPrice)
	row.Zone = l.Zone
	if l.Flags != nil {
		f := flagsJSON(*l.Flags)
		row.Flags = &f
	}
	return row, 0
}

// zone is the text of a range of two lines.
func zone(c valuelinesapi.Calculator, low, high string) string {
	return round(c, low, placesPrice) + zoneSep + round(c, high, placesPrice)
}
