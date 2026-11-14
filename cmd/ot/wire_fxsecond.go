package main

import (
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/fxrefresh"
	"github.com/andrewrong/optdesk/internal/sources"
)

// Design: docs/design/source-rank.md#6 source-rank-wire.

// attachFxSecondary attaches the FRED source to an fx_refresh job (fxrefresh.AttachSecondary); a fake job is left alone.
var attachFxSecondary fxrefresh.AttachSecondaryFunc = fxrefresh.AttachSecondary

// withFxSecondary attaches src, the FRED source of the factory, to j when a FRED key is set: without a key src is
// the noKeySource stand-in and j stays as built.
func withFxSecondary(j contracts.Job, src sources.Source) contracts.Job {
	if _, none := src.(noKeySource); !none && src != nil {
		attachFxSecondary(j, src)
	}
	return j
}
