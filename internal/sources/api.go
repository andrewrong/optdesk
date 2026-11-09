// Package sources owns the clients of external data sources.
package sources

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// FREDConfig holds the arguments of NewFREDFunc.
type FREDConfig struct {
	BaseURL string
	// APIKey is passed in by the caller; this package reads no environment.
	APIKey string
	Clock  clock.Clock
}

// FetchPlan names the series to fetch and the observation date range, inclusive.
type FetchPlan struct {
	SeriesIDs []string
	From      contracts.Day
	To        contracts.Day
}

// Source returns the points of a plan with provenance; it returns contracts.TypedError values only.
type Source interface {
	Fetch(ctx context.Context, plan FetchPlan) ([]contracts.SeriesPoint, error)
}

// NewFREDFunc is the constructor of the FRED source (internal/sources/fred.New).
type NewFREDFunc func(FREDConfig) (Source, error)

// FlexSource fetches one IBKR Flex report; it returns contracts.TypedError values only, and none holds
// the token, the query id or an address.
type FlexSource interface {
	Fetch(ctx context.Context, req FlexRequest) (FlexReport, error)
}

// FlexRequest names one query and the inclusive window of the report.
type FlexRequest struct {
	QueryID string
	First   contracts.Day
	Last    contracts.Day
}

// FlexReport is one fetched report: the fetch instant, the SHA-256 of the raw body and its statements.
type FlexReport struct {
	FetchedAt  time.Time
	RawSHA256  string
	Statements []FlexStatement
}

// FlexStatement is one account statement. AccountID is returned only so that the job can map it to an
// alias and drop it; it is never logged. Trades holds the EXECUTION-level trade rows as text;
// OtherLevels counts the rows of other levels.
type FlexStatement struct {
	AccountID     string
	FromDate      string
	ToDate        string
	Period        string
	WhenGenerated string
	Trades        []map[string]string
	OtherLevels   int
}

// FlexConfig holds the arguments of NewFlexFunc. Token is passed in by the caller; this package reads
// no environment. Wait pauses between requests and returns early with the context error.
type FlexConfig struct {
	Token string
	Wait  func(ctx context.Context, d time.Duration) error
	Clock clock.Clock
}

// NewFlexFunc is the constructor of the Flex source (internal/sources/flex).
type NewFlexFunc func(FlexConfig) (FlexSource, error)

// FlexFile is the content of $OT_HOME/flex.toml: the logins and the account allow-list.
type FlexFile struct {
	Logins   []FlexLogin
	Accounts []FlexAccount
}

// FlexLogin is one [[login]] entry. TokenEnv is the NAME of the variable that holds the token.
// Required is the `required` key (default false): the login that serves the main account; the loader refuses a file with
// no required login or with more than one.
type FlexLogin struct {
	Name         string
	TokenEnv     string
	TradeQueries []string
	Required     bool
}

// FlexAccount is one [[account]] entry. AccountEnv is the NAME of the variable that holds the account number.
type FlexAccount struct {
	Alias      string
	AccountEnv string
}

// LoadFlexFileFunc reads and validates the file at path. Errors wrap ErrFlexNotConfigured (no file) or
// ErrFlexConfig (a bad or unsafe file); their texts name the entry and the field and never a value.
type LoadFlexFileFunc func(path string) (FlexFile, error)

// ErrFlexNotConfigured means there is no flex.toml, or a login's token variable is empty.
var ErrFlexNotConfigured = errors.New("flex: not configured")

// ErrFlexConfig means flex.toml is unsafe or invalid.
var ErrFlexConfig = errors.New("flex: bad configuration")

// HTTPDoer is the HTTP client a source is given; *http.Client satisfies it, and a test passes a fake.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// EarningsConfig holds the arguments of NewEarningsFunc. APIKey is passed in by the caller; this package
// reads no environment. Symbols are the JP codes of the J-Quants source; Alpha Vantage ignores them.
// Doer is optional: nil means an http.Client that follows no redirect.
type EarningsConfig struct {
	BaseURL string
	APIKey  string
	Symbols []string
	Doer    HTTPDoer
}

// EarningsListing is one parsed row of an earnings calendar. Dates are YYYY-MM-DD; Estimate is decimal
// text; empty strings mean the source did not say.
type EarningsListing struct {
	Symbol          string
	ReportDate      contracts.Day
	FiscalPeriodEnd contracts.Day
	Estimate        string
	TimeOfDay       string
}

// EarningsSource fetches the earnings calendar of one outside service. One Fetch makes the calls the
// service's budget allows (Alpha Vantage: exactly one request), follows no redirect and never retries.
// It returns CodedError values only; none holds the key or an address. Only parsed rows are returned.
type EarningsSource interface {
	Fetch(ctx context.Context) ([]EarningsListing, error)
}

// NewEarningsFunc is the constructor of an earnings source (internal/sources/alphavantage.New).
type NewEarningsFunc func(EarningsConfig) (EarningsSource, error)

// CodedError is a typed error that also names the error_code of the earnings_date row the job writes:
// rate_limited (a limit answer or a notice body), http_status, network, not_csv, short_calendar.
type CodedError interface {
	contracts.TypedError
	ErrorCode() string
}

// NewJQuantsFunc is the constructor of the J-Quants earnings source (internal/sources/jquants.New). It
// reads EarningsConfig.Symbols (4-digit JP codes) and makes one call per symbol per Fetch.
type NewJQuantsFunc func(EarningsConfig) (EarningsSource, error)
