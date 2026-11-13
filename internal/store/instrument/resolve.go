package instrument

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/andrewrong/optdesk/internal/store"
)

// The statements of ResolveInstrument, in the order they run in one transaction.
const (
	// lockSQL serialises the callers of one key: the primary key of the identifier table includes valid_from, so it
	// alone would let two callers create two instruments.
	lockSQL = `SELECT pg_advisory_xact_lock(hashtext($1::text || ':' || $2::text))`

	lookupSQL = `SELECT instrument_id FROM master.instrument_identifier WHERE scheme = $1 AND value = $2 AND valid_to IS NULL`

	insertInstrumentSQL = `INSERT INTO master.instrument (instrument_type, name, currency, mic, calendar_id, status, price_increment, lot_size, recorded_at)
VALUES ($1, $2, $3, $4, $5, $6, $7::numeric, $8::numeric, now())
RETURNING instrument_id`

	insertIdentifierSQL = `INSERT INTO master.instrument_identifier (instrument_id, scheme, value, valid_from) VALUES ($1, $2, $3, now())`

	// setSymbolSQL gives the instrument its symbol; the insert above is the one the previous binary sent, so it stays as it
	// is and the column keeps its default until this statement runs (migration 0044).
	setSymbolSQL = `UPDATE master.instrument SET symbol = $2 WHERE instrument_id = $1`

	// insertStockSQL writes the stock side row of a new stock or index; a row that exists (the backfill of 0044) stays.
	insertStockSQL = `INSERT INTO master.stock (instrument_id, name, market) VALUES ($1, $2, $3) ON CONFLICT (instrument_id) DO NOTHING`

	// emptySymbolSQL answers one row when the instrument was made without a symbol (by the binary of before 0044).
	emptySymbolSQL = `SELECT instrument_type FROM master.instrument WHERE instrument_id = $1 AND symbol = ''`

	// The terms of a new option run between a savepoint and its release, so a failure costs only the terms row.
	savepointSQL = `SAVEPOINT option_terms`
	releaseSQL   = `RELEASE SAVEPOINT option_terms`
	rollbackSQL  = `ROLLBACK TO SAVEPOINT option_terms`

	// insertTermsSQL writes the current terms row; exercise style and settlement type are not known to the caller. The
	// conflict target is read by the statement: ot_w_master has SELECT on instrument_id and valid_to for it (0011).
	insertTermsSQL = `INSERT INTO master.option_terms
  (instrument_id, "right", strike, strike_currency, multiplier, expiration_date, exercise_style, settlement_type, valid_from)
VALUES ($1, $2, $3::numeric, $4, $5::numeric, $6::date, 'unknown', 'unknown', now())
ON CONFLICT (instrument_id) WHERE valid_to IS NULL DO NOTHING`
)

var (
	schemes     = map[string]bool{"ibkr_conid": true, "osi": true, "ticker_exchange": true, "jpx_option_code": true, "isin": true}
	types       = map[string]bool{"stock": true, "index": true, "option": true}
	currencyRe  = regexp.MustCompile(`^[A-Z]{3}$`)
	decimalRe   = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)
	defaultStat = "active"
)

func checkIdentifier(what string, id store.Identifier) error {
	if !schemes[id.Scheme] {
		return permanent(fmt.Errorf("instrument: %s: unknown scheme %q", what, id.Scheme))
	}
	if id.Value == "" {
		return permanent(fmt.Errorf("instrument: %s: empty value for scheme %s", what, id.Scheme))
	}
	return nil
}

func checkInput(key store.Identifier, spec store.InstrumentSpec, extra []store.Identifier) error {
	if err := checkIdentifier("key", key); err != nil {
		return err
	}
	for _, e := range extra {
		if err := checkIdentifier("extra", e); err != nil {
			return err
		}
	}
	if !types[spec.Type] {
		return permanent(fmt.Errorf("instrument: type %q is not stock, index or option", spec.Type))
	}
	if !currencyRe.MatchString(spec.Currency) {
		return permanent(fmt.Errorf("instrument: currency %q is not three upper-case letters", spec.Currency))
	}
	if !decimalRe.MatchString(spec.PriceIncrement) {
		return permanent(fmt.Errorf("instrument: price increment %q is not a decimal", spec.PriceIncrement))
	}
	if !decimalRe.MatchString(spec.LotSize) {
		return permanent(fmt.Errorf("instrument: lot size %q is not a decimal", spec.LotSize))
	}
	return nil
}

// ResolveInstrument returns the id of the instrument that key names now. When none exists it creates the instrument
// with spec (status "active" when spec has none) and the identifiers key and extra, valid from the database's now(),
// in one transaction that first takes an advisory lock over key. For a new option with a valid osi identifier it also
// inserts the current master.option_terms row inside a savepoint; a failure there never fails the call. An extra identifier that already names an
// instrument is a Permanent error and nothing is written; so is any invalid input, before any statement. A new
// instrument also gets its symbol and, for a stock or index, its master.stock row; a found stock or index with an empty
// symbol gets both once (docs/design/instrument-company.md Decision 8a); a JPY symbol with a dot is Permanent.
func (s *instrumentStore) ResolveInstrument(ctx context.Context, key store.Identifier, spec store.InstrumentSpec, extra []store.Identifier) (int64, error) {
	if err := checkInput(key, spec, extra); err != nil {
		return 0, err
	}
	if spec.Status == "" {
		spec.Status = defaultStat
	}
	var id int64
	var termsClass string // the class of a rolled-back terms insert, "" when none
	err := s.db.InTx(ctx, func(tx execer) error {
		var err error
		id, err = resolveTx(ctx, tx, key, spec, extra, &termsClass)
		return err
	})
	if err != nil {
		return 0, asTyped(err, "resolve")
	}
	if termsClass != "" {
		s.logTermsRolledBack(id, termsClass)
	}
	return id, nil
}

