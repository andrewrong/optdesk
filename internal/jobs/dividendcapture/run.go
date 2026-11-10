// Package dividendcapture is the dividend_capture job: once per XNYS session day it writes one
// obs.dividend_expectation row per watch entry (docs/design/event-sources.md Decisions 2, 5, 6 and 7).
package dividendcapture

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/dividend"
)

//go:embed job.toml
var manifestBytes []byte

const (
	datasetID = "obs.dividend_expectation"
	dayLayout = "2006-01-02"
)

var _ jobs.NewDividendCaptureFunc = New

type manifest struct {
	Version       int      `toml:"version"`
	ID            string   `toml:"id"`
	PassedMarkets []string `toml:"passed_markets"`
	Schedule      struct {
		Calendar         string `toml:"calendar"`
		Anchor           string `toml:"anchor"`
		Zone             string `toml:"zone"`
		Offset           string `toml:"offset"`
		RunBound         string `toml:"run_bound"`
		RestartSensitive bool   `toml:"restart_sensitive"`
	} `toml:"schedule"`
}

type job struct {
	m        manifest
	passed   map[string]bool
	offset   time.Duration
	runBound time.Duration
	ref      broker.ReferenceClient
	res      store.InstrumentResolver
	w        dividend.Writer
	r        dividend.Reader
	entries  []jobs.WatchEntry
	clk      clock.Clock
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

// New builds the dividend_capture job from the embedded job.toml, whose passed_markets key names the markets
// that are asked.
func New(
	ref broker.ReferenceClient, res store.InstrumentResolver, w dividend.Writer, r dividend.Reader,
	entries []jobs.WatchEntry, clk clock.Clock,
) (contracts.Job, error) {
	var m manifest
	if err := toml.Unmarshal(manifestBytes, &m); err != nil {
		return nil, errors.New("dividendcapture: manifest is invalid")
	}
	return newWith(ref, res, w, r, entries, clk, m.PassedMarkets)
}

// newWith is New with the passed markets given, for a test that needs another set than the manifest's.
func newWith(
	ref broker.ReferenceClient, res store.InstrumentResolver, w dividend.Writer, r dividend.Reader,
	entries []jobs.WatchEntry, clk clock.Clock, passed []string,
) (contracts.Job, error) {
	if ref == nil || res == nil || w == nil || r == nil || clk == nil {
		return nil, errors.New("dividendcapture: client, resolver, writer, reader and clock are required")
	}
	var m manifest
	if err := toml.Unmarshal(manifestBytes, &m); err != nil || m.ID == "" {
		return nil, errors.New("dividendcapture: manifest is invalid")
	}
	off, err1 := time.ParseDuration(m.Schedule.Offset)
	bound, err2 := time.ParseDuration(m.Schedule.RunBound)
	if err1 != nil || err2 != nil || m.Schedule.Calendar == "" {
		return nil, errors.New("dividendcapture: manifest: schedule is invalid")
	}
	p := map[string]bool{}
	for _, mk := range passed {
		if mk != "US" && mk != "JP" {
			return nil, errors.New("dividendcapture: passed_markets holds US or JP only")
		}
		p[mk] = true
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.Issuer == "" || seen[e.Issuer] {
			return nil, errors.New("dividendcapture: an entry needs a unique issuer")
		}
		if e.Market != "US" && e.Market != "JP" {
			return nil, errors.New("dividendcapture: market must be US or JP")
		}
		seen[e.Issuer] = true
	}
	return &job{m: m, passed: p, offset: off, runBound: bound, ref: ref, res: res, w: w, r: r, entries: entries, clk: clk}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Schedule is the slot of the manifest as plain values.
func (j *job) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	s := j.m.Schedule
	return s.Calendar, s.Anchor, s.Zone, j.offset
}

// RunBound is the longest time one run may take.
func (j *job) RunBound() time.Duration { return j.runBound }

// RestartSensitive is false: a rerun of the day finds the rows and asks nothing.
func (j *job) RestartSensitive() bool { return false }

// Plan returns one key per watch entry and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys := make([]contracts.ExpectedKey, 0, len(j.entries))
	ids := make([]string, 0, len(j.entries))
	for _, e := range j.entries {
		keys = append(keys, contracts.ExpectedKey{DatasetID: datasetID, SubjectKey: e.Issuer})
		ids = append(ids, e.Issuer)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s\n%s", j.m.ID, j.m.Version, day, strings.Join(ids, "\n"))))
	return keys, hex.EncodeToString(sum[:]), nil
}

func (j *job) entryOf(issuer string) (jobs.WatchEntry, bool) {
	for _, e := range j.entries {
		if e.Issuer == issuer {
			return e, true
		}
	}
	return jobs.WatchEntry{}, false
}

// resolve gives the instrument id of an entry through the ibkr_conid identifier.
func (j *job) resolve(ctx context.Context, e jobs.WatchEntry) (int, error) {
	c := e.Contract
	spec := store.InstrumentSpec{Type: "stock", Name: c.Symbol, Currency: c.Currency, Status: "active", PriceIncrement: "0.01", LotSize: "1"}
	id, err := j.res.ResolveInstrument(ctx, store.Identifier{Scheme: "ibkr_conid", Value: strconv.FormatInt(c.ConID, 10)}, spec, nil)
	return int(id), err
}

// Postcondition is true when a row exists for the day and the subject key.
func (j *job) Postcondition(ctx context.Context, day, dataset, subjectKey string) (bool, error) {
	if dataset != datasetID {
		return false, fmt.Errorf("dividendcapture: unknown dataset %q", dataset)
	}
	e, ok := j.entryOf(subjectKey)
	if !ok {
		return false, fmt.Errorf("dividendcapture: unknown subject %q", subjectKey)
	}
	id, err := j.resolve(ctx, e)
	if err != nil {
		id = int(e.Contract.ConID) // the key a resolve_failed row carries
	}
	rows, err := j.r.Dividends(ctx, id, contracts.Day(day), contracts.Day(day))
	if err != nil {
		return false, fmt.Errorf("dividendcapture: dividends: %w", err)
	}
	return len(rows) > 0, nil
}

type target struct {
	e  jobs.WatchEntry
	id int
}

// Run writes the rows of day that are missing. It makes one Dividends call for the entries of the passed
// markets that have no row yet and never retries; a stock that fails does not stop the others.
func (j *job) Run(ctx context.Context, day contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	sd, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return contracts.JobResult{Err: &typedError{contracts.Permanent, "dividendcapture: bad day"}}
	}
	now := j.clk.Now().UTC()
	base := func(t target) contracts.DividendExpectation {
		return contracts.DividendExpectation{
			Attempt: 1, InstrumentID: t.id, Issuer: t.e.Issuer, Currency: t.e.Contract.Currency,
			SessionDay: types.SerializableDate{Time: sd}, ReceivedAt: now, Source: "none",
		}
	}
	var rows []contracts.DividendExpectation
	var ask []target
	for _, e := range j.entries {
		t := target{e: e}
		id, rerr := j.resolve(ctx, e)
		if rerr != nil {
			// The resolve failed: the row is keyed by the con id, the key the watch list holds.
			row := base(target{e: e, id: int(e.Contract.ConID)})
			row.Status, row.ErrorCode = "error", "resolve_failed"
			rows = append(rows, row)
			continue
		}
		t.id = id
		have, herr := j.r.Dividends(ctx, id, day, day)
		if herr != nil {
			return contracts.JobResult{Err: asTyped(herr, contracts.Retryable)}
		}
		switch {
		case len(have) > 0:
		case !j.passed[e.Market]:
			row := base(t)
			row.Status = "unsupported"
			rows = append(rows, row)
		default:
			ask = append(ask, t)
		}
	}
	rows = append(rows, j.ask(ctx, ask, base)...)
	return j.write(ctx, rows)
}

