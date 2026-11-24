package optioneod

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

func TestChunkSkipsRefusedContract(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2026-11-06 11:00")
	r.res.refuse = map[string]error{"951": newErr(contracts.Permanent, "osi names instrument 7")}
	var buf bytes.Buffer
	r.job.logger = slog.New(slog.NewTextHandler(&buf, nil))
	rs := &runState{j: r.job, start: r.clk.Now()}
	u, _ := r.job.underlyingOf("TEST")
	mk := func(strike string, id int64) broker.Contract {
		return broker.Contract{Symbol: "TEST", Expiry: "20261106", Right: "C", Strike: strike, Multiplier: "100", TradingClass: "TEST", ConID: id}
	}
	var res contracts.JobResult
	var tl tally
	terr := rs.chunk(context.Background(), u, 1, 100, false, jobs.CaptureID{},
		[]broker.Contract{mk("95", 950), mk("96", 951), mk("97", 952)}, &res, &tl)
	if terr != nil {
		t.Fatal(terr)
	}
	if len(r.w.all()) != 2 || res.Returned != 2 || len(r.b.snaps) != 1 || len(r.b.snaps[0].cs) != 2 {
		t.Fatalf("rows %d result %+v snaps %+v", len(r.w.all()), res, r.b.snaps)
	}
	if len(tl.skipped) != 1 || tl.skipped[0].osi != "TEST  261106C00096000" || !strings.Contains(tl.skipped[0].text, "instrument 7") {
		t.Fatalf("skipped %+v", tl.skipped)
	}
	logged := buf.String()
	if strings.Count(logged, "level=WARN") != 1 || !strings.Contains(logged, "TEST  261106C00096000") || !strings.Contains(logged, "instrument 7") {
		t.Fatalf("log %q", logged)
	}
}

func TestChunkRetryableResolverErrorStillFails(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2026-11-06 11:00")
	r.res.refuse = map[string]error{"950": newErr(contracts.Retryable, "db down")}
	rs := &runState{j: r.job, start: r.clk.Now()}
	u, _ := r.job.underlyingOf("TEST")
	c := broker.Contract{Symbol: "TEST", Expiry: "20261106", Right: "C", Strike: "95", Multiplier: "100", TradingClass: "TEST", ConID: 950}
	var res contracts.JobResult
	terr := rs.chunk(context.Background(), u, 1, 100, false, jobs.CaptureID{}, []broker.Contract{c}, &res, &tally{})
	if terr == nil || terr.Kind() != contracts.Retryable || len(r.w.all()) != 0 {
		t.Fatalf("terr %v", terr)
	}
}

func TestRunKeyKeepsChainAfterRefusedContract(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2026-11-06 17:00")
	r.res.refuse = map[string]error{"950": newErr(contracts.Permanent, "osi names instrument 7")}
	rw := &recordingReasons{}
	r.job.reasons = rw
	res := r.run(testDay, testKey)
	if res.Err != nil || res.Returned != 3 || res.Usable != 3 || len(r.w.all()) != 3 {
		t.Fatalf("result %+v rows %d", res, len(r.w.all()))
	}
	if len(rw.calls) != 1 {
		t.Fatalf("reason writes %d", len(rw.calls))
	}
	var got []runview.Reason
	for _, row := range rw.calls[0].rows {
		if row.Code == codeNoConID {
			got = append(got, row)
		}
	}
	if len(got) != 1 || got[0].Scope != "contract:TEST  261106C00095000" || !strings.Contains(got[0].Detail, "instrument 7") {
		t.Fatalf("no_con_id rows %+v", got)
	}
}