// resolveTx is the body of ResolveInstrument inside the transaction tx; the input is checked and the status set.
// A new instrument gets its symbol (the osi identifier of an option, else spec.Name) and, for a stock or an index, its
// master.stock row. A found stock or index whose symbol is empty gets both once; a found option is left alone. A
// symbol with a dot on a JPY instrument is a Permanent error before any statement.
func resolveTx(ctx context.Context, tx execer, key store.Identifier, spec store.InstrumentSpec, extra []store.Identifier, termsClass *string) (int64, error) {
	ids := []store.Identifier{key}
	for _, e := range extra {
		if e != key {
			ids = append(ids, e)
		}
	}
	symbol := spec.Name
	if osi, ok := firstOSI(ids); ok && spec.Type == "option" {
		symbol = osi
	}
	if spec.Currency == "JPY" && strings.Contains(symbol, ".") {
		return 0, permanent(fmt.Errorf("instrument: symbol %q of a JPY instrument has a dot; a Tokyo symbol has no suffix", symbol))
	}
	if err := tx.Exec(ctx, lockSQL, key.Scheme, key.Value); err != nil {
		return 0, err
	}
	found, err := lookup(ctx, tx, key)
	if err != nil {
		return 0, err
	}
	if found != 0 {
		return found, repairSymbol(ctx, tx, found, spec, symbol)
	}
	for _, e := range ids[1:] {
		taken, err := lookup(ctx, tx, e)
		if err != nil {
			return 0, err
		}
		if taken != 0 {
			return 0, permanent(fmt.Errorf("instrument: identifier %s %q already names instrument %d", e.Scheme, e.Value, taken))
		}
	}
	var id int64
	err = tx.Query(ctx, insertInstrumentSQL, []any{spec.Type, spec.Name, spec.Currency, spec.MIC, spec.CalendarID,
		spec.Status, spec.PriceIncrement, spec.LotSize}, func(r scanner) error { return r.Scan(&id) })
	if err != nil {
		return 0, err
	}
	if symbol != "" {
		if err := tx.Exec(ctx, setSymbolSQL, id, symbol); err != nil {
			return 0, err
		}
	}
	for _, i := range ids {
		if err := tx.Exec(ctx, insertIdentifierSQL, id, i.Scheme, i.Value); err != nil {
			return 0, err
		}
	}
	if spec.Type == "option" {
		return id, writeTerms(ctx, tx, id, spec, ids, termsClass)
	}
	return id, tx.Exec(ctx, insertStockSQL, id, spec.Name, marketOf(spec.Currency))
}

// repairSymbol fills the symbol and the stock row of a found stock or index that has no symbol. Options are not touched.
func repairSymbol(ctx context.Context, tx execer, id int64, spec store.InstrumentSpec, symbol string) error {
	if spec.Type == "option" || symbol == "" {
		return nil
	}
	var typ string
	if err := tx.Query(ctx, emptySymbolSQL, []any{id}, func(r scanner) error { return r.Scan(&typ) }); err != nil {
		return err
	}
	if typ != "stock" && typ != "index" {
		return nil
	}
	if err := tx.Exec(ctx, setSymbolSQL, id, symbol); err != nil {
		return err
	}
	return tx.Exec(ctx, insertStockSQL, id, spec.Name, marketOf(spec.Currency))
}

// marketOf is the market of a currency: US, JP or EU, nil (NULL) for any other currency.
func marketOf(currency string) any {
	switch currency {
	case "USD":
		return "US"
	case "JPY":
		return "JP"
	case "EUR":
		return "EU"
	}
	return nil
}

// writeTerms inserts the current terms row of the new option id from its osi identifier. No osi, or one that does
// not parse, writes nothing. A failed insert is rolled back to the savepoint and is not an error: the instrument stays.
// Only a failure of the savepoint or of the rollback to it, which leaves the transaction unusable, is returned.
func writeTerms(ctx context.Context, tx execer, id int64, spec store.InstrumentSpec, ids []store.Identifier, class *string) error {
	osi, ok := firstOSI(ids)
	if !ok {
		return nil
	}
	right, strike, expiry, err := parseOSI(osi)
	if err != nil {
		return nil
	}
	if err := tx.Exec(ctx, savepointSQL); err != nil {
		return err
	}
	err = tx.Exec(ctx, insertTermsSQL, id, right, strike, spec.Currency, spec.LotSize, expiry)
	if err == nil {
		err = tx.Exec(ctx, releaseSQL)
	}
	if err != nil {
		*class = sqlClass(err)
		return tx.Exec(ctx, rollbackSQL)
	}
	return nil
}

// firstOSI returns the value of the first osi identifier of ids; only that one gives terms.
func firstOSI(ids []store.Identifier) (string, bool) {
	for _, i := range ids {
		if i.Scheme == "osi" {
			return i.Value, true
		}
	}
	return "", false
}

// lookup returns the id of the instrument the valid identifier names, or 0.
func lookup(ctx context.Context, tx execer, id store.Identifier) (int64, error) {
	var found int64
	err := tx.Query(ctx, lookupSQL, []any{id.Scheme, id.Value}, func(r scanner) error { return r.Scan(&found) })
	return found, err
}
