// Package underlyingresolve is the underlying_resolve job: it resolves each pending name of the underlying store
// once through the broker and settles it (docs/design/underlyings.md, Decision 7).
package underlyingresolve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

type job struct {
	m   Manifest
	b   broker.Broker
	s   underlying.Settler
	clk clock.Clock
}

type typedError struct {
	kind contracts.ErrorKind
	msg  string
}

func (e *typedError) Error() string             { return e.msg }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func asTyped(err error, kind contracts.ErrorKind) contracts.TypedError {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te
	}
	return &typedError{kind, err.Error()}
}

// New builds the underlying_resolve job from the embedded job.toml.
func New(b broker.Broker, s underlying.Settler, clk clock.Clock) (contracts.Job, error) {
	if b == nil || s == nil || clk == nil {
		return nil, errors.New("underlyingresolve: broker, settler and clock are required")
	}
	m, err := DefaultManifest()
	if err != nil {
		return nil, err
	}
	return &job{m: m, b: b, s: s, clk: clk}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Schedule is the slot of the manifest as plain values.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	s := j.m.Schedule
	return s.Calendar, s.Anchor, s.Zone, s.Offset
}

// RunBound is the longest time one run may take.
func (j *job) RunBound() time.Duration { return j.m.Schedule.RunBound }

// RestartSensitive is false: a rerun reads the store again and asks only what is still pending.
func (j *job) RestartSensitive() bool { return false }

// due reads the rows to ask: pending, and unresolved for at least retry_days, at most max_names.
func (j *job) due(ctx context.Context) ([]underlying.Row, error) {
	before := j.clk.Now().Add(-time.Duration(j.m.RetryDays) * 24 * time.Hour)
	rows, err := j.s.Pending(ctx, before)
	if err != nil {
		return nil, err
	}
	if len(rows) > j.m.MaxNames {
		rows = rows[:j.m.MaxNames]
	}
	return rows, nil
}

// Plan returns one key per name to ask, at most max_names, and the hash of the plan.
func (j *job) Plan(ctx context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	rows, err := j.due(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("underlyingresolve: pending: %w", err)
	}
	keys := make([]contracts.ExpectedKey, 0, len(rows))
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		keys = append(keys, contracts.ExpectedKey{DatasetID: DatasetID, SubjectKey: r.Key})
		ids = append(ids, r.Key)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s\n%s", j.m.ID, j.m.Version, day, strings.Join(ids, "\n"))))
	return keys, hex.EncodeToString(sum[:]), nil
}

// Run asks the broker once for each key that is still due and settles it. Returned counts the names asked; Usable
// counts those that need no retry: settled, or changed by the owner meanwhile (Settle wrote nothing, no error). A
// broker or store error settles nothing for that name, the others go on, and the first error is the result's.
func (j *job) Run(ctx context.Context, _ contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	rows, err := j.due(ctx)
	if err != nil {
		return contracts.JobResult{Err: asTyped(err, contracts.Retryable)}
	}
	byKey := make(map[string]underlying.Row, len(rows))
	for _, r := range rows {
		byKey[r.Key] = r
	}
	var res contracts.JobResult
	for i, k := range keys {
		if i >= j.m.MaxNames {
			break
		}
		row, ok := byKey[k.SubjectKey]
		if !ok {
			continue // settled or changed since Plan: nothing to ask
		}
		res.Returned++
		if err := j.resolve(ctx, row); err != nil {
			if res.Err == nil {
				res.Err = asTyped(err, contracts.Retryable)
			}
			continue
		}
		res.Usable++
	}
	return res
}

// resolve asks for one name and settles the answer.
func (j *job) resolve(ctx context.Context, r underlying.Row) error {
	exchange := r.Exchange
	if exchange == "" {
		exchange = "SMART"
	}
	got, err := j.b.ResolveContract(ctx, broker.Contract{
		SecType: "STK", Symbol: r.Symbol, Exchange: exchange, Currency: r.Currency,
	})
	if err != nil {
		return fmt.Errorf("resolve %s: %w", r.Key, err)
	}
	o := underlying.Outcome{Key: r.Key, AfterID: r.ID}
	switch {
	case !got.Listed:
		o.Reason = reason("not listed", got)
	case got.Contract.ConID <= 0:
		o.Reason = reason("ambiguous", got)
	default:
		o.ConID = got.Contract.ConID
	}
	// written false: the owner changed the name meanwhile; that is no error and nothing is retried.
	if _, err := j.s.Settle(ctx, o); err != nil {
		return fmt.Errorf("settle %s: %w", r.Key, err)
	}
	return nil
}

func reason(what string, got broker.Resolved) string {
	if got.ErrorText == "" {
		return what
	}
	return what + ": " + got.ErrorText
}
