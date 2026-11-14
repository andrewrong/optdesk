package fxrefresh

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

const (
	datasetID      = "obs.fx_rate"
	pair           = "USDJPY"
	sourceMidpoint = "ibkr_midpoint"
	sourceFred     = "fred_h10"
	dayLayout      = "2006-01-02"
)

//go:embed job.toml
var manifestBytes []byte

var _ NewFxRefreshFunc = New

type manifest struct {
	Version      int    `toml:"version"`
	ID           string `toml:"id"`
	DurationDays int    `toml:"duration_days"`
	// Sources is the rank: the first is the primary, every later one a fill source.
	Sources          []string `toml:"sources"`
	FillLookbackDays int      `toml:"fill_lookback_days"`
	Schedule         struct {
		Calendar string `toml:"calendar"`
		Anchor   string `toml:"anchor"`
		Offset   string `toml:"offset"`
		RunBound string `toml:"run_bound"`
	} `toml:"schedule"`
}

type typedError struct {
	kind contracts.ErrorKind
	err  error
}

func (e *typedError) Error() string             { return e.err.Error() }
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func newErr(kind contracts.ErrorKind, format string, a ...any) contracts.TypedError {
	return &typedError{kind: kind, err: fmt.Errorf("fxrefresh: "+format, a...)}
}

// asTyped returns err unchanged when it is a TypedError, else wraps it with kind.
func asTyped(err error, kind contracts.ErrorKind) contracts.TypedError {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te
	}
	return &typedError{kind: kind, err: err}
}

type job struct {
	m        manifest
	offset   time.Duration
	runBound time.Duration
	b        broker.Broker
	h        broker.HistoryClient
	w        bar.FxRateWriter
	r        bar.FxRateReader
	clk      clock.Clock
	log      *slog.Logger
	second   sources.Source // the fill source; nil means no fill
}

// New builds the fx_refresh job from the embedded job.toml: one key, {obs.fx_rate, USDJPY}.
func New(b broker.Broker, h broker.HistoryClient, w bar.FxRateWriter, r bar.FxRateReader, clk clock.Clock) (contracts.Job, error) {
	if b == nil || h == nil || w == nil || r == nil || clk == nil { // r is kept for the signature only
		return nil, errors.New("fxrefresh: broker, history client, writer, reader and clock are required")
	}
	return build(manifestBytes, b, h, w, r, clk)
}

// build is New over the bytes of a manifest.
func build(raw []byte, b broker.Broker, h broker.HistoryClient, w bar.FxRateWriter, r bar.FxRateReader, clk clock.Clock) (contracts.Job, error) {
	var m manifest
	if err := toml.Unmarshal(raw, &m); err != nil {
		return nil, errors.New("fxrefresh: manifest: not valid TOML")
	}
	if m.ID == "" || m.DurationDays <= 0 || m.Schedule.Calendar == "" {
		return nil, errors.New("fxrefresh: manifest: id, duration_days and schedule.calendar are required")
	}
	if err := checkSources(m); err != nil {
		return nil, err
	}
	j := &job{m: m, b: b, h: h, w: w, r: r, clk: clk, log: slog.Default()}
	var err error
	if j.offset, err = time.ParseDuration(m.Schedule.Offset); err != nil {
		return nil, errors.New("fxrefresh: manifest: schedule.offset is not a duration")
	}
	if j.runBound, err = time.ParseDuration(m.Schedule.RunBound); err != nil {
		return nil, errors.New("fxrefresh: manifest: schedule.run_bound is not a duration")
	}
	return j, nil
}

// checkSources refuses a rank that is empty, does not start with the primary or names a source the job cannot serve.
func checkSources(m manifest) error {
	if len(m.Sources) == 0 || m.Sources[0] != sourceMidpoint {
		return fmt.Errorf("fxrefresh: manifest: sources must start with %s", sourceMidpoint)
	}
	for _, s := range m.Sources {
		if s != sourceMidpoint && s != sourceFred {
			return fmt.Errorf("fxrefresh: manifest: unknown source %q", s)
		}
	}
	if m.FillLookbackDays <= 0 {
		return errors.New("fxrefresh: manifest: fill_lookback_days must be positive")
	}
	return nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Schedule is the slot of the manifest as plain values.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	return j.m.Schedule.Calendar, j.m.Schedule.Anchor, "", j.offset
}

