package reconbars

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/reconapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	reconstore "github.com/andrewrong/optdesk/internal/store/recon"
)

const (
	datasetID = "recon.run"
	pairID    = "R6"
)

var _ NewReconBarsFunc = New

type typedError struct {
	kind contracts.ErrorKind
	err  error
}

func (e *typedError) Error() string             { return e.err.Error() }
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func newErr(kind contracts.ErrorKind, format string, a ...any) contracts.TypedError {
	return &typedError{kind: kind, err: fmt.Errorf("reconbars: "+format, a...)}
}

// asTyped returns err unchanged when it is a TypedError, else wraps it with kind.
func asTyped(err error, kind contracts.ErrorKind) contracts.TypedError {
	if te, ok := errors.AsType[contracts.TypedError](err); ok {
		return te
	}
	return &typedError{kind: kind, err: err}
}

type job struct {
	m     Manifest
	src   sources.BarSource
	res   store.InstrumentResolver
	r     bar.PriceBarReader
	w     reconstore.Writer
	cmp   reconapi.Func
	clk   clock.Clock
	pairs []sources.BarPair
	conID map[sources.BarPair]int64
}

// New builds the recon_bars job from the embedded job.toml. The pairs are the watch entries of market US and JP,
// by symbol and currency of the contract, each once, in the order of the list; an entry with no con id cannot be
// resolved and is left out.
func New(
	src sources.BarSource, res store.InstrumentResolver, r bar.PriceBarReader, w reconstore.Writer,
	cmp reconapi.Func, entries []jobs.WatchEntry, clk clock.Clock,
) (contracts.Job, error) {
	if src == nil || res == nil || r == nil || w == nil || cmp == nil || clk == nil {
		return nil, errors.New("reconbars: source, resolver, reader, writer, compare function and clock are required")
	}
	m, err := DefaultManifest()
	if err != nil {
		return nil, err
	}
	j := &job{m: m, src: src, res: res, r: r, w: w, cmp: cmp, clk: clk, conID: map[sources.BarPair]int64{}}
	for _, e := range entries {
		if e.Market != "US" && e.Market != "JP" {
			continue
		}
		p := sources.BarPair{Symbol: e.Contract.Symbol, Currency: e.Contract.Currency}
		if _, dup := j.conID[p]; dup || e.Contract.ConID == 0 || p.Symbol == "" {
			continue
		}
		j.conID[p] = e.Contract.ConID
		j.pairs = append(j.pairs, p)
	}
	return j, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Schedule is the slot of the manifest as plain values.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	s := j.m.Schedule
	return s.Calendar, s.Anchor, "", s.Offset
}

// RunBound is the longest time one run may take.
func (j *job) RunBound() time.Duration { return j.m.Schedule.RunBound }

// RestartSensitive is false: a re-run is a new run, nothing is overwritten.
func (j *job) RestartSensitive() bool { return false }

// Plan returns the one key of the day, {recon.run, R6}, and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s\n%d", j.m.ID, j.m.Version, day, len(j.pairs))))
	return []contracts.ExpectedKey{{DatasetID: datasetID, SubjectKey: pairID}}, hex.EncodeToString(sum[:]), nil
}
