package positionmarks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

const (
	datasetID = "account.position_mark"
	runKey    = "daily"

	// batchSize is below the 70 lines of the gateway (DEFAULT_LINES in pyside).
	batchSize = 60
	// budget is the wall-clock time after which no further batch is started; it is below the 60 s
	// SnapshotTimeout of the broker client so that the last batch still ends inside it.
	budget = 45 * time.Second
)

var _ NewPositionMarksFunc = New

type job struct {
	m       manifest
	market  broker.Broker
	aliases []string
	acct    store.AccountReader
	w       mark.Writer
	clk     clock.Clock
	log     *slog.Logger

	mu   sync.Mutex
	last map[string]mark.PositionMark // newest row written by this job per (capture, con id)
}

type typedError struct {
	kind contracts.ErrorKind
	text string
}

func (e *typedError) Error() string             { return e.text }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

// fail is a typed error whose text is the job id, a status word and the alias only (Decision 6).
func fail(kind contracts.ErrorKind, status, alias string) contracts.TypedError {
	t := "position_marks: status " + status
	if alias != "" {
		t += " alias " + alias
	}
	return &typedError{kind: kind, text: t}
}

// New builds the position_marks job from the embedded job.toml. market is the one market-data gateway,
// aliases the accounts whose positions are marked, the main one first. When w also reads (mark.Reader), the
// retry reads the newest rows from it; otherwise it keeps the rows this job wrote.
func New(market broker.Broker, aliases []string, acct store.AccountReader, w mark.Writer, clk clock.Clock) (contracts.Job, error) {
	return newJob(market, aliases, acct, w, clk, slog.Default())
}

func newJob(market broker.Broker, aliases []string, acct store.AccountReader, w mark.Writer, clk clock.Clock, log *slog.Logger) (*job, error) {
	if market == nil || acct == nil || w == nil || clk == nil {
		return nil, errors.New("positionmarks: market, account reader, writer and clock are required")
	}
	if len(aliases) == 0 {
		return nil, errors.New("positionmarks: at least one alias is required")
	}
	m, err := parseManifest(manifestBytes)
	if err != nil {
		return nil, err
	}
	return &job{
		m: m, market: market, aliases: append([]string(nil), aliases...), acct: acct, w: w, clk: clk, log: log,
		last: map[string]mark.PositionMark{},
	}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Plan expects one key per day; the hash covers the job, the day and the aliases.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys := []contracts.ExpectedKey{{DatasetID: datasetID, SubjectKey: runKey}}
	sum := sha256.Sum256([]byte(strings.Join([]string{j.m.ID, string(day), strings.Join(j.aliases, ",")}, "\n")))
	return keys, hex.EncodeToString(sum[:]), nil
}

// outcome is what one alias adds to the result.
type outcome struct {
	rows, priced int
	quotes, down int  // quotes asked and those answered disconnected
	unreached    int  // positions left without a row by the budget
	hasMarks     bool // some position of the alias has a priced mark (this run or an earlier one)
	positions    int
	stop         bool // the gateway call or the write failed: the run ends whichever alias met it
	err          contracts.TypedError
}

// Run marks every alias, the main one first. Returned counts the rows written and Usable those with a price.
// Only the main alias without marks, the market-data gateway down, or positions not reached end it with an
// error; another alias that has no capture or whose positions are not marked is logged and skipped.
func (j *job) Run(ctx context.Context, day contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	start := j.clk.Now()
	var res contracts.JobResult
	var quotes, down, unreached int
	setErr := func(e contracts.TypedError) {
		if res.Err == nil {
			res.Err = e
		}
	}
	for i, alias := range j.aliases {
		o := j.runAlias(ctx, alias, string(day), start)
		res.Returned += o.rows
		res.Usable += o.priced
		quotes += o.quotes
		down += o.down
		unreached += o.unreached
		main := i == 0
		switch {
		case o.err != nil && (main || o.stop):
			setErr(o.err)
		case o.err != nil:
			j.log.Warn("alias skipped", "alias", alias, "kind", string(o.err.Kind()))
		case main && o.positions > 0 && !o.hasMarks:
			setErr(fail(contracts.Retryable, "no_marks", alias))
		}
		if o.stop {
			break
		}
	}
	if quotes > 0 && down == quotes {
		setErr(fail(contracts.Retryable, "disconnected", ""))
	}
	if unreached > 0 {
		setErr(fail(contracts.Retryable, "partial", ""))
	}
	return res
}

