package main

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optioncandidates"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/candidatecur"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// Design: docs/design/candidates-jp.md#7 (Decisions 1, 4, 13 and 27).

const (
	// jobOptionCandidatesJP is the id of the Japan candidates family.
	jobOptionCandidatesJP = "option_candidates_jp"

	// reasonJPHeld is the one line that names the hold: the scheduler lists the family as disabled with it until
	// the schedule entry of candidates-jp removes the startOff. run-once is not held by it.
	reasonJPHeld = "option_candidates_jp is held in the scheduler until the schedule entry of candidates-jp"
	// reasonJPNoOpen and reasonJPNoWriter are the lines logged when the job is left out.
	reasonJPNoOpen   = openquote.DSNEnvOptionOpen + " is not set or the option open store could not be opened"
	reasonJPNoRule   = "no currency rule is set in deps"
	reasonJPNoWriter ="the candidate store cannot write the currency of a row"
)

func init() {
	// Order 183: after option_open_jp (180). startOff holds the family in the scheduler once the open line is set
	// (a build with no open line leaves it out anyway); ot run-once --job option_candidates_jp builds it.
	registerJob(jobFamily{
		id: jobOptionCandidatesJP, order: 183,
		startOff: ifOpenDSN(func(runOnceConfig, bool) string { return reasonJPHeld }),
		build: func(_ context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			return buildCandidatesJP(a, jc)
		},
	})
}

// buildCandidatesJP builds option_candidates_jp over the stores option_candidates opened (the factory of the run
// holds them): the open store, the candidate writer through candidatecur.Writer, the account store and the JP
// entries of the list. Without the open store or a writer that stores the currency the job is left out with one
// line and the others still run.
func buildCandidatesJP(a *app, jc *jobCtx) (contracts.Job, string, error) {
	f := jc.f
	left := func(key, why string) (contracts.Job, string, error) {
		a.log.Warn("job left out", "component", "ot", "job", jobOptionCandidatesJP, "status", why)
		if key != "" {
			jc.notWired.record(jobOptionCandidatesJP, key)
		}
		return nil, "", nil
	}
	if f.d.newCurrencyRule == nil {
		return left("", reasonJPNoRule)
	}
	if f.candOpen == nil {
		return left(openquote.DSNEnvOptionOpen, reasonJPNoOpen)
	}
	rows, ok := f.candidates.(candidatecur.Writer)
	if !ok {
		return left(store.DSNEnvCandidate, reasonJPNoWriter)
	}
	st := optioncandidates.JPStores{
		CandidateStores: optioncandidates.CandidateStores{
			OptionEod: f.compute.OptionEod, Catalog: f.compute.Catalog, Terms: f.compute.Terms, Names: f.compute.Names,
			Series: f.compute.Series, Earnings: f.earnRd, Accounts: f.accounts, Aliases: f.candAliases,
			SessionDays: f.compute.SessionDays, Open: f.candOpen,
		},
		Rows: rows,
	}
	j, err := f.listed(groupOptions, func(entries []jobs.WatchEntry) (contracts.Job, error) {
		j, err := optioncandidates.NewJP(st, f.d.newCurrencyRule(), entries, f.clk)
		if err != nil {
			return nil, fmt.Errorf("build %s job failed: %w", jobOptionCandidatesJP, err)
		}
		return j, nil
	})
	if err != nil {
		return nil, "", err
	}
	return j, "", nil
}
