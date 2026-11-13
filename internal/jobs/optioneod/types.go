package optioneod

import (
	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
)

// NewOptionEodFunc is the constructor of the option_eod job (internal/jobs/optioneod.New).
type NewOptionEodFunc func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock) (contracts.Job, error)

// NewOptionEodSymbolsFunc is NewOptionEodFunc with a symbol.Translator as the last argument. A nil translator
// means no translation: the job behaves as it does without one, including its own matching of share-class
// separators.
type NewOptionEodSymbolsFunc func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock, symbol.Translator) (contracts.Job, error)

// NewOptionEodWatchFunc is the constructor of the option_eod job that takes its underlyings from the watch
// list (internal/jobs/optioneod.NewWithWatch): the US stock entries, each with a key per symbol. Expiries are
// those with DTE in min_dte..max_dte of job.toml; the strikes of an entry with a BuyPrice run from
// buy_low x BuyPrice to max(buy_high x BuyPrice, spot) (the manifest keys buy_low and buy_high), at most max_strikes, the nearest BuyPrice kept; an entry
// without a BuyPrice, or a manifest without the two keys, keeps the strike_band. The list comes before the clock.
type NewOptionEodWatchFunc func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []jobs.WatchEntry, clock.Clock) (contracts.Job, error)

// NewOptionEodWatchSymbolsFunc is NewOptionEodWatchFunc with a symbol.Translator as the last argument
// (internal/jobs/optioneod.NewWithWatchSymbols). A nil translator means no translation: the job behaves as it
// does without one, including its own matching of share-class separators.
type NewOptionEodWatchSymbolsFunc func(
	broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []jobs.WatchEntry, clock.Clock, symbol.Translator,
) (contracts.Job, error)

// NewOptionEodHeldFunc is the constructor of the option_eod job that also captures the contracts of the short
// calls the account holds, whatever their DTE (declaration only; its implementation is a later issue). The
// watch list comes after the writer, the reader after the list and the clock last.
type NewOptionEodHeldFunc func(
	broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []jobs.WatchEntry, store.AccountReader, clock.Clock,
) (contracts.Job, error)

// OptionEodWindow is the window the owner settings give the option_eod job: min_dte, max_dte and max_strikes.
// A zero field keeps the value of the embedded job.toml; a non-zero field replaces it.
type OptionEodWindow struct {
	MinDTE, MaxDTE, MaxStrikes int
}

// NewOptionEodWatchWindowFunc is the constructor of the option_eod job that takes its underlyings from the
// watch list and its window from the owner settings (internal/jobs/optioneod.NewWithWatchWindow). It behaves
// as NewOptionEodWatchFunc, then replaces min_dte, max_dte and max_strikes of the manifest with each non-zero
// field of the window; a window with min_dte above max_dte is an error. The window comes after the list.
type NewOptionEodWatchWindowFunc func(
	broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []jobs.WatchEntry, OptionEodWindow, clock.Clock,
) (contracts.Job, error)
