package underlyingresolve

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/stockreg"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// NewWithRegistrar builds the job like New and also registers the stock and its company with reg after each Settle
// that wrote a con_id (docs/design/instrument-company.md, Decision 9a).
func NewWithRegistrar(b broker.Broker, s underlying.Settler, reg stockreg.StockRegistrar, clk clock.Clock) (contracts.Job, error) {
	if reg == nil {
		return nil, errors.New("underlyingresolve: registrar is required")
	}
	built, err := New(b, s, clk)
	if err != nil {
		return nil, err
	}
	j := built.(*job)
	j.reg = reg
	return j, nil
}

// RegistrationOf maps a settled row of settings.underlying to the registration of its stock and company. It is the
// one mapping the job and `ot underlying import` use. MIC is empty and the calendar XNYS, the values every job passes
// today; a primary exchange of "SMART" is a routing name and is left empty.
func RegistrationOf(row underlying.Row) stockreg.Registration {
	primary := row.Exchange
	if primary == "SMART" {
		primary = ""
	}
	return stockreg.Registration{
		ConID: row.ConID, Kind: row.Kind, Symbol: row.Symbol, Market: row.Market, PrimaryExchange: primary,
		Name: row.Name, Currency: row.Currency, CalendarID: "XNYS", Issuer: issuerOf(row),
	}
}

// issuerOf reads the watch-list text "cik:<10 digits>" or "edinet:<code>"; anything else gives nil.
func issuerOf(row underlying.Row) *stockreg.Issuer {
	switch {
	case strings.HasPrefix(row.Issuer, "cik:"):
		cik := strings.TrimPrefix(row.Issuer, "cik:")
		if len(cik) != 10 || strings.Trim(cik, "0123456789") != "" {
			return nil
		}
		return &stockreg.Issuer{Name: row.Name, Country: "US", CIK: cik}
	case strings.HasPrefix(row.Issuer, "edinet:"):
		code := strings.TrimPrefix(row.Issuer, "edinet:")
		if code == "" || strings.ContainsAny(code, " \t\r\n") {
			return nil
		}
		return &stockreg.Issuer{Name: row.Name, Country: "JP", EdinetCode: code}
	}
	return nil
}

// register sends the settled row to the registrar. A Permanent refusal is logged and returns nil; any other error is
// returned for the caller to make Retryable.
func (j *job) register(ctx context.Context, r underlying.Row, conID int64) error {
	if j.reg == nil {
		return nil
	}
	r.ConID = conID
	if _, err := j.reg.RegisterStock(ctx, RegistrationOf(r)); err != nil {
		var te contracts.TypedError
		if errors.As(err, &te) && te.Kind() == contracts.Permanent {
			slog.Warn("underlying_resolve: company registration refused", "key", r.Key, "reason", err.Error())
			return nil
		}
		return err
	}
	return nil
}
