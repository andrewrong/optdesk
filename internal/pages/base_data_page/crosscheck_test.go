package base_data_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/compute/reconapi"
	reconstore "github.com/andrewrong/optdesk/internal/store/recon"
)

// testDay is the last closed session of both calendars at testNow.
const testDay = "2026-11-17"

type fakeRecon struct {
	runs []reconstore.Run
	err  error
}

func (f fakeRecon) Latest(context.Context) ([]reconstore.Run, error) { return f.runs, f.err }
func (fakeRecon) Close() error                                       { return nil }

// verdictOf records its calls and answers level.
type verdictOf struct {
	level basecheckapi.Level
	calls *[]reconapi.Output
}

func (v verdictOf) Func() reconapi.VerdictFunc {
	return func(_ string, o reconapi.Output, _, _ int) basecheckapi.Level {
		if v.calls != nil {
			*v.calls = append(*v.calls, o)
		}
		return v.level
	}
}

func reconInput(runs []reconstore.Run, level basecheckapi.Level, calls *[]reconapi.Output) Input {
	in := defaultInput(fakeEval{})
	in.Recon = fakeRecon{runs: runs}
	in.Verdict = verdictOf{level: level, calls: calls}.Func()
	return in
}

func r6Run() reconstore.Run {
	return reconstore.Run{PairID: "R6", SessionDay: testDay, Status: "ok",
		FinishedAt: time.Date(2026, 11, 17, 23, 5, 0, 0, time.UTC),
		N:          90, Matched: 88, Diff: 1, MissingOurs: 1, Skipped: 10, PairsAsked: 100, PairsCompared: 90,
		ToleranceText: "收盘价相差不超过 0.01%", SourceID: "yfinance",
		Items: []reconstore.Item{
			{Key: "2026-11-12", Company: "NVDA", Ours: "140.00", Other: "70.00", Diff: "70.00", Kind: "diff"},
			{Key: "2026-11-13", Company: "AAPL", Ours: "", Other: "230.10", Diff: "", Kind: "missing_ours"},
			{Key: "2026-11-13", Company: "7203.T", Kind: "skipped", Reason: "source_lacks"},
		}}
}

func crossOf(t *testing.T, in Input) map[string]any {
	t.Helper()
	m, _ := build(t, in)
	for _, g := range m["groups"].([]any) {
		for _, r := range g.(map[string]any)["rows"].([]any) {
			if row := r.(map[string]any); row["dataset"] == "bars" {
				cc, _ := row["cross_check"].(map[string]any)
				return cc
			}
		}
	}
	t.Fatal("no bars row")
	return nil
}

// TestNoRunIsUnknown: a day with no run is unknown with reason no_run, and the verdict function is not asked.
func TestNoRunIsUnknown(t *testing.T) {
	t.Parallel()
	var calls []reconapi.Output
	cc := crossOf(t, reconInput(nil, basecheckapi.Pass, &calls))
	if cc["level"] != "unknown" || cc["reason"] != "no_run" || cc["reason_zh"] != "尚未运行" || cc["run_at"] != nil {
		t.Fatalf("cross_check = %v", cc)
	}
	if len(calls) != 0 {
		t.Fatalf("Verdict called %d times", len(calls))
	}
	other := r6Run()
	other.SessionDay = "2026-11-16"
	if cc := crossOf(t, reconInput([]reconstore.Run{other}, basecheckapi.Pass, nil)); cc["reason"] != "no_run" {
		t.Fatalf("a run of another day counted: %v", cc)
	}
}

// TestFailedRunIsUnknown: a failed run is unknown with its code and the Chinese name of the code, with no call.
func TestFailedRunIsUnknown(t *testing.T) {
	t.Parallel()
	for code, zh := range map[string]string{"rate_limited": "被限流", "network": "网络错误", "timeout": "超时", "source_changed": "数据源格式变了"} {
		var calls []reconapi.Output
		run := r6Run()
		run.Status, run.Reason = "failed", code
		cc := crossOf(t, reconInput([]reconstore.Run{run}, basecheckapi.Pass, &calls))
		if cc["level"] != "unknown" || cc["reason"] != code || cc["reason_zh"] != "未能比对："+zh {
			t.Fatalf("%s: cross_check = %v", code, cc)
		}
		if len(calls) != 0 {
			t.Fatalf("%s: Verdict called", code)
		}
	}
}

