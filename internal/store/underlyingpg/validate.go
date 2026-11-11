package underlyingpg

import (
	"math/big"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// The rules of docs/design/underlyings.md Decision 3. This file is pure: no database, no clock, so the page route
// and the import share it.
var (
	// usSymbol allows one inner single space (`BRK B`, as IB holds it); ValidateAdd also limits it to 12 characters.
	usSymbol = regexp.MustCompile(`^[A-Z0-9.]+( [A-Z0-9.]+)?$`)
	// jpSymbol: TSE codes issued since 2024 may carry a letter in the fourth place (130A).
	jpSymbol  = regexp.MustCompile(`^[0-9]{3}[0-9A-Z]$`)
	exchange  = regexp.MustCompile(`^[A-Z0-9._-]{1,16}$`)
	currency  = regexp.MustCompile(`^[A-Z]{3}$`)
	issuerRe  = regexp.MustCompile(`^(cik:[0-9]{10}|edinet:[A-Za-z0-9]{1,16})$`)
	priceText = regexp.MustCompile(`^[0-9]+(\.[0-9]{1,4})?$`)
)

const (
	maxName = 80
	maxNote = 200
)

// invalidError is the unexported implementation of underlying.InvalidError.
type invalidError struct{ field, why string }

var _ underlying.InvalidError = (*invalidError)(nil)

func (e *invalidError) Error() string { return "underlying: " + e.field + ": " + e.why }
func (e *invalidError) Field() string { return e.field }
func (e *invalidError) Why() string   { return e.why }

func invalid(field, why string) error { return &invalidError{field: field, why: why} }

// ValidateAdd checks a new name and returns the row to insert: pending, or active when it carries a ConID (the
// import). An empty Exchange and Currency take the default of the market. exists says whether a key already has a row; a name_key is never added twice.
func ValidateAdd(a underlying.Add, exists func(key string) bool) (underlying.Row, error) {
	if a.Market != "US" && a.Market != "JP" {
		return underlying.Row{}, invalid("market", "must be US or JP")
	}
	sym := usSymbol
	if a.Market == "JP" {
		sym = jpSymbol
	}
	if !sym.MatchString(a.Symbol) || (a.Market == "US" && len(a.Symbol) > 12) {
		return underlying.Row{}, invalid("symbol", "does not match the pattern of market "+a.Market)
	}
	if a.Kind != underlying.KindStock && a.Kind != underlying.KindETF {
		return underlying.Row{}, invalid("kind", "must be stock or etf")
	}
	ex, cur := a.Exchange, a.Currency
	if ex == "" {
		ex = map[string]string{"US": "SMART", "JP": "TSEJ"}[a.Market]
	}
	if cur == "" {
		cur = map[string]string{"US": "USD", "JP": "JPY"}[a.Market]
	}
	if a.ConID < 0 {
		return underlying.Row{}, invalid("con_id", "must be positive")
	}
	key := a.Market + ":" + a.Symbol
	if exists != nil && exists(key) {
		return underlying.Row{}, invalid("symbol", "name "+key+" exists")
	}
	r := underlying.Row{
		Key: key, Symbol: a.Symbol, Market: a.Market, Kind: a.Kind, Exchange: ex, Currency: cur,
		Name: a.Name, Issuer: a.Issuer, ConID: a.ConID, Status: underlying.StatusPending,
		Options: a.Options, Fundamentals: a.Fundamentals,
		BuyPrice: a.BuyPrice, SalePrice: a.SalePrice, ReportDateOverride: a.ReportDateOverride, Note: a.Note,
	}
	if r.ConID > 0 {
		r.Status = underlying.StatusActive
	}
	return r, validateFields(r)
}

// ValidateEdit applies e to the newest row cur of its key and returns the row to insert; changed is false when
// nothing differs, and then nothing is written. A correction of exchange or currency of a pending or unresolved row
// clears ConID and Reason and makes the row pending again; on any other row those two fields are not editable.
func ValidateEdit(cur underlying.Row, e underlying.Edit) (next underlying.Row, changed bool, err error) {
	next = cur
	setStr := func(dst *string, v *string) {
		if v != nil {
			*dst = *v
		}
	}
	setStr(&next.Name, e.Name)
	setStr(&next.Issuer, e.Issuer)
	setStr(&next.BuyPrice, e.BuyPrice)
	setStr(&next.SalePrice, e.SalePrice)
	setStr(&next.ReportDateOverride, e.ReportDateOverride)
	setStr(&next.Note, e.Note)
	if e.Options != nil {
		next.Options = *e.Options
	}
	if e.Fundamentals != nil {
		next.Fundamentals = *e.Fundamentals
	}
	open := cur.Status == underlying.StatusPending || cur.Status == underlying.StatusUnresolved
	for _, f := range []struct {
		field string
		dst   *string
		v     *string
	}{{"exchange", &next.Exchange, e.Exchange}, {"currency", &next.Currency, e.Currency}} {
		if f.v == nil || *f.v == *f.dst {
			continue
		}
		if !open {
			return cur, false, invalid(f.field, "cannot be edited once the name is resolved")
		}
		*f.dst = *f.v
		next.ConID, next.Reason, next.Status = 0, "", underlying.StatusPending
	}
	if e.Status != nil {
		switch *e.Status {
		case underlying.StatusActive, underlying.StatusPaused, underlying.StatusRetired:
			next.Status = *e.Status
		default:
			return cur, false, invalid("status", "the owner sets active, paused or retired")
		}
	}
	if next.Status == underlying.StatusActive && next.ConID == 0 {
		return cur, false, invalid("status", "a name without a contract id cannot be active")
	}
	if err := validateFields(next); err != nil {
		return cur, false, err
	}
	next.ID, next.SetAt = 0, time.Time{}
	c := cur
	c.ID, c.SetAt = 0, time.Time{}
	return next, next != c, nil
}

// validateFields checks the text fields of a row.
func validateFields(r underlying.Row) error {
	if !exchange.MatchString(r.Exchange) {
		return invalid("exchange", "must be upper-case letters and digits")
	}
	if !currency.MatchString(r.Currency) {
		return invalid("currency", "must be three upper-case letters")
	}
	if utf8.RuneCountInString(r.Name) > maxName {
		return invalid("name", "at most 80 characters")
	}
	if r.Issuer != "" && !issuerRe.MatchString(r.Issuer) {
		return invalid("issuer", "must be cik:<10 digits> or edinet:<code>, or empty")
	}
	buy, err := price("buy_price", r.BuyPrice)
	if err != nil {
		return err
	}
	sale, err := price("sale_price", r.SalePrice)
	if err != nil {
		return err
	}
	if buy != nil && sale != nil && sale.Cmp(buy) < 0 {
		return invalid("sale_price", "must not be below buy_price")
	}
	if r.ReportDateOverride != "" {
		if _, err := time.Parse("2006-01-02", r.ReportDateOverride); err != nil {
			return invalid("report_date_override", "must be YYYY-MM-DD or empty")
		}
	}
	if utf8.RuneCountInString(r.Note) > maxNote {
		return invalid("note", "at most 200 characters")
	}
	return nil
}

// price parses positive decimal text of at most 4 places; empty gives nil.
func price(field, s string) (*big.Rat, error) {
	if s == "" {
		return nil, nil
	}
	if !priceText.MatchString(s) {
		return nil, invalid(field, "must be a decimal with at most 4 places, or empty")
	}
	v, ok := new(big.Rat).SetString(s)
	if !ok || v.Sign() <= 0 {
		return nil, invalid(field, "must be positive")
	}
	return v, nil
}
