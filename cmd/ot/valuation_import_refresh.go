package main

import (
	"context"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/valuation_import_page"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

// Design: docs/design/buy-price-source.md#Decisions (Decision 1). A confirmed import changes the buy price the gate
// uses only at the next option_candidates run; the confirm is therefore a trigger of that run, for a market whose
// run of the day has finished. No rule of the gate changes.

const (
	candidatesDataset = "candidates.option_candidate"

	textRefreshNotYet  = " of this day has not run yet; it will use the new valuation"
	textRefreshQueued  = " of this day is queued to run again with the new valuation"
	textRefreshNoRuns  = "the valuation is written; the run of this day could not be read, press Run in the job console"
	textRefreshNoQueue = "the valuation is written; the gate run could not be queued, press Run in the job console"
)

// gateJob is one market's job and the issuers of the lines inserted for it.
type gateJob struct {
	id      string
	issuers map[string]bool
}

// refreshGate gives one outcome per market with an inserted line, US before JP, keyed by the job id. The inserted
// lines are the plan rows whose outcome in applied is inserted. A job whose run of day has finished is queued with
// force; one that has not run yet is left to its run of the day. Nothing is given when the console or the run store
// is not wired. A failure becomes a failed line with a fixed text and never a status or an error.
func (p importPost) refreshGate(ctx context.Context, plan valueimportapi.Plan, applied []valuation_import_page.Outcome, day contracts.Day) []valuation_import_page.Outcome {
	openers, ok := jobRequestOpenersOf(p.a.deps)
	if !ok || p.a.deps.openRuns == nil || p.rd.underlying == nil {
		return nil
	}
	inserted := map[string]bool{}
	for _, o := range applied {
		if o.Outcome == valuation_import_page.OutcomeInserted {
			inserted[o.Key] = true
		}
	}
	rows, err := p.rd.underlying.Current(ctx)
	if err != nil {
		return nil
	}
	us := gateJob{id: jobOptionCandidates, issuers: map[string]bool{}}
	jp := gateJob{id: jobOptionCandidatesJP, issuers: map[string]bool{}}
	for _, r := range plan.Rows {
		if !inserted[rowLabel(r)] {
			continue
		}
		for _, u := range rows {
			if !strings.EqualFold(u.Symbol, r.Ticker) || u.Issuer == "" {
				continue
			}
			switch u.Market {
			case "US":
				us.issuers[u.Issuer] = true
			case "JP":
				jp.issuers[u.Issuer] = true
			}
			break
		}
	}
	var out []valuation_import_page.Outcome
	for _, j := range []gateJob{us, jp} {
		if len(j.issuers) > 0 {
			out = append(out, p.refreshJob(ctx, openers, j, day))
		}
	}
	return out
}

func (p importPost) refreshJob(ctx context.Context, openers jobRequestOpeners, j gateJob, day contracts.Day) valuation_import_page.Outcome {
	line := func(outcome, text string) valuation_import_page.Outcome {
		return valuation_import_page.Outcome{Key: j.id, Outcome: outcome, Text: text}
	}
	failedRuns := line(valuation_import_page.OutcomeFailed, textRefreshNoRuns)
	finished, ok := p.finishedRun(ctx, j, day)
	switch {
	case !ok:
		return failedRuns
	case !finished:
		return line(valuation_import_page.OutcomeSame, j.id+textRefreshNotYet)
	}
	failedQueue := line(valuation_import_page.OutcomeFailed, textRefreshNoQueue)
	_, dsn, reason, err := p.a.ownerEditSecrets()
	if err != nil || reason != "" || openers.Poster == nil {
		return failedQueue
	}
	st, err := openers.Poster(ctx, dsn)
	if err != nil {
		return failedQueue
	}
	defer func() { _ = st.Close() }()
	res, err := st.Enqueue(ctx, jobrequest.Request{JobID: j.id, Day: string(day), Force: true})
	if err != nil || (res.Outcome != jobrequest.EnqueueCreated && res.Outcome != jobrequest.EnqueueExisting) {
		return failedQueue
	}
	return line(valuation_import_page.OutcomeInserted, j.id+textRefreshQueued)
}

// finishedRun reports whether a candidates run of an issuer of j has finished on day; ok is false when the run
// store could not be opened or read.
func (p importPost) finishedRun(ctx context.Context, j gateJob, day contracts.Day) (finished, ok bool) {
	env, err := p.a.envFile()
	if err != nil || env[pagesDSNName] == "" {
		return false, false
	}
	s, err := p.a.deps.openRuns(ctx, env[pagesDSNName])
	if err != nil {
		return false, false
	}
	defer func() { _ = s.Close() }()
	runs, err := runview.Reader(s).RunsOn(ctx, day)
	if err != nil {
		return false, false
	}
	for _, r := range runs {
		if r.Dataset == candidatesDataset && !r.FinishedAt.IsZero() && j.issuers[r.Key] {
			return true, true
		}
	}
	return false, true
}
