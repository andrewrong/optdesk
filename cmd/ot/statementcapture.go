package main

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/statementcapture"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/flexsection"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

// Design: docs/design/statement-account.md#5 statement-wire.

func init() {
	registerJob(jobFamily{
		id: jobStatementCapture, order: 55,
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			j, err := a.statementCaptureJob(ctx, jc)
			return j, "", err
		},
	})
}

const (
	jobStatementCapture = "statement_capture"

	// The lines logged when statement_capture is left out; they name a key or a file, never a value.
	reasonNoStatementAlias = "the book has no statement alias"
	reasonNoBook           = accountsFileName + " is not usable"
)

// statementCaptureJob builds statement_capture over the statement aliases of the book, the section store of
// OT_PG_DSN_OPTION_LEDGER (through openFlexSections), the account store of OT_PG_DSN_ACCOUNT and the mark store of
// OT_PG_DSN_POSMARK. A book without statement alias, a book that cannot be loaded or a missing line leaves the
// job out (nil, one "job left out" line naming the line or file, no value); that is not an error. Every check
// comes before a store is opened; what it opens goes to jc.extra.
func (a *app) statementCaptureJob(ctx context.Context, jc *jobCtx) (contracts.Job, error) {
	f := *jc.f
	out := func(reason string) (contracts.Job, error) {
		a.log.Warn("job left out", "component", "ot", "job", jobStatementCapture, "status", reason)
		return nil, nil
	}
	od, ok := f.d.ext[extOptionLedger].(optionLedgerDeps)
	if !ok || od.openFlexSections == nil || f.d.openAccount == nil || f.d.openMark == nil {
		return nil, nil
	}
	book, err := a.loadBook()
	if err != nil {
		return out(reasonNoBook)
	}
	aliases := statementAliases(book)
	if len(aliases) == 0 {
		return out(reasonNoStatementAlias)
	}
	env, err := a.envFile()
	if err != nil {
		return nil, err
	}
	for _, key := range []string{store.DSNEnvOptionLedger, store.DSNEnvAccount, mark.DSNEnvPositionMark} {
		if env[key] == "" {
			return out(key + " is not set")
		}
	}
	sections, err := od.openFlexSections(ctx, env[store.DSNEnvOptionLedger])
	if err != nil {
		return out("open the section store with " + store.DSNEnvOptionLedger + " failed")
	}
	jc.extra.addCloser(sections)
	reader, ok := sections.(flexsection.Reader)
	if !ok {
		return out("the section store of " + store.DSNEnvOptionLedger + " cannot be read")
	}
	acct, err := f.d.openAccount(ctx, env[store.DSNEnvAccount])
	if err != nil {
		return out("open the account store with " + store.DSNEnvAccount + " failed")
	}
	jc.extra.addCloser(acct)
	marks, err := f.d.openMark(ctx, env[mark.DSNEnvPositionMark])
	if err != nil {
		return out("open the mark store with " + mark.DSNEnvPositionMark + " failed")
	}
	jc.extra.addCloser(marks)
	return statementcapture.New(statementcapture.Config{
		Aliases: aliases, Sections: reader, Account: acct, Marks: marks, Calendar: jc.cal, Clock: f.clk,
	})
}

// pageStatementAliases is the Statement field of the page configs: the statement aliases of the book. A book that
// cannot be loaded and the demo book give nil; the warn of siteAliasList is not repeated.
func pageStatementAliases(a *app) []string {
	book, err := a.loadBook()
	if err != nil {
		return nil
	}
	return statementAliases(book)
}
