package optioneod_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optioneod"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
)

type fakeJob struct{ id string }

func (j fakeJob) ID() string { return j.id }
func (fakeJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
func (fakeJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	return contracts.JobResult{}
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

// captureOf is the capture id of a key, as the design says: uuid.NewSHA1 over day|dataset|subject.
func captureOf(day contracts.Day, k contracts.ExpectedKey) jobs.CaptureID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(day)+"|"+k.DatasetID+"|"+k.SubjectKey))
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

func TestNewOptionEodFuncContract(t *testing.T) {
	t.Parallel()
	br, res, w := &fakeBroker{}, &fakeResolver{}, &fakeEodWriter{}
	var newJob optioneod.NewOptionEodFunc = func(b broker.Broker, r store.InstrumentResolver, ew store.OptionEodWriter, _ clock.Clock) (contracts.Job, error) {
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

func TestOptionEodWatchFuncOrder(t *testing.T) {
	t.Parallel()
	var _ optioneod.NewOptionEodWatchFunc = func(
		broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []jobs.WatchEntry, clock.Clock,
	) (contracts.Job, error) {
		return nil, nil
	}
	var _ optioneod.NewOptionEodHeldFunc = func(
		broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []jobs.WatchEntry, store.AccountReader, clock.Clock,
	) (contracts.Job, error) {
		return nil, nil
	}
}

func TestOptionEodWindowContract(t *testing.T) {
	t.Parallel()
	var zero optioneod.OptionEodWindow
	if zero.MinDTE != 0 || zero.MaxDTE != 0 || zero.MaxStrikes != 0 {
		t.Fatal("the zero window keeps the manifest")
	}
	w := optioneod.OptionEodWindow{MinDTE: 21, MaxDTE: 45, MaxStrikes: 24}
	var f optioneod.NewOptionEodWatchWindowFunc = func(
		broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []jobs.WatchEntry, optioneod.OptionEodWindow, clock.Clock,
	) (contracts.Job, error) {
		return nil, errors.New(strconv.Itoa(w.MinDTE))
	}
	if _, err := f(nil, nil, nil, nil, w, nil); err == nil || err.Error() != "21" {
		t.Fatalf("the window is the fifth argument, got %v", err)
	}
}

type fakeTranslator struct{}

func (fakeTranslator) ToVendor(s string, _ symbol.Vendor) (string, error) { return s, nil }
func (fakeTranslator) OSIRoot(s string) string                            { return s }

// TestSymbolWatchCtorType: NewOptionEodWatchSymbolsFunc takes the translator last; nil means none.
func TestSymbolWatchCtorType(t *testing.T) {
	t.Parallel()
	var got []symbol.Translator
	var watch optioneod.NewOptionEodWatchSymbolsFunc = func(
		_ broker.Broker, _ store.InstrumentResolver, _ store.OptionEodWriter, _ []jobs.WatchEntry, _ clock.Clock, tr symbol.Translator,
	) (contracts.Job, error) {
		got = append(got, tr)
		return fakeJob{id: "option_eod"}, nil
	}
	for _, tr := range []symbol.Translator{fakeTranslator{}, nil} {
		if j, err := watch(nil, nil, nil, nil, nil, tr); err != nil || j.ID() != "option_eod" {
			t.Fatalf("option_eod watch: %v %v", j, err)
		}
	}
	if len(got) != 2 || got[0] != (fakeTranslator{}) || got[1] != nil {
		t.Fatalf("translators: %v", got)
	}
}

// TestSymbolCtorTypes: the constructor carries the translator as the last argument, and a nil translator
// is accepted (no translation).
func TestSymbolCtorTypes(t *testing.T) {
	t.Parallel()
	var got []symbol.Translator
	var eod optioneod.NewOptionEodSymbolsFunc = func(
		_ broker.Broker, _ store.InstrumentResolver, _ store.OptionEodWriter, _ clock.Clock, tr symbol.Translator,
	) (contracts.Job, error) {
		got = append(got, tr)
		return fakeJob{id: "option_eod"}, nil
	}
	for _, tr := range []symbol.Translator{fakeTranslator{}, nil} {
		if j, err := eod(nil, nil, nil, nil, tr); err != nil || j.ID() != "option_eod" {
			t.Fatalf("option_eod: %v %v", j, err)
		}
	}
	if len(got) != 2 || got[0] != (fakeTranslator{}) || got[1] != nil {
		t.Fatalf("translators passed through: %v", got)
	}
}
