package ivhistory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	datasetReadings = "readings.reading"
	kindUnderlying  = "underlying"
	dayLayout       = "2006-01-02"
)

type typedError struct {
	kind contracts.ErrorKind
	err  error
}

func (e *typedError) Error() string             { return e.err.Error() }
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func newErr(kind contracts.ErrorKind, format string, a ...any) contracts.TypedError {
	return &typedError{kind: kind, err: fmt.Errorf("ivhistory: "+format, a...)}
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
	m        Manifest
	h        broker.HistoryClient
	r        store.ReadingReader
	w        store.ReadingWriter
	ivText   func(close string) (string, error)
	clk      clock.Clock
	names    []string // sorted
	contract map[string]broker.Contract
}

// New builds the iv_history job from the embedded job.toml: one key per watch entry, named by the symbol of
// its contract. ivText turns the close of a bar into the stored text of the reading, or an error for a close
// the reading refuses (cmd/ot passes ivbar.FromClose); the job imports no compute package.
func New(
	h broker.HistoryClient, r store.ReadingReader, w store.ReadingWriter, entries []jobs.WatchEntry,
	ivText func(close string) (string, error), clk clock.Clock,
) (contracts.Job, error) {
	if h == nil || r == nil || w == nil || ivText == nil || clk == nil {
		return nil, errors.New("ivhistory: history client, reader, writer, text function and clock are required")
	}
	m, err := DefaultManifest()
	if err != nil {
		return nil, err
	}
	j := &job{m: m, h: h, r: r, w: w, ivText: ivText, clk: clk, contract: map[string]broker.Contract{}}
	for _, e := range entries {
		name := e.Contract.Symbol
		if name == "" || strings.ContainsAny(name, "|") {
			return nil, errors.New("ivhistory: an entry needs a symbol without |")
		}
		if _, dup := j.contract[name]; dup {
			return nil, fmt.Errorf("ivhistory: duplicate symbol %q", name)
		}
		j.contract[name] = e.Contract
		j.names = append(j.names, name)
	}
	sort.Strings(j.names)
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

// RestartSensitive is false: a rerun inserts only what is missing.
func (j *job) RestartSensitive() bool { return false }

func (j *job) keyOf(name string) contracts.ExpectedKey {
	return contracts.ExpectedKey{DatasetID: datasetReadings, SubjectKey: j.m.Reading + "|" + kindUnderlying + "|" + name}
}

// nameOf is the name of a key of this job.
func (j *job) nameOf(k contracts.ExpectedKey) (string, bool) {
	name, ok := strings.CutPrefix(k.SubjectKey, j.m.Reading+"|"+kindUnderlying+"|")
	_, known := j.contract[name]
	return name, ok && known
}

// Plan lists one key market.iv_ibkr|underlying|<name> per name, sorted, and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys := make([]contracts.ExpectedKey, 0, len(j.names))
	h := sha256.New()
	fmt.Fprintf(h, "%s\n%d\n%s\n", j.m.ID, j.m.Version, day)
	for _, n := range j.names {
		k := j.keyOf(n)
		keys = append(keys, k)
		fmt.Fprintf(h, "%s\n", k.SubjectKey)
	}
	return keys, hex.EncodeToString(h.Sum(nil)), nil
}
