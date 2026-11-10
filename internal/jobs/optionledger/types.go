package optionledger

import (
	"context"
	"io"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/flexsection"
)

// OptionLedgerStores is the store interfaces the option ledger job uses.
type OptionLedgerStores struct {
	Ledger store.OptionLedgerWriter
	Read   store.OptionLedgerReader
	// Sections records the Flex section rows; nil switches the section step off.
	Sections flexsection.Writer
}

// FlexRun is one Flex login of the option ledger job. A login whose token variable is empty reaches the job
// as a FlexRun with a nil Source: the constructor accepts it and never fails for a missing token; Run
// returns a failed result when the login is Required, else skips it with the degraded log record
// (job ledger-09-job).
type FlexRun struct {
	Name     string
	QueryIDs []string
	Required bool
	Source   sources.FlexSource
}

// OptionLedgerConfig is the configuration of the option_ledger job. Accounts maps account number to alias;
// the wiring builds it from the environment and it is never printed. HistoryFrom is the first day fetched.
type OptionLedgerConfig struct {
	Logins      []FlexRun
	Accounts    map[string]string
	HistoryFrom string
}

// NewOptionLedgerJobFunc is the constructor of the option_ledger job (internal/jobs/optionledger).
type NewOptionLedgerJobFunc func(OptionLedgerStores, OptionLedgerConfig, clock.Clock) (contracts.Job, error)

// ImportOptionTraderFunc imports the JSON Lines export of option-trader's stored Flex rows
// (internal/jobs/optionledger); aliasMap maps the alias of the file to the alias of the ledger and now is the
// recording instant.
type ImportOptionTraderFunc func(
	ctx context.Context, r io.Reader, w store.OptionLedgerWriter, aliasMap map[string]string, now time.Time,
) (jobs.ImportResult, error)

// FlexAttributesSHA256Func is the hash of the attributes of one Flex row (internal/jobs/optionledger.AttributesSHA256).
// The recipe: remove the keys accountId and acctAlias; encode the rest as one JSON object, keys ascending by
// byte order, no whitespace, strings in UTF-8 with only ", \ and control characters below 0x20 escaped and no
// HTML escaping; the hash is the lower-case hex SHA-256 of that text.
type FlexAttributesSHA256Func func(attrs map[string]string) (string, error)
