package instrument

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/stockreg"
)

var _ stockreg.StockRegistrar = (*instrumentStore)(nil)

// The statements of RegisterStock (docs/design/instrument-company.md Decision 8b). Every write is preceded by a read,
// so equal input sends no write.
const (
	// issuerSQL answers the issuer rows that carry either code, in id order; a NULL parameter matches nothing.
	issuerSQL = `SELECT issuer_id, name, country, cik, edinet_code FROM master.issuer
WHERE cik = $1::text OR edinet_code = $2::text ORDER BY issuer_id`

	insertIssuerSQL = `INSERT INTO master.issuer (name, country, cik, edinet_code) VALUES ($1, $2, $3::text, $4::text) RETURNING issuer_id`

	updateIssuerSQL = `UPDATE master.issuer SET name = $2, country = $3, cik = $4::text, edinet_code = $5::text WHERE issuer_id = $1`

	// stockStateSQL answers the one row of the instrument with its stock row (NULL columns when there is none).
	stockStateSQL = `SELECT i.symbol, i.issuer_id, s.instrument_id, s.name, s.market, s.primary_exchange, s.issuer_id, s.sector_fund
FROM master.instrument i LEFT JOIN master.stock s ON s.instrument_id = i.instrument_id
WHERE i.instrument_id = $1`

	setInstrumentSQL = `UPDATE master.instrument SET symbol = $2, issuer_id = $3 WHERE instrument_id = $1`

	upsertStockSQL = `INSERT INTO master.stock (instrument_id, name, market, primary_exchange, issuer_id, sector_fund)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (instrument_id) DO UPDATE
SET name = EXCLUDED.name, market = EXCLUDED.market, primary_exchange = EXCLUDED.primary_exchange,
    issuer_id = EXCLUDED.issuer_id, sector_fund = EXCLUDED.sector_fund`
)

var markets = map[string]bool{"US": true, "JP": true, "EU": true}

// issuerRow is a row of master.issuer; the codes are "" when NULL.
type issuerRow struct {
	id                         int64
	name, country, cik, edinet string
}

// RegisterStock writes the stock side and the company of the stock, etf or index whose ibkr_conid is r.ConID and
// returns its instrument id. It creates the instrument as the jobs do (type stock, calendar XNYS when none is given)
// when none exists, overwrites symbol, name, market, primary exchange and sector of the stock, and finds or creates
// the issuer by its codes (a changed name or country overwrites), setting the issuer on the instrument row and the stock
// row alike. A nil Issuer leaves the company link as it is. Two issuer rows that carry the codes of the registration
// are a Permanent error naming both ids and nothing is written. Equal input writes nothing. Input that is not valid is
// a Permanent error before any statement.
func (s *instrumentStore) RegisterStock(ctx context.Context, r stockreg.Registration) (int64, error) {
	key := store.Identifier{Scheme: "ibkr_conid", Value: strconv.FormatInt(r.ConID, 10)}
	spec, err := registrationSpec(r)
	if err != nil {
		return 0, err
	}
	var id int64
	var termsClass string
	err = s.db.InTx(ctx, func(tx execer) error {
		if err := tx.Exec(ctx, lockSQL, key.Scheme, key.Value); err != nil {
			return err
		}
		issuerID, err := findOrWriteIssuer(ctx, tx, r.Issuer)
		if err != nil {
			return err
		}
		if id, err = resolveTx(ctx, tx, key, spec, nil, &termsClass); err != nil {
			return err
		}
		return writeStock(ctx, tx, id, r, issuerID)
	})
	if err != nil {
		return 0, asTyped(err, "register stock")
	}
	return id, nil
}