// runAlias marks one alias: the newest capture of the day, the positions that still need a mark, in batches.
func (j *job) runAlias(ctx context.Context, alias, day string, start time.Time) outcome {
	var o outcome
	ps, err := j.acct.Positions(ctx, alias, day)
	if err != nil {
		o.err = fail(contracts.Retryable, "error", alias)
		return o
	}
	ps = newestCapture(ps)
	if len(ps) == 0 {
		o.err = fail(contracts.Retryable, "no_capture", alias)
		return o
	}
	o.positions = len(ps)
	prior, err := j.prior(ctx, alias, day, ps[0].CaptureID)
	if err != nil {
		// Without the stored rows the attempt number is unknown: a guess would collide with a stored key.
		o.err = fail(contracts.Retryable, "error", alias)
		return o
	}
	var todo []store.Position
	attempt := map[string]int{}
	for _, p := range ps {
		k := key(p.CaptureID, p.ConID)
		pm, ok := prior[k]
		switch {
		case !ok:
			attempt[k] = 1
			todo = append(todo, p)
		case pm.Basis == mark.BasisNone:
			attempt[k] = pm.Attempt + 1
			todo = append(todo, p)
		default:
			o.hasMarks = true
		}
	}
	for from := 0; from < len(todo); from += batchSize {
		if j.clk.Now().Sub(start) >= budget {
			o.unreached = len(todo) - from
			return o
		}
		batch := todo[from:min(from+batchSize, len(todo))]
		rows, quoted, down, err := j.markBatch(ctx, alias, batch, attempt)
		if err != nil {
			o.err, o.stop = err, true
			return o
		}
		if err := j.w.WriteMarks(ctx, rows); err != nil {
			o.err, o.stop = fail(contracts.Retryable, "write", alias), true
			return o
		}
		j.remember(rows)
		o.quotes += quoted
		o.down += down
		for _, r := range rows {
			o.rows++
			if r.Basis != mark.BasisNone {
				o.priced++
				o.hasMarks = true
			}
		}
	}
	return o
}

// markBatch plans the batch, asks the market-data gateway once and returns one row per position.
func (j *job) markBatch(ctx context.Context, alias string, batch []store.Position, attempt map[string]int) (
	rows []mark.PositionMark, quoted, down int, _ contracts.TypedError) {
	ts := make([]target, len(batch))
	var cs []broker.Contract
	for i, p := range batch {
		ts[i] = planTarget(ctx, j.market, p)
		if ts[i].status == "" {
			cs = append(cs, ts[i].contract)
		}
	}
	var qs []broker.Quote
	if len(cs) > 0 {
		var err error
		qs, err = j.market.Snapshot(ctx, broker.KindSnapshot, cs)
		if err != nil {
			j.log.Warn("snapshot failed", "alias", alias)
			return nil, 0, 0, fail(contracts.Retryable, "disconnected", alias)
		}
		if len(qs) != len(cs) {
			return nil, 0, 0, fail(contracts.Permanent, "error", alias)
		}
	}
	now := j.clk.Now().UTC()
	qi := 0
	for _, t := range ts {
		r := mark.PositionMark{
			CaptureID: t.pos.CaptureID, Alias: alias, MarkedAt: now, SessionDay: t.pos.SessionDay, ConID: t.pos.ConID,
			Attempt: attempt[key(t.pos.CaptureID, t.pos.ConID)], Currency: t.pos.Currency, Basis: mark.BasisNone, Status: t.status,
		}
		if t.status == "" {
			q := qs[qi]
			qi++
			quoted++
			r.Status = statusWord(q.Status)
			r.MarketDataType = q.MarketDataType
			r.Bid, r.Ask, r.Last, r.Close = text(q.Bid), text(q.Ask), text(q.Last), text(q.Close)
			if q.Status == broker.StatusDisconnected {
				down++
			}
			if q.Status == broker.StatusOK {
				r.Price, r.Basis = MarkOf(q)
			}
		}
		rows = append(rows, r)
	}
	return rows, quoted, down, nil
}

func text(v *float64) string {
	if v == nil {
		return ""
	}
	return dec(*v)
}

// prior returns the newest row of each position of the capture: from the store when the writer also reads
// (a read error is returned, never replaced by the memory of this instance), else what this job wrote.
func (j *job) prior(ctx context.Context, alias, day, captureID string) (map[string]mark.PositionMark, error) {
	out := map[string]mark.PositionMark{}
	if rd, ok := j.w.(mark.Reader); ok {
		rows, err := rd.Marks(ctx, alias, day)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if r.CaptureID == captureID {
				out[key(r.CaptureID, r.ConID)] = r
			}
		}
		return out, nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	for k, r := range j.last {
		if r.CaptureID == captureID {
			out[k] = r
		}
	}
	return out, nil
}

func (j *job) remember(rows []mark.PositionMark) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, r := range rows {
		j.last[key(r.CaptureID, r.ConID)] = r
	}
}