// TestPartialRunCarriesReason: a partial run with a reason keeps the level of Verdict and carries the reason.
func TestPartialRunCarriesReason(t *testing.T) {
	t.Parallel()
	run := r6Run()
	run.Status, run.Reason = "partial", "rate_limited"
	cc := crossOf(t, reconInput([]reconstore.Run{run}, basecheckapi.Warn, nil))
	if cc["level"] != "warn" || cc["reason"] != "rate_limited" || cc["reason_zh"] != "未能比对：被限流" {
		t.Fatalf("cross_check = %v", cc)
	}
}

// TestSkippedItemsShownWithReason: a skipped item stays in the list as kind skipped, its reason in Chinese in the hint.
func TestSkippedItemsShownWithReason(t *testing.T) {
	t.Parallel()
	cc := crossOf(t, reconInput([]reconstore.Run{r6Run()}, basecheckapi.Pass, nil))
	items := cc["items"].([]any)
	skipped := items[2].(map[string]any)
	if skipped["kind"] != "skipped" || skipped["hint_zh"] != "未比对：数据源没有这一天" || skipped["company"] != "7203.T" {
		t.Fatalf("item = %v", skipped)
	}
	diff := items[0].(map[string]any)
	if diff["hint_zh"] != "两个来源不一致，先看是否有拆股尚未重述；两边都已按拆股调整，都不含分红调整" || diff["company"] != "NVDA" {
		t.Fatalf("item = %v", diff)
	}
	if cc["skipped"] != float64(10) || cc["missing"] != float64(1) || cc["n"] != float64(90) {
		t.Fatalf("counts = %v", cc)
	}
	if got := cc["tolerance_text"].(string); !strings.Contains(got, "已比对 90 个，共 100 个") {
		t.Fatalf("tolerance_text = %s", got)
	}
}

// TestFirstTwentyItems: 30 items stored give the first 20, in order.
func TestFirstTwentyItems(t *testing.T) {
	t.Parallel()
	run := r6Run()
	run.Items = nil
	for i := 0; i < 30; i++ {
		run.Items = append(run.Items, reconstore.Item{Key: fmt.Sprintf("k%02d", i), Company: "NVDA", Kind: "diff"})
	}
	items := crossOf(t, reconInput([]reconstore.Run{run}, basecheckapi.Fail, nil))["items"].([]any)
	if len(items) != 20 || items[0].(map[string]any)["key"] != "k00" || items[19].(map[string]any)["key"] != "k19" {
		t.Fatalf("items = %d", len(items))
	}
}

// TestR6Golden: the cross_check of a fixed run equals testdata/cross_check_r6.json, and the page is valid.
func TestR6Golden(t *testing.T) {
	t.Parallel()
	cc := crossOf(t, reconInput([]reconstore.Run{r6Run()}, basecheckapi.Warn, nil))
	b, err := json.Marshal(cc)
	if err != nil {
		t.Fatal(err)
	}
	jsonEqual(t, b, "cross_check_r6.json")
}

// TestReaderFailureLeavesCrossCheckOut: a failed reader gives no cross_check and no Pairs.
func TestReaderFailureLeavesCrossCheckOut(t *testing.T) {
	t.Parallel()
	var got basecheckapi.Input
	in := reconInput(nil, basecheckapi.Pass, nil)
	in.Eval = fakeEval{got: &got}.Func()
	in.Recon = fakeRecon{err: errors.New("down")}
	if cc := crossOf(t, in); cc != nil || len(got.Pairs) != 0 {
		t.Fatalf("cross_check %v pairs %v", cc, got.Pairs)
	}
}
