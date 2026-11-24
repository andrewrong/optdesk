package valuation_page

import (
	"context"
	"errors"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// requiredFields are the four values a review that is not a draft needs (Decision 1).
var requiredFields = []string{"type", "version_date", "v", "p"}

// BuildForm returns the JSON of page.valuation_page.v1, view form: the profile the entry will use, the preview of
// the typed V and P as `lines` (one Calculator.Lines call through Resolve), the four required fields and the
// problems of the typed record. Problems are listed and nothing is refused. The profile is the stored one, else
// the one defaulted from the list row of the key, else built from the typed fields of a free key.
func BuildForm(ctx context.Context, in Input) ([]byte, error) {
	p, err := newPage(in, ViewForm)
	if err != nil {
		return nil, err
	}
	p.Required = append(p.Required, requiredFields...)
	key := in.Company
	if key == "" {
		key = in.Typed.Key
	}
	prof, known := formProfile(in, key)
	if prof.CompanyKey != "" {
		p.Company = companyDoc(prof)
	}
	if !known && key != "" {
		if _, ok := MarketDefault(key); !ok && in.Typed.RMarket == "" {
			p.Problems = append(p.Problems, problemJSON{Field: "r_market", Text: chooseMarketText})
		}
	}
	t := in.Typed
	if t.Type != "" || t.VersionDate != "" || t.V != "" || t.P != "" {
		p.Problems = append(p.Problems, check(in)...)
	}
	if t.V != "" && t.P != "" && prof.CompanyKey != "" {
		preview(ctx, p, in, prof)
	}
	return p.marshal()
}

// formProfile is the profile of key and whether a stored one exists.
func formProfile(in Input, key string) (valuation.Profile, bool) {
	if key == "" {
		return valuation.Profile{}, false
	}
	for _, e := range in.State.Entries {
		if e.Profile.CompanyKey == key {
			return e.Profile, true
		}
	}
	for _, r := range in.Underlying {
		if r.Key == key {
			return profileFromRow(r), false
		}
	}
	// a free key: the typed fields, the market from the key or the choice
	market, ok := MarketDefault(key)
	if !ok {
		market = in.Typed.RMarket
	}
	if market == "" {
		market = valuelinesapi.MarketHKCN
	}
	return valuation.Profile{
		CompanyKey: key, Name: in.Typed.Name, Symbol: in.Typed.Symbol, Currency: orDefault(in.Typed.Currency, "USD"),
		RMarket: market, Pool: "in_pool",
	}, false
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// check is the problems of the typed record as the calculator lists them (Decision 12).
func check(in Input) []problemJSON {
	var margin []valuelinesapi.MarginRow
	if in.State.Margin != nil {
		for _, r := range in.State.Margin.Rows {
			margin = append(margin, valuelinesapi.MarginRow{PFrom: r.PFrom, M: r.M, W: r.W, H: r.H, S: r.S})
		}
	}
	t := in.Typed
	var out []problemJSON
	for _, pr := range in.Calc.Check(valuelinesapi.CheckInput{Type: t.Type, VersionDate: t.VersionDate, V: t.V, P: t.P, Margin: margin}) {
		out = append(out, problemJSON{Field: pr.Field, Text: pr.Text})
	}
	return out
}

// preview writes lines, band and close of the typed V and P through Resolve; a V or P the calculator refuses
// leaves lines null, the check above says why.
func preview(ctx context.Context, p *pageJSON, in Input, prof valuation.Profile) {
	e := valuation.Entry{Profile: prof, Current: &valuation.Review{V: in.Typed.V, P: in.Typed.P}}
	res, err := Resolve(ctx, e, in.State, in.Closes, in.Calc, in.Day)
	switch {
	case errors.Is(err, ErrNoTables):
		p.addGap("tables_missing", tablesMissingText)
		return
	case err != nil || res == nil:
		return
	}
	p.Lines = linesDoc(in.Calc, res.Lines)
	p.Band = bandDoc(in.Calc, res.Band)
}
