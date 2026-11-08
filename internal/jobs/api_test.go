package jobs_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeJob struct{ id string }

func (j fakeJob) ID() string { return j.id }
func (fakeJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
func (fakeJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	return contracts.JobResult{}
}

func TestRegistryContract(t *testing.T) {
	t.Parallel()
	var newJob jobs.NewFredSeriesFunc = func(sources.Source, store.SeriesWriter, clock.Clock) (contracts.Job, error) {
		return fakeJob{"fred_series"}, nil
	}
	j, err := newJob(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	reg := jobs.Registry{j}
	var list []contracts.Job = reg // a Registry is the slice the runner takes
	if len(list) != 1 || list[0].ID() != "fred_series" {
		t.Fatalf("registry = %v", list)
	}
}

// fakeBroker answers one empty Quote per contract asked and counts the calls.
type fakeBroker struct {
	broker.Broker
	snapshots int
}

func (b *fakeBroker) Snapshot(_ context.Context, _ broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	b.snapshots++
	return make([]broker.Quote, len(cs)), nil
}

// fakeResolver gives every key id 7.
type fakeResolver struct{ calls int }

func (r *fakeResolver) ResolveInstrument(context.Context, store.Identifier, store.InstrumentSpec, []store.Identifier) (int64, error) {
	r.calls++
	return 7, nil
}

type fakeEodWriter struct{ rows []contracts.OptionEod }

func (w *fakeEodWriter) InsertOptionEod(_ context.Context, rows []contracts.OptionEod) error {
	w.rows = append(w.rows, rows...)
	return nil
}

type fakeQuoteWriter struct{ rows []contracts.QuoteSnapshot }

func (w *fakeQuoteWriter) InsertQuoteSnapshots(_ context.Context, rows []contracts.QuoteSnapshot) error {
	w.rows = append(w.rows, rows...)
	return nil
}

// captureOf is the capture id of a key, as the design says: uuid.NewSHA1 over day|dataset|subject.
func captureOf(day contracts.Day, k contracts.ExpectedKey) jobs.CaptureID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(day)+"|"+k.DatasetID+"|"+k.SubjectKey))
}

// portJob is a fake job that works only through its ports: it plans the keys it was built with, and for each
// key asks the broker, resolves an instrument and hands a row with the capture id of the key to write.
type portJob struct {
	id      string
	dataset string
	subject []string
	br      broker.Broker
	res     store.InstrumentResolver
	write   func(context.Context, jobs.CaptureID, int64) error
}

func (j portJob) ID() string { return j.id }

func (j portJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys := make([]contracts.ExpectedKey, 0, len(j.subject))
	for _, s := range j.subject {
		keys = append(keys, contracts.ExpectedKey{DatasetID: j.dataset, SubjectKey: s})
	}
	return keys, "hash", nil
}

func (j portJob) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	var out contracts.JobResult
	for _, k := range keys {
		qs, err := j.br.Snapshot(ctx, broker.KindSnapshot, []broker.Contract{{Symbol: k.SubjectKey}})
		if err != nil {
			return contracts.JobResult{Returned: out.Returned}
		}
		id, err := j.res.ResolveInstrument(ctx, store.Identifier{Scheme: "ibkr_conid", Value: "1"}, store.InstrumentSpec{}, nil)
		if err != nil {
			return contracts.JobResult{Returned: out.Returned}
		}
		if err := j.write(ctx, captureOf(day, k), id); err != nil {
			return contracts.JobResult{Returned: out.Returned}
		}
		out.Returned += len(qs)
		out.Usable += len(qs)
	}
	return out
}

// planAndRun is the caller: it plans the job for a day, runs the planned keys, and takes the CaptureID the
// design derives for each key.
func planAndRun(t *testing.T, j contracts.Job, day contracts.Day) ([]jobs.CaptureID, contracts.JobResult) {
	t.Helper()
	keys, _, err := j.Plan(context.Background(), day)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]jobs.CaptureID, 0, len(keys))
	for _, k := range keys {
		ids = append(ids, captureOf(day, k))
	}
	return ids, j.Run(context.Background(), day, keys)
}

