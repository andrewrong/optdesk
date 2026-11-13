package main

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/positionmarks"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
	"github.com/andrewrong/optdesk/internal/store/markpg"
)

// newPositionMarksFunc is the constructor of the position_marks job (positionmarks.NewPositionMarksFunc), named here
// so that wire.go need not import the job package.
type newPositionMarksFunc = positionmarks.NewPositionMarksFunc

// positionMarks builds position_marks (Decision 6 of docs/design/position-marks.md): the market-data gateway
// brk, the aliases of the account job, the account store as reader and the mark store opened with
// store/mark's DSNEnvPositionMark as writer. It is nil for a deps without openMark (a test deps literal; newDeps
// always sets it). An unset OT_PG_DSN_POSMARK leaves only this job out, with a warning.
// Every check comes before a store is opened; errors name the key only. What it opens goes to extra.
func (a *app) positionMarks(ctx context.Context, f jobFactory, brk broker.Broker, extra *closerSet) (contracts.Job, error) {
	if f.d.openMark == nil || f.d.newPositionMarks == nil {
		return nil, nil
	}
	aliases, acctDSN, markDSN, err := a.markSettings()
	if err != nil {
		return nil, err
	}
	if markDSN == "" {
		// A deploy that has not yet set the new line keeps every other job: only position_marks is left out.
		a.log.Warn("job left out", "component", "ot", "job", "position_marks", "status", mark.DSNEnvPositionMark+" is not set")
		return nil, nil
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
	j, err := f.d.newPositionMarks(brk, aliases, acct, w, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build position_marks job failed: %w", err)
	}
	return j, nil
}

// realMarkDeps sets the real constructors of position_marks: markpg.Open and positionmarks.New.
func realMarkDeps(d *deps) {
	d.openMark, d.newPositionMarks = markpg.Open, positionmarks.New
}

// markSettings reads the lines position_marks needs: the aliases of the account job (main first) and the two DSNs.
// markDSN is empty when its line is unset (the caller leaves the job out). It opens nothing; an error names the key only.
func (a *app) markSettings() (aliases []string, acctDSN, markDSN string, err error) {
	env, err := a.envFile()
	if err != nil {
		return nil, "", "", err
	}
	book, err := a.accountBook()
	if err != nil {
		return nil, "", "", err
	}
	acctDSN, markDSN = env[store.DSNEnvAccount], env[mark.DSNEnvPositionMark]
	if acctDSN == "" {
		return nil, "", "", fmt.Errorf("%s is not set", store.DSNEnvAccount)
	}
	return bookAliases(book), acctDSN, markDSN, nil
}
