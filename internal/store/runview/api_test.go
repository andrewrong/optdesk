package runview_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

// fake keeps the promises of the interfaces in memory: the caller side of the contract.
type fake struct {
	runs    []runview.Row
	reasons []runview.Reason
}

var (
	_ runview.Reader       = (*fake)(nil)
	_ runview.ReasonWriter = (*fake)(nil)
)

func (f *fake) RunsOn(_ context.Context, day contracts.Day) ([]runview.Row, error) {
	out := []runview.Row{}
	for _, r := range f.runs {
		if r.SessionDay == day {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Dataset != out[j].Dataset {
			return out[i].Dataset < out[j].Dataset
		}
		return out[i].Key < out[j].Key
	})
	return out, nil
}

func (f *fake) ReasonsOn(_ context.Context, day contracts.Day) ([]runview.Reason, error) {
	out := []runview.Reason{}
	for _, r := range f.reasons {
		if r.Day == day {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Dataset != b.Dataset {
			return a.Dataset < b.Dataset
		}
		if a.Subject != b.Subject {
			return a.Subject < b.Subject
		}
		return a.Scope < b.Scope
	})
	return out, nil
}

func (f *fake) NotWired(ctx context.Context) ([]runview.Reason, error) {
	out := []runview.Reason{}
	for _, r := range f.reasons {
		if r.Day == runview.NotWiredDay && r.Dataset == runview.NotWiredDataset {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Scope < out[j].Scope })
	return out, nil
}

func (f *fake) ReplaceReasons(_ context.Context, day contracts.Day, dataset, subject string, rs []runview.Reason) error {
	kept := f.reasons[:0:0]
	for _, r := range f.reasons {
		if !(r.Day == day && r.Dataset == dataset && r.Subject == subject) {
			kept = append(kept, r)
		}
	}
	f.reasons = append(kept, rs...)
	return nil
}

func TestRunviewContractBothSides(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	at := time.Date(2026, 11, 12, 1, 0, 0, 0, time.UTC)
	f := &fake{runs: []runview.Row{
		{SessionDay: "2026-11-12", Dataset: "plan.option_open_item", Key: "us", Status: "done", FinishedAt: at, Returned: 3, Usable: 2, ReasonCode: "no_chain"},
		{SessionDay: "2026-11-11", Dataset: "obs.option_open", Key: "us", Status: "done"},
	}}
	var r runview.Reader = f
	var w runview.ReasonWriter = f
	runs, err := r.RunsOn(ctx, "2026-11-12")
	if err != nil || len(runs) != 1 || runs[0].Usable != 2 || runs[0].ReasonCode != "no_chain" {
		t.Fatalf("runs %+v err %v", runs, err)
	}
	if runview.NotWiredDay != "1970-01-01" || runview.NotWiredDataset != "ops.not_wired" {
		t.Fatal("the not wired key moved")
	}
	if err := w.ReplaceReasons(ctx, runview.NotWiredDay, runview.NotWiredDataset, "scheduler", []runview.Reason{
		{Day: runview.NotWiredDay, Dataset: runview.NotWiredDataset, Subject: "scheduler", Scope: "b", Code: "not_wired", Detail: "OT_X", At: at},
		{Day: runview.NotWiredDay, Dataset: runview.NotWiredDataset, Subject: "scheduler", Scope: "a", Code: "not_wired", Detail: "OT_Y", At: at},
	}); err != nil {
		t.Fatal(err)
	}
	nw, _ := r.NotWired(ctx)
	if len(nw) != 2 || nw[0].Scope != "a" || nw[1].Scope != "b" {
		t.Fatalf("not wired %+v", nw)
	}
	if on, _ := r.ReasonsOn(ctx, "2026-11-12"); len(on) != 0 {
		t.Fatalf("a day reads only its own reasons: %+v", on)
	}
}

func TestReasonsOnNeverNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var r runview.Reader = &fake{}
	if got, _ := r.ReasonsOn(ctx, "2026-11-12"); got == nil || len(got) != 0 {
		t.Fatalf("ReasonsOn = %#v", got)
	}
	if got, _ := r.RunsOn(ctx, "2026-11-12"); got == nil || len(got) != 0 {
		t.Fatalf("RunsOn = %#v", got)
	}
	if got, _ := r.NotWired(ctx); got == nil || len(got) != 0 {
		t.Fatalf("NotWired = %#v", got)
	}
}

func TestReplaceReasonsClearsOld(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &fake{}
	day, ds := contracts.Day("2026-11-12"), "plan.option_open_item"
	rs := func(code string) []runview.Reason {
		return []runview.Reason{{Day: day, Dataset: ds, Subject: "us", Scope: "AAPL", Code: code}}
	}
	other := runview.Reason{Day: day, Dataset: ds, Subject: "jp", Scope: "7203", Code: "no_close"}
	_ = f.ReplaceReasons(ctx, day, ds, "jp", []runview.Reason{other})
	_ = f.ReplaceReasons(ctx, day, ds, "us", rs("no_chain"))
	_ = f.ReplaceReasons(ctx, day, ds, "us", rs("no_list"))
	got, _ := f.ReasonsOn(ctx, day)
	if len(got) != 2 || got[0].Subject != "jp" || got[1].Code != "no_list" {
		t.Fatalf("after replace %+v", got)
	}
	_ = f.ReplaceReasons(ctx, day, ds, "us", nil)
	got, _ = f.ReasonsOn(ctx, day)
	if len(got) != 1 || got[0] != other {
		t.Fatalf("a clean run must clear only its own subject: %+v", got)
	}
}
