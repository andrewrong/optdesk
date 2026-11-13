package computefigures_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/readingkey"
)

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

// computeJob is a fake compute_figures job: it plans one key per name given and, on Run, reads through the
// stores it was built with and answers one usable row per key.
type computeJob struct {
	stores computefigures.ComputeStores
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
		if _, err := j.stores.Readings.(readingkey.ReadingUpserter).UpsertReadings(ctx, []contracts.ReadingRecord{{}}); err != nil {
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

func (r *fakeReadings) UpsertReadings(_ context.Context, rows []contracts.ReadingRecord) (readingkey.WriteCount, error) {
	r.inserted += len(rows)
	return readingkey.WriteCount{Written: len(rows)}, nil
}

func TestNewComputeFiguresFuncContract(t *testing.T) {
	t.Parallel()
	rd := &fakeReadings{}
	var newJob computefigures.NewComputeFiguresFunc = func(
		s computefigures.ComputeStores, _ compute.Figures, _ compute.Rules, _ compute.Units, _ compute.SeriesOps, _ clock.Clock,
	) (contracts.Job, error) {
		return computeJob{stores: s, names: []string{"market.vix_term_ratio", "market.iv30_own"}}, nil
	}
	j, err := newJob(computefigures.ComputeStores{Readings: rd}, nil, nil, nil, nil, nil)
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

// easeJob is a fake compute_figures job built with an Ease: it adds the ease key only when the Ease is not nil.
type easeJob struct {
	computeJob
	ease compute.Ease
}

func (j easeJob) Plan(ctx context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys, h, _ := j.computeJob.Plan(ctx, day)
	if j.ease != nil {
		keys = append(keys, contracts.ExpectedKey{DatasetID: "readings.reading", SubjectKey: "macro.ease|market|WALCL"})
	}
	return keys, h, nil
}

type stubEase struct{}

func (stubEase) Ease(string, compute.EaseSeries, contracts.Day) compute.EaseFigure {
	return compute.EaseFigure{}
}

func TestNewComputeFiguresEaseFuncContract(t *testing.T) {
	t.Parallel()
	var newJob computefigures.NewComputeFiguresEaseFunc = func(
		s computefigures.ComputeStores, _ compute.Figures, _ compute.Rules, _ compute.Units, _ compute.SeriesOps,
		e compute.Ease, _ clock.Clock,
	) (contracts.Job, error) {
		return easeJob{computeJob{stores: s, names: []string{"market.iv30_own"}}, e}, nil
	}
	for _, tc := range []struct {
		ease compute.Ease
		want int
	}{{nil, 1}, {stubEase{}, 2}} {
		rd := &fakeReadings{}
		j, err := newJob(computefigures.ComputeStores{Readings: rd}, nil, nil, nil, nil, tc.ease, nil)
		if err != nil {
			t.Fatal(err)
		}
		ids, got := planAndRun(t, j, "2026-11-06")
		if len(ids) != tc.want || got.Returned != tc.want || got.Usable != tc.want || rd.inserted != tc.want {
			t.Fatalf("ease %v: ids %v, result %+v, inserted %d, want %d keys", tc.ease, ids, got, rd.inserted, tc.want)
		}
	}
}

func TestNewComputeFiguresEaseFuncEaseBeforeClock(t *testing.T) {
	t.Parallel()
	var f computefigures.NewComputeFiguresEaseFunc = func(computefigures.ComputeStores, compute.Figures, compute.Rules,
		compute.Units, compute.SeriesOps, compute.Ease, clock.Clock) (contracts.Job, error) {
		return nil, nil
	}
	_ = f
}

// TestTechDepsFromBothSides: a caller fills Classify and Params, the reader reads them back.
func TestTechDepsFromBothSides(t *testing.T) {
	t.Parallel()
	d := computefigures.TechDeps{
		Classify: func(in blockapi.TechIn) blockapi.TechResult {
			return blockapi.TechResult{State: string(in.Day), Rank: in.Params.Fold}
		},
		Params: blockapi.TechParams{Band: "1.00", Hard: "5.00", Short: 21, Long: 63, Fold: 10, MaxAgeDays: 5},
	}
	got := d.Classify(blockapi.TechIn{Day: "2026-11-13", Params: d.Params})
	if got.State != "2026-11-13" || got.Rank != 10 || d.Params.Band != "1.00" || d.Params.Hard != "5.00" ||
		d.Params.Short != 21 || d.Params.Long != 63 || d.Params.MaxAgeDays != 5 {
		t.Fatalf("got %+v, params %+v", got, d.Params)
	}
}

func TestWithTechStubReturnsTheSameJob(t *testing.T) {
	t.Parallel()
	j := computeJob{names: []string{"market.iv30_own"}}
	deps := computefigures.TechDeps{Classify: func(blockapi.TechIn) blockapi.TechResult { return blockapi.TechResult{} }}
	got := computefigures.WithTech(j, deps)
	gj, ok := got.(computeJob)
	if !ok || len(gj.names) != 1 || gj.names[0] != "market.iv30_own" {
		t.Fatalf("got %#v", got)
	}
}
