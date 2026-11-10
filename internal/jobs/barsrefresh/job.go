package barsrefresh

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

const (
	datasetID   = "obs.price_bar"
	basisTrades = "trades"
	dayLayout   = "2006-01-02"
	indexPrefix = "index:"
	sourceIbkr  = "ibkr"
	typeStock   = "stock"
	typeIndex   = "index"
)

var _ NewBarsRefreshFunc = New

type typedError struct {
	kind contracts.ErrorKind
	err  error
}

func (e *typedError) Error() string             { return e.err.Error() }
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func newErr(kind contracts.ErrorKind, format string, a ...any) contracts.TypedError {
	return &typedError{kind: kind, err: fmt.Errorf("barsrefresh: "+format, a...)}
}

// asTyped returns err unchanged when it is a TypedError, else wraps it with kind.
func asTyped(err error, kind contracts.ErrorKind) contracts.TypedError {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te
	}
	return &typedError{kind: kind, err: err}
}

// target is what one key pulls: the contract to ask for and the identity of its instrument.
type target struct {
	contract broker.Contract
	spec     store.InstrumentSpec
}

type job struct {
	m     Manifest
	b     broker.Broker
	h     broker.HistoryClient
	res   store.InstrumentResolver
	w     bar.PriceBarWriter
	r     bar.PriceBarReader
	clk   clock.Clock
	keys  []string
	byKey map[string]target

	mu      sync.Mutex
	started map[contracts.Day]map[string]bool // keys that began a first pull, per day
}

// New builds the bars_refresh job from the embedded job.toml: one key per watch entry (its Issuer) and
// per [[index]] (index:<symbol>).
func New(
	b broker.Broker, h broker.HistoryClient, res store.InstrumentResolver, w bar.PriceBarWriter,
	r bar.PriceBarReader, entries []jobs.WatchEntry, clk clock.Clock,
) (contracts.Job, error) {
	if b == nil || h == nil || res == nil || w == nil || r == nil || clk == nil {
		return nil, errors.New("barsrefresh: broker, history client, resolver, writer, reader and clock are required")
	}
	m, err := DefaultManifest()
	if err != nil {
		return nil, err
	}
	j := &job{m: m, b: b, h: h, res: res, w: w, r: r, clk: clk, byKey: map[string]target{},
		started: map[contracts.Day]map[string]bool{}}
	add := func(key string, t target) error {
		if key == "" || key == indexPrefix {
			return errors.New("barsrefresh: an entry needs a key")
		}
		if _, dup := j.byKey[key]; dup {
			return fmt.Errorf("barsrefresh: duplicate key %q", key)
		}
		j.keys = append(j.keys, key)
		j.byKey[key] = t
		return nil
	}
	for _, e := range entries {
		if err := add(e.Issuer, target{contract: e.Contract, spec: specOf(typeStock, e.Contract)}); err != nil {
			return nil, err
		}
	}
	for _, ix := range m.Indexes {
		c := broker.Contract{SecType: ix.SecType, Symbol: ix.Symbol, Exchange: ix.Exchange, Currency: ix.Currency, ConID: ix.ConID}
		if err := add(indexPrefix+ix.Symbol, target{contract: c, spec: specOf(typeIndex, c)}); err != nil {
			return nil, err
		}
	}
	return j, nil
}

func specOf(typ string, c broker.Contract) store.InstrumentSpec {
	return store.InstrumentSpec{Type: typ, Name: c.Symbol, Currency: c.Currency, Status: "active", PriceIncrement: "0.01", LotSize: "1"}
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Schedule is the slot of the manifest as plain values.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	s := j.m.Schedule
	return s.Calendar, s.Anchor, "", s.Offset
}

// RunBound is the longest time one run of a key may take.
func (j *job) RunBound() time.Duration { return j.m.Schedule.RunBound }

// RestartSensitive is false: a rerun inserts only what is missing.
func (j *job) RestartSensitive() bool { return false }
