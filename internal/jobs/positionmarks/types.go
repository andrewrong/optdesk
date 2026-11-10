package positionmarks

import (
	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

// NewPositionMarksFunc is the constructor of the position_marks job (internal/jobs/positionmarks.New). market is
// the one market-data gateway, whatever account holds the position; aliases are the accounts whose positions
// are marked, the main one first; acct is where the job reads the newest capture of the day per alias.
type NewPositionMarksFunc func(market broker.Broker, aliases []string, acct store.AccountReader, w mark.Writer, clk clock.Clock) (contracts.Job, error)
