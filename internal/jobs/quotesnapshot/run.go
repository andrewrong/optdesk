package quotesnapshot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ NewQuoteSnapshotFunc = New

type job struct {
	m   manifest
	b   broker.Broker
	res store.InstrumentResolver
	w   store.QuoteSnapshotWriter
	clk clock.Clock

	cal         string
	open, close time.Duration
}

type typedError struct {
	kind contracts.ErrorKind
	err  error
}

func (e *typedError) Error() string             { return e.err.Error() }
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

// asTyped returns err unchanged when it is a TypedError, else wraps it with kind.
func asTyped(err error, kind contracts.ErrorKind) contracts.TypedError {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te
	}
	return &typedError{kind: kind, err: err}
}

func fail(kind contracts.ErrorKind, text string) contracts.TypedError {
	return &typedError{kind: kind, err: errors.New(text)}
}

// New builds the quote_snapshot job from the embedded job.toml.
func New(b broker.Broker, res store.InstrumentResolver, w store.QuoteSnapshotWriter, clk clock.Clock) (contracts.Job, error) {
	if b == nil || res == nil || w == nil || clk == nil {
		return nil, errors.New("quotesnapshot: broker, resolver, writer and clock are required")
	}
	return build(manifestBytes, b, res, w, clk)
}

func build(data []byte, b broker.Broker, res store.InstrumentResolver, w store.QuoteSnapshotWriter, clk clock.Clock) (*job, error) {
	m, err := parseManifest(data)
	if err != nil {
		return nil, err
	}
	cal, open, closeOff, err := windowOf(m)
	if err != nil {
		return nil, err
	}
	return &job{m: m, b: b, res: res, w: w, clk: clk, cal: cal, open: open, close: closeOff}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// captureID is the deterministic capture id of one key.
func captureID(day contracts.Day, subject string) jobs.CaptureID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(day)+"|"+datasetID+"|"+subject))
}

// Run collects every slot key of keys in sequence and stops at the first error.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	var total contracts.JobResult
	for _, k := range keys {
		r := j.runSlot(ctx, day, k.SubjectKey)
		total.Returned += r.Returned
		total.Usable += r.Usable
		if r.Err != nil {
			total.Err = r.Err
			return total
		}
	}
	return total
}

func (j *job) runSlot(ctx context.Context, _ contracts.Day, slot string) contracts.JobResult {
	// Development mode: a run outside its slot or on another day stores the clock's session day.
	day := contracts.Day(j.clk.Now().In(j.m.loc).Format(time.DateOnly))
	sessionDay, err := time.Parse(time.DateOnly, string(day))
	if err != nil {
		return contracts.JobResult{Err: fail(contracts.Permanent, fmt.Sprintf("bad day %q", day))}
	}
	capture := captureID(day, slot).String()
	start := j.clk.Now()
	var res contracts.JobResult
	ins := j.m.Instruments
	for from := 0; from < len(ins); from += j.m.chunk() {
		if j.clk.Now().Sub(start) >= j.m.budget() {
			res.Err = fail(contracts.Retryable, "budget spent")
			return res
		}
		chunk := ins[from:min(from+j.m.chunk(), len(ins))]
		rows, err := j.collect(ctx, chunk, sessionDay, slot, capture)
		if err != nil {
			res.Err = err
			return res
		}
		if err := j.w.InsertQuoteSnapshots(ctx, rows); err != nil {
			res.Usable = 0
			res.Err = asTyped(err, contracts.Retryable)
			return res
		}
		res.Returned += len(rows)
		for _, r := range rows {
			if r.Status == statusOK {
				res.Usable++
			}
		}
	}
	if res.Usable == 0 {
		res.Err = fail(contracts.NoValue, "no quote")
	}
	return res
}

// collect resolves the ids of a chunk, asks the broker once and maps the answers to rows.
func (j *job) collect(ctx context.Context, chunk []instrument, day time.Time, slot, capture string) ([]contracts.QuoteSnapshot, contracts.TypedError) {
	ids := make([]int64, len(chunk))
	cs := make([]broker.Contract, len(chunk))
	for i, in := range chunk {
		id, err := j.res.ResolveInstrument(ctx,
			store.Identifier{Scheme: "ibkr_conid", Value: strconv.FormatInt(in.ConID, 10)}, specOf(in), nil)
		if err != nil {
			return nil, asTyped(err, contracts.Retryable)
		}
		ids[i] = id
		cs[i] = broker.Contract{SecType: in.SecType, Symbol: in.Symbol, Exchange: in.Exchange, Currency: in.Currency, ConID: in.ConID}
	}
	qs, err := j.b.Snapshot(ctx, broker.KindSnapshot, cs)
	if err != nil {
		return nil, asTyped(err, contracts.Retryable)
	}
	if len(qs) != len(cs) {
		return nil, fail(contracts.Permanent, fmt.Sprintf("snapshot answered %d quotes for %d contracts", len(qs), len(cs)))
	}
	rows := make([]contracts.QuoteSnapshot, len(qs))
	for i, q := range qs {
		rows[i] = mapQuote(q, day, slot, capture, chunk[i].Currency, ids[i])
	}
	return rows, nil
}

func specOf(in instrument) store.InstrumentSpec {
	typ := "stock"
	if in.SecType == "IND" {
		typ = "index"
	}
	return store.InstrumentSpec{
		Type: typ, Name: in.Symbol, Currency: in.Currency, MIC: in.MIC, CalendarID: in.CalendarID,
		Status: "active", PriceIncrement: in.PriceIncrement, LotSize: in.LotSize,
	}
}