// ask makes the one Dividends call and turns each answer into a row.
func (j *job) ask(ctx context.Context, ts []target, base func(target) contracts.DividendExpectation) []contracts.DividendExpectation {
	if len(ts) == 0 {
		return nil
	}
	cs := make([]broker.Contract, len(ts))
	for i, t := range ts {
		cs[i] = t.e.Contract
	}
	got, err := j.ref.Dividends(ctx, cs)
	fail := err != nil || len(got) != len(ts)
	code := "request_failed"
	if err == nil && fail {
		code = "bad_answer"
	}
	out := make([]contracts.DividendExpectation, 0, len(ts))
	for i, t := range ts {
		row := base(t)
		if fail {
			row.Status, row.ErrorCode = "error", code
		} else {
			row = fill(row, got[i])
		}
		out = append(out, row)
	}
	return out
}

func text(v *float64) *string {
	if v == nil {
		return nil
	}
	s := strconv.FormatFloat(*v, 'f', -1, 64)
	return &s
}

// fill sets the row from one answer: present needs a parsable ex-date and an amount, else it is no_value.
func fill(row contracts.DividendExpectation, p broker.DividendProjection) contracts.DividendExpectation {
	row.Source = "ibkr_tick_456"
	switch p.Status {
	case broker.DividendOK:
		var ex time.Time
		perr := errors.New("no date")
		if p.NextExDate != nil {
			ex, perr = time.Parse("20060102", *p.NextExDate)
		}
		if perr != nil || p.NextAmount == nil {
			row.Status = "no_value"
			return row
		}
		row.Status = "present"
		row.Past12M, row.Next12M, row.NextAmount = text(p.Past12m), text(p.Next12m), text(p.NextAmount)
		row.NextExDate = &types.SerializableDate{Time: ex}
	case broker.DividendNoValue:
		row.Status = "no_value"
	case broker.DividendTimeout:
		row.Status, row.ErrorCode = "error", "timeout"
	case broker.DividendError:
		fallthrough
	default:
		row.Status, row.ErrorCode = "error", "error_status"
		if p.ErrorCode != 0 {
			row.ErrorCode = "ibkr_" + strconv.Itoa(p.ErrorCode)
		}
	}
	return row
}

// write inserts every row, keeps going after a failed insert and reports the first one as Retryable.
func (j *job) write(ctx context.Context, rows []contracts.DividendExpectation) contracts.JobResult {
	res := contracts.JobResult{}
	for _, row := range rows {
		if err := j.w.InsertDividend(ctx, row); err != nil {
			if res.Err == nil {
				res.Err = asTyped(err, contracts.Retryable)
			}
			continue
		}
		res.Returned++
		if row.Status == "present" || row.Status == "no_value" {
			res.Usable++
		}
	}
	return res
}