func TestNewOptionEodFuncContract(t *testing.T) {
	t.Parallel()
	br, res, w := &fakeBroker{}, &fakeResolver{}, &fakeEodWriter{}
	var newJob jobs.NewOptionEodFunc = func(b broker.Broker, r store.InstrumentResolver, ew store.OptionEodWriter, _ clock.Clock) (contracts.Job, error) {
		return portJob{id: "option_eod", dataset: "obs.option_eod", subject: []string{"SPY", "QQQ"}, br: b, res: r,
			write: func(ctx context.Context, c jobs.CaptureID, inst int64) error {
				return ew.InsertOptionEod(ctx, []contracts.OptionEod{{CaptureID: c.String(), InstrumentID: int(inst)}})
			}}, nil
	}
	j, err := newJob(br, res, w, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids, got := planAndRun(t, j, "2026-11-06")
	if len(ids) != 2 || ids[0] == ids[1] {
		t.Fatalf("one capture id per key expected, got %v", ids)
	}
	if got.Returned != 2 || got.Usable != 2 || br.snapshots != 2 || res.calls != 2 {
		t.Fatalf("result %+v, snapshots %d, resolves %d", got, br.snapshots, res.calls)
	}
	if len(w.rows) != 2 || w.rows[0].CaptureID != ids[0].String() || w.rows[1].CaptureID != ids[1].String() {
		t.Fatalf("rows %+v do not carry the capture ids %v", w.rows, ids)
	}
}

func TestNewQuoteSnapshotFuncContract(t *testing.T) {
	t.Parallel()
	br, res, w := &fakeBroker{}, &fakeResolver{}, &fakeQuoteWriter{}
	var newJob jobs.NewQuoteSnapshotFunc = func(b broker.Broker, r store.InstrumentResolver, qw store.QuoteSnapshotWriter, _ clock.Clock) (contracts.Job, error) {
		return portJob{id: "quote_snapshot", dataset: "obs.quote_snapshot", subject: []string{"0945"}, br: b, res: r,
			write: func(ctx context.Context, c jobs.CaptureID, inst int64) error {
				return qw.InsertQuoteSnapshots(ctx, []contracts.QuoteSnapshot{{CaptureID: c.String(), InstrumentID: int(inst)}})
			}}, nil
	}
	j, err := newJob(br, res, w, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids, got := planAndRun(t, j, "2026-11-06")
	if len(ids) != 1 || got.Returned != 1 || len(w.rows) != 1 || w.rows[0].CaptureID != ids[0].String() {
		t.Fatalf("ids %v, result %+v, rows %+v", ids, got, w.rows)
	}
	// a same-day re-run of the key writes the same capture id
	ids2, _ := planAndRun(t, j, "2026-11-06")
	if ids2[0] != ids[0] || w.rows[1].CaptureID != w.rows[0].CaptureID {
		t.Fatalf("capture id is not stable: %v vs %v", ids, ids2)
	}
}

type otherKeyJob struct{ fakeJob }

func (otherKeyJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return []contracts.ExpectedKey{{DatasetID: "obs.option_eod", SubjectKey: "QQQ"}}, "", nil
}

func TestCaptureFollowsPlannedKey(t *testing.T) {
	t.Parallel()
	ids, _ := planAndRun(t, otherKeyJob{fakeJob{"option_eod"}}, "2026-11-06")
	want := captureOf("2026-11-06", contracts.ExpectedKey{DatasetID: "obs.option_eod", SubjectKey: "QQQ"})
	if len(ids) != 1 || ids[0] != want {
		t.Fatalf("capture id %v ignores the planned key; want %v", ids, want)
	}
}

// computeJob is a fake compute_figures job: it plans one key per name given and, on Run, reads through the
// stores it was built with and answers one usable row per key.
type computeJob struct {
	stores jobs.ComputeStores
	names  []string
}

func (computeJob) ID() string { return "compute_figures" }

func (j computeJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys := make([]contracts.ExpectedKey, 0, len(j.names))
	for _, n := range j.names {
		keys = append(keys, contracts.ExpectedKey{DatasetID: "readings.reading", SubjectKey: n + "|market|VIXCLS"})
	}
	return keys, "hash", nil
}

func (j computeJob) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	var out contracts.JobResult
	for range keys {
		if _, err := j.stores.Readings.InsertReadings(ctx, []contracts.ReadingRecord{{}}); err != nil {
			return contracts.JobResult{Returned: out.Returned}
		}
		out.Returned++
		out.Usable++
	}
	return out
}

type fakeReadings struct {
	store.ReadingStore
	inserted int
}

func (r *fakeReadings) InsertReadings(_ context.Context, rows []contracts.ReadingRecord) (int, error) {
	r.inserted += len(rows)
	return len(rows), nil
}

func TestNewComputeFiguresFuncContract(t *testing.T) {
	t.Parallel()
	rd := &fakeReadings{}
	var newJob jobs.NewComputeFiguresFunc = func(s jobs.ComputeStores, _ compute.Figures, _ compute.Rules, _ compute.Units, _ compute.SeriesOps, _ clock.Clock) (contracts.Job, error) {
		return computeJob{stores: s, names: []string{"market.vix_term_ratio", "market.iv30_own"}}, nil
	}
	j, err := newJob(jobs.ComputeStores{Readings: rd}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if j.ID() != "compute_figures" {
		t.Fatalf("id = %q", j.ID())
	}
	ids, got := planAndRun(t, j, "2026-11-06")
	if len(ids) != 2 || ids[0] == ids[1] {
		t.Fatalf("one capture id per key expected, got %v", ids)
	}
	if got.Returned != 2 || got.Usable != 2 || rd.inserted != 2 {
		t.Fatalf("result %+v, inserted %d", got, rd.inserted)
	}
}
