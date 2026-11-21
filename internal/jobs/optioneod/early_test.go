package optioneod

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// stampBroker stamps every option quote with the rig clock, so a later run has a later ReceivedAt.
type stampBroker struct {
	*fakeBroker
	clk clockwork.Clock
}

func (b stampBroker) Snapshot(ctx context.Context, kind broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	qs, err := b.fakeBroker.Snapshot(ctx, kind, cs)
	for i := range qs {
		qs[i].ReceivedAt = b.clk.Now()
	}
	return qs, err
}

func TestEarlyRunStoresRowsAndEndsRetryable(t *testing.T) {
	t.Parallel()
	early := newRig(t, "2026-11-06 10:30")
	got := early.run(testDay, testKey)
	ref := newRig(t, "2026-11-06 11:00")
	want := ref.run(testDay, testKey)
	if want.Err != nil || len(ref.w.all()) == 0 {
		t.Fatalf("reference run %+v", want)
	}
	if len(early.w.all()) != len(ref.w.all()) {
		t.Fatalf("early run wrote %d rows, at-slot run %d", len(early.w.all()), len(ref.w.all()))
	}
	if kindOf(got) != contracts.Retryable || !strings.Contains(got.Err.Error(), "before_slot") ||
		!strings.HasPrefix(got.Err.Error(), "option_eod: before_slot:") {
		t.Fatalf("err %v", got.Err)
	}
	if got.Returned != want.Returned || got.Usable != want.Usable {
		t.Fatalf("early %+v, at slot %+v", got, want)
	}
}

func TestEarlyRunKeepsItsOwnError(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2026-11-06 10:30")
	r.b.chainErr = &kindErr{contracts.Permanent}
	if res := r.run(testDay, testKey); kindOf(res) != contracts.Permanent {
		t.Fatalf("result %+v", res)
	}
}

func TestEarlyRunDoesNotShareTheScheduledCapture(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2026-11-06 10:30")
	r.job.b = stampBroker{r.b, r.clk}
	var ids []string
	var received []time.Time
	var errs []contracts.TypedError
	for _, step := range []time.Duration{0, 10 * time.Minute, 20 * time.Minute} { // 10:30, 10:40, 11:00
		r.clk.Advance(step)
		before := len(r.w.all())
		res := r.run(testDay, testKey)
		rows := r.w.all()[before:]
		if len(rows) == 0 {
			t.Fatalf("no rows after +%s", step)
		}
		for _, row := range rows[1:] {
			if row.CaptureID != rows[0].CaptureID {
				t.Fatalf("one run, two captures")
			}
		}
		ids = append(ids, rows[0].CaptureID)
		received = append(received, rows[0].ReceivedAt)
		errs = append(errs, res.Err)
	}
	if ids[0] == ids[1] || ids[0] == ids[2] || ids[1] == ids[2] {
		t.Fatalf("capture ids %v are not three different ids", ids)
	}
	if want := captureID(testDay, testKey, "1100").String(); ids[2] != want {
		t.Fatalf("scheduled capture %s, want %s", ids[2], want)
	}
	if !received[2].After(received[0]) || !received[2].After(received[1]) {
		t.Fatalf("received_at %v: the run at the slot must be the newest", received)
	}
	if errs[2] != nil {
		t.Fatalf("run at the slot: %v", errs[2])
	}
}

func TestSlotInstantIsNotEarly(t *testing.T) {
	t.Parallel()
	ny, _ := time.LoadLocation("America/New_York")
	tk, _ := time.LoadLocation("Asia/Tokyo")
	cases := map[string]struct {
		run  func(t *testing.T, at time.Time) contracts.JobResult
		slot time.Time
	}{
		"option_eod": {slot: time.Date(2026, 11, 6, 11, 0, 0, 0, ny), run: func(t *testing.T, at time.Time) contracts.JobResult {
			r := newRig(t, "2026-11-06 11:00")
			r.job.clk = clockwork.NewFakeClockAt(at)
			return r.run(testDay, testKey)
		}},
		"option_eod_index": {slot: time.Date(2026, 11, 13, 11, 0, 0, 0, ny), run: func(t *testing.T, at time.Time) contracts.JobResult {
			r := newIndexRig(t, nil)
			r.job.clk = clockwork.NewFakeClockAt(at)
			return r.run("SPX")
		}},
		"option_eod_jp": {slot: time.Date(2026, 11, 9, 10, 30, 0, 0, tk), run: func(t *testing.T, at time.Time) contracts.JobResult {
			r := newJPRig(t, "2026-11-09 10:30")
			r.job.clk = clockwork.NewFakeClockAt(at)
			return r.run(jpDay, jpKey)
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if res := tc.run(t, tc.slot.Add(-time.Second)); kindOf(res) != contracts.Retryable || !strings.Contains(res.Err.Error(), "before_slot") {
				t.Fatalf("one second before the slot: %+v", res)
			}
			for _, d := range []time.Duration{0, time.Second} {
				if res := tc.run(t, tc.slot.Add(d)); res.Err != nil || res.Returned == 0 {
					t.Fatalf("slot +%s: %+v", d, res)
				}
			}
		})
	}
}