// RunBound is the longest time one run may take.
func (j *job) RunBound() time.Duration { return j.runBound }

// RestartSensitive is false: a rerun is idempotent.
func (j *job) RestartSensitive() bool { return false }

// Plan returns the one key and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s\n%s", j.m.ID, j.m.Version, day, pair)))
	return []contracts.ExpectedKey{{DatasetID: datasetID, SubjectKey: pair}}, hex.EncodeToString(sum[:]), nil
}

// Run upserts the days of the MIDPOINT answer that end on day, by key, with writtenBy "fx_refresh:<day>".
// A differing stored rate is replaced, an equal one is untouched, an empty answer replaces nothing.
// After that step, whatever its result, the fill source (when attached) stores the days the table has no row for;
// the result is the primary's.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	res := j.runPrimary(ctx, day, keys)
	j.fill(ctx, day)
	return res
}

func (j *job) runPrimary(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	if _, err := time.Parse(dayLayout, string(day)); err != nil {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "bad day")}
	}
	up, ok := j.w.(bar.FxRateUpserter)
	if !ok {
		return contracts.JobResult{Err: newErr(contracts.Retryable, "the bar store is not available (%s is not set): no rate can be written", bar.DSNEnvBar)}
	}
	var res contracts.JobResult
	for _, k := range keys {
		if k.SubjectKey != pair {
			return contracts.JobResult{Err: newErr(contracts.Permanent, "unknown key %q", k.SubjectKey)}
		}
		returned, terr := j.runPair(ctx, up, day)
		res.Returned += returned
		if returned > 0 {
			res.Usable++
		}
		if terr != nil {
			res.Err = terr
		}
	}
	return res
}

func (j *job) runPair(ctx context.Context, up bar.FxRateUpserter, day contracts.Day) (int, contracts.TypedError) {
	got, err := j.b.ResolveContract(ctx, broker.Contract{SecType: "CASH", Symbol: "USD", Exchange: "IDEALPRO", Currency: "JPY"})
	if err != nil {
		return 0, asTyped(err, contracts.Retryable)
	}
	if !got.Listed || got.Contract.ConID == 0 {
		return 0, newErr(contracts.Permanent, "%s is not listed", pair)
	}
	now := j.clk.Now().UTC()
	q := broker.BarQuery{Contract: got.Contract, Kind: broker.BarMidpoint, DurationDays: j.m.DurationDays}
	if string(day) < now.Format(dayLayout) {
		q.End = string(day)
	}
	bars, err := j.h.Bars(ctx, q)
	if err != nil {
		return 0, asTyped(err, contracts.Retryable)
	}
	var rows []contracts.FxRate
	for _, b := range bars {
		d, err := time.Parse(dayLayout, b.Day)
		if err != nil {
			return 0, newErr(contracts.Permanent, "bad bar day")
		}
		if v, ok := new(big.Rat).SetString(b.Close); !ok || v.Sign() <= 0 {
			return 0, newErr(contracts.Permanent, "bar close is not a positive decimal")
		}
		if b.Day > string(day) {
			continue
		}
		rows = append(rows, contracts.FxRate{
			Pair: pair, Rate: b.Close, RateDay: types.SerializableDate{Time: d}, ReceivedAt: now,
			Source: sourceMidpoint,
		})
	}
	if len(rows) == 0 {
		return 0, nil
	}
	if _, err := up.UpsertFxRates(ctx, rows, "fx_refresh:"+string(day)); err != nil {
		return 0, asTyped(err, contracts.Retryable)
	}
	return len(rows), nil
}
