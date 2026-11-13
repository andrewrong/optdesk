package main

import (
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/optioneod"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

// Design: docs/design/option-field-reasons.md#decisions (Decision 9, the composition root).
//
// The run store is already opened by the wiring: openLedgerWithReasons opens it with the runner DSN and closes it
// with the ledger (the writer, nil when the ledger is not one: dry run), and openOpsTodayRuns opens it with the
// pages DSN among the site readers (the reader, absent in the demo profile). This file only hands both to the
// option_eod jobs and to the option data page.

// withOptionEodReasons wraps a built option_eod or option_eod_jp job (and its error) with the reason writer w. A nil
// w, or a failed build, returns the job and the error as they came.
func withOptionEodReasons(j contracts.Job, err error, w runview.ReasonWriter) (contracts.Job, error) {
	if err != nil || w == nil {
		return j, err
	}
	return optioneod.WithReasons(j, w)
}

// optionReasonReader is the reader of stored reasons the site opened (nil: the demo profile or no run store).
func optionReasonReader(rd siteReaders) runview.Reader {
	r, _ := siteReaderOf[runview.Reader](rd, siteOpsTodayKey)
	return r
}
