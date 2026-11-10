package quotesnapshot

import (
	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// NewQuoteSnapshotFunc is the constructor of the quote_snapshot job (internal/jobs/quotesnapshot.New).
type NewQuoteSnapshotFunc func(broker.Broker, store.InstrumentResolver, store.QuoteSnapshotWriter, clock.Clock) (contracts.Job, error)
