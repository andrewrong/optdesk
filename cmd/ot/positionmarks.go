package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/positionmarks"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
	"github.com/andrewrong/optdesk/internal/store/markpg"
)

// markAliases returns the aliases position_marks marks: the labels of OT_ACCOUNT_SOCKETS, the main one first and
// the others sorted, the same aliases account_capture captures.
func markAliases(sockets map[string]string, main string) []string {
	rest := make([]string, 0, len(sockets))
	for l := range sockets {
		if l != main {
			rest = append(rest, l)
		}
	}
	sort.Strings(rest)
	return append([]string{main}, rest...)
}

// positionMarks builds position_marks (Decision 6 of docs/design/position-marks.md): the market-data gateway
// brk, the aliases of the account job, the account store as reader and the mark store opened with
// store/mark's DSNEnvPositionMark as writer. It is nil for a deps without openMark (a test deps literal; newDeps
// always sets it). Every check comes before a store is opened; errors name the key only. What it opens goes to extra.
func (a *app) positionMarks(ctx context.Context, f jobFactory, brk broker.Broker, extra *closerSet) (contracts.Job, error) {
	if f.d.openMark == nil || f.d.newPositionMarks == nil {
		return nil, nil
	}
	env, err := a.envFile()
	if err != nil {
		return nil, err
	}
	sockets, err := parseAccountSockets(env[envAccountSockets])
	if err != nil {
		return nil, err
	}
	main, err := accountMain(env[envIbkrGateways])
	if err != nil {
		return nil, err
	}
	if _, ok := sockets[main]; !ok {
		return nil, fmt.Errorf("the main label of %s is not in %s", envIbkrGateways, envAccountSockets)
	}
	acctDSN, markDSN := env[store.DSNEnvAccount], env[mark.DSNEnvPositionMark]
	if acctDSN == "" {
		return nil, fmt.Errorf("%s is not set", store.DSNEnvAccount)
	}
	if markDSN == "" {
		return nil, fmt.Errorf("%s is not set", mark.DSNEnvPositionMark)
	}
	acct, err := f.d.openAccount(ctx, acctDSN)
	if err != nil {
		return nil, fmt.Errorf("open account store with %s failed", store.DSNEnvAccount)
	}
	extra.addCloser(acct)
	w, err := f.d.openMark(ctx, markDSN)
	if err != nil {
		return nil, fmt.Errorf("open mark store with %s failed", mark.DSNEnvPositionMark)
	}
	extra.addCloser(w)
	j, err := f.d.newPositionMarks(brk, markAliases(sockets, main), acct, w, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build position_marks job failed: %w", err)
	}
	return j, nil
}

// realMarkDeps sets the real constructors of position_marks: markpg.Open and positionmarks.New.
func realMarkDeps(d *deps) {
	d.openMark, d.newPositionMarks = markpg.Open, positionmarks.New
}

// withAccountJobs appends account_capture and then position_marks (ten minutes after it) to jobs, each when wired.
func (a *app) withAccountJobs(ctx context.Context, f jobFactory, brk broker.Broker, extra *closerSet, jobs []contracts.Job) ([]contracts.Job, error) {
	acct, err := a.accountCapture(ctx, f, extra)
	if err != nil {
		return jobs, err
	}
	if acct != nil {
		jobs = append(jobs, acct)
	}
	marks, err := a.positionMarks(ctx, f, brk, extra)
	if err != nil || marks == nil {
		return jobs, err
	}
	return append(jobs, marks), nil
}
