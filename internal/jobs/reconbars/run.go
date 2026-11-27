package reconbars

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/reconapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	reconstore "github.com/andrewrong/optdesk/internal/store/recon"
)

const (
	basisTrades  = "trades"
	dayLayout    = "2006-01-02"
	windowDays   = 6 // the run day and the 6 days before it hold 5 weekdays
	statusOK     = "ok"
	statusFailed = "failed"
	statusPart   = "partial"
)

// Run reads our winner rows of the window that ends at day, makes one DailyBars call, compares and writes one run.
// A day is any day: no force and no time guard. A run killed through ctx writes nothing. A source error writes a
// run of status failed with its reason code and no items, and the result carries the error.
func (j *job) Run(ctx context.Context, day contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	to, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "bad day")}
	}
	from := contracts.Day(to.AddDate(0, 0, -windowDays).Format(dayLayout))
	started := j.clk.Now()
	ours, terr := j.readOurs(ctx, from, day)
	if terr != nil {
		return contracts.JobResult{Err: terr}
	}
	run := reconstore.Run{
		PairID: pairID, SessionDay: day, StartedAt: started, PairsAsked: len(j.pairs),
		ToleranceText: j.m.ToleranceText, SourceID: j.m.SourceID,
	}
	var answer sources.BarResult
	var srcErr error
	if len(j.pairs) > 0 {
		dctx, cancel := context.WithDeadline(ctx, started.Add(j.m.Schedule.RunBound-deadlineMargin))
		answer, srcErr = j.src.DailyBars(dctx, sources.BarRequest{Pairs: j.pairs, From: from, To: day})
		timedOut := dctx.Err() != nil
		cancel()
		if ctx.Err() != nil {
			return contracts.JobResult{Err: newErr(contracts.Retryable, "run was cut: %v", ctx.Err())}
		}
		if srcErr != nil && timedOut {
			srcErr = &deadlineError{srcErr}
		}
	}
	if srcErr != nil {
		run.Status, run.Reason = statusFailed, failureCode(srcErr)
		run.Items = []reconstore.Item{}
		run.FinishedAt = j.clk.Now()
		if _, err := j.w.InsertRun(ctx, run); err != nil {
			return contracts.JobResult{Err: asTyped(err, contracts.Retryable)}
		}
		return contracts.JobResult{Err: asTyped(srcErr, contracts.Retryable)}
	}
	in, unanswered := j.input(ours, answer)
	out := j.cmp(in)
	run.PairsCompared = run.PairsAsked - unanswered
	run.Status, run.Reason = statusOK, ""
	if len(answer.Unanswered) > 0 {
		run.Status = statusPart
		for _, u := range answer.Unanswered {
			if u.Code != sources.BarCodeNoData {
				run.Reason = u.Code
				break
			}
		}
	}
	run.N, run.Matched, run.Diff, run.MissingOurs = out.N, out.Matched, out.Diff, out.MissingOurs
	run.MissingOther, run.Skipped = out.MissingOther, out.Skipped
	run.Items = items(out.Items)
	run.FinishedAt = j.clk.Now()
	if _, err := j.w.InsertRun(ctx, run); err != nil {
		return contracts.JobResult{Err: asTyped(err, contracts.Retryable)}
	}
	return contracts.JobResult{Returned: run.N + run.Skipped, Usable: run.N}
}

// deadlineError marks a source error that came after the deadline of the call; its code is timeout.
type deadlineError struct{ error }

func (e *deadlineError) Unwrap() error { return e.error }
func (e *deadlineError) Code() string  { return sources.BarCodeTimeout }

// codeError is an error that names its reason code itself.
type codeError interface{ Code() string }

// failureCode is the reason code of a source error: the code the error names when it is one of the four, timeout
// for a deadline, else network.
func failureCode(err error) string {
	var ce codeError
	if errors.As(err, &ce) {
		switch c := ce.Code(); c {
		case sources.BarCodeRateLimited, sources.BarCodeNetwork, sources.BarCodeTimeout, sources.BarCodeSourceChanged:
			return c
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return sources.BarCodeTimeout
	}
	return sources.BarCodeNetwork
}

func keyOf(p sources.BarPair, day contracts.Day) string {
	return p.Symbol + "|" + p.Currency + "|" + string(day)
}

// readOurs reads the winner rows of basis trades of every pair in the window, as keyed values.
func (j *job) readOurs(ctx context.Context, from, to contracts.Day) (map[sources.BarPair][]reconapi.Value, contracts.TypedError) {
	out := make(map[sources.BarPair][]reconapi.Value, len(j.pairs))
	for _, p := range j.pairs {
		con := j.conID[p]
		spec := store.InstrumentSpec{Type: "stock", Name: p.Symbol, Currency: p.Currency, Status: "active", PriceIncrement: "0.01", LotSize: "1"}
		id, err := j.res.ResolveInstrument(ctx, store.Identifier{Scheme: "ibkr_conid", Value: strconv.FormatInt(con, 10)}, spec, nil)
		if err != nil {
			return nil, asTyped(err, contracts.Permanent)
		}
		rows, err := j.r.PriceBars(ctx, id, basisTrades, from, to)
		if err != nil {
			return nil, asTyped(err, contracts.Retryable)
		}
		for _, r := range rows {
			d := contracts.Day(r.SessionDay.Format(dayLayout))
			if d < from || d > to {
				continue
			}
			out[p] = append(out[p], reconapi.Value{Key: keyOf(p, d), Amount: r.Close})
		}
	}
	return out, nil
}

// input builds the compare input; it returns the count of unanswered pairs.
func (j *job) input(ours map[sources.BarPair][]reconapi.Value, a sources.BarResult) (reconapi.Input, int) {
	in := reconapi.Input{
		Skip: map[string]string{}, OtherMissingSkipped: true, Tol: reconapi.Tolerance{Rel: j.m.CloseRel},
	}
	for _, p := range j.pairs {
		in.Ours = append(in.Ours, ours[p]...)
	}
	for _, b := range a.Bars {
		in.Other = append(in.Other, reconapi.Value{Key: keyOf(b.Pair, b.Day), Amount: b.Close})
	}
	unanswered := map[sources.BarPair]bool{}
	for _, u := range a.Unanswered {
		unanswered[u.Pair] = true
		for _, v := range ours[u.Pair] {
			in.Skip[v.Key] = u.Code
		}
	}
	return in, len(unanswered)
}

// severity orders the kinds as the reader of the page does.
var severity = map[reconapi.Kind]int{reconapi.Diff: 0, reconapi.MissingOurs: 1, reconapi.MissingOther: 2, reconapi.Skipped: 3}

// items keeps the items that are not match, the most severe first and at most RunItemsMax; the company is the symbol.
func items(all []reconapi.Item) []reconstore.Item {
	out := make([]reconstore.Item, 0, len(all))
	kept := make([]reconapi.Item, 0, len(all))
	for _, it := range all {
		if it.Kind != reconapi.Match {
			kept = append(kept, it)
		}
	}
	sort.SliceStable(kept, func(a, b int) bool { return severity[kept[a].Kind] < severity[kept[b].Kind] })
	for _, it := range kept {
		if len(out) == reconstore.RunItemsMax {
			break
		}
		out = append(out, reconstore.Item{
			Key: it.Key, Company: symbolOf(it.Key), Ours: it.Ours, Other: it.Other, Diff: it.Diff,
			Kind: string(it.Kind), Reason: it.Reason,
		})
	}
	return out
}

func symbolOf(key string) string {
	for i := range len(key) {
		if key[i] == '|' {
			return key[:i]
		}
	}
	return key
}