// registrationSpec checks r and returns the spec the instrument is created with.
func registrationSpec(r stockreg.Registration) (store.InstrumentSpec, error) {
	if r.ConID <= 0 {
		return store.InstrumentSpec{}, permanent(fmt.Errorf("instrument: register stock: con_id %d is not positive", r.ConID))
	}
	if r.Kind != "stock" && r.Kind != "etf" {
		return store.InstrumentSpec{}, permanent(fmt.Errorf("instrument: register stock: kind %q is not stock or etf", r.Kind))
	}
	if r.Symbol == "" {
		return store.InstrumentSpec{}, permanent(fmt.Errorf("instrument: register stock: empty symbol"))
	}
	if !markets[r.Market] {
		return store.InstrumentSpec{}, permanent(fmt.Errorf("instrument: register stock: market %q is not US, JP or EU", r.Market))
	}
	if r.Issuer != nil && r.Issuer.CIK == "" && r.Issuer.EdinetCode == "" {
		return store.InstrumentSpec{}, permanent(fmt.Errorf("instrument: register stock: issuer has neither cik nor edinet_code"))
	}
	cal := r.CalendarID
	if cal == "" {
		cal = "XNYS"
	}
	spec := store.InstrumentSpec{Type: "stock", Name: r.Symbol, Currency: r.Currency, MIC: r.MIC, CalendarID: cal,
		Status: defaultStat, PriceIncrement: "0.01", LotSize: "1"}
	if err := checkInput(store.Identifier{Scheme: "ibkr_conid", Value: strconv.FormatInt(r.ConID, 10)}, spec, nil); err != nil {
		return spec, err
	}
	if r.Currency == "JPY" && strings.Contains(r.Symbol, ".") {
		return spec, permanent(fmt.Errorf("instrument: register stock: symbol %q of a JPY instrument has a dot", r.Symbol))
	}
	return spec, nil
}

// findOrWriteIssuer returns the id of the issuer of the registration, creating or updating the row; nil when the
// registration has no issuer. Two rows for its codes are refused before any write.
func findOrWriteIssuer(ctx context.Context, tx execer, in *stockreg.Issuer) (*int64, error) {
	if in == nil {
		return nil, nil
	}
	var rows []issuerRow
	err := tx.Query(ctx, issuerSQL, []any{nullText(in.CIK), nullText(in.EdinetCode)}, func(sc scanner) error {
		var row issuerRow
		var cik, edinet *string
		if err := sc.Scan(&row.id, &row.name, &row.country, &cik, &edinet); err != nil {
			return err
		}
		row.cik, row.edinet = deref(cik), deref(edinet)
		rows = append(rows, row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	switch len(rows) {
	case 0:
		var id int64
		err := tx.Query(ctx, insertIssuerSQL, []any{in.Name, in.Country, nullText(in.CIK), nullText(in.EdinetCode)},
			func(sc scanner) error { return sc.Scan(&id) })
		return &id, err
	case 1:
		row := rows[0]
		cik, edinet := orKeep(in.CIK, row.cik), orKeep(in.EdinetCode, row.edinet)
		if row.name != in.Name || row.country != in.Country || row.cik != cik || row.edinet != edinet {
			if err := tx.Exec(ctx, updateIssuerSQL, row.id, in.Name, in.Country, nullText(cik), nullText(edinet)); err != nil {
				return nil, err
			}
		}
		return &row.id, nil
	}
	return nil, permanent(fmt.Errorf("instrument: register stock: cik %q is issuer %d and edinet_code %q is issuer %d",
		in.CIK, rows[0].id, in.EdinetCode, rows[1].id))
}

// writeStock brings the instrument row and the stock row of id to the registration; what already equals it is not
// written. A nil issuerID keeps the issuer the rows have.
func writeStock(ctx context.Context, tx execer, id int64, r stockreg.Registration, issuerID *int64) error {
	var symbol string
	var instIssuer, stockID, stockIssuer *int64
	var name, market, exchange, sector *string
	err := tx.Query(ctx, stockStateSQL, []any{id}, func(sc scanner) error {
		return sc.Scan(&symbol, &instIssuer, &stockID, &name, &market, &exchange, &stockIssuer, &sector)
	})
	if err != nil {
		return err
	}
	issuer := issuerID
	if issuer == nil {
		issuer = stockIssuer
	}
	if symbol != r.Symbol || !sameID(instIssuer, issuer) {
		if err := tx.Exec(ctx, setInstrumentSQL, id, r.Symbol, idArg(issuer)); err != nil {
			return err
		}
	}
	name0 := r.Name
	if name0 == "" {
		name0 = r.Symbol
	}
	if stockID != nil && deref(name) == name0 && deref(market) == r.Market && deref(exchange) == r.PrimaryExchange &&
		sameID(stockIssuer, issuer) && deref(sector) == r.SectorFund {
		return nil
	}
	return tx.Exec(ctx, upsertStockSQL, id, name0, r.Market, nullText(r.PrimaryExchange), idArg(issuer), nullText(r.SectorFund))
}

func nullText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func orKeep(v, old string) string {
	if v != "" {
		return v
	}
	return old
}

func sameID(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func idArg(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}
