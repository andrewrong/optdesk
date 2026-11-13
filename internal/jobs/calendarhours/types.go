package calendarhours

import (
	"log/slog"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/hoursnotice"
)

// NewCalendarHoursFunc is the constructor of the calendar_hours_us and calendar_hours_jp jobs
// (internal/jobs/calendarhours.New); the market is XNYS or XTKS.
type NewCalendarHoursFunc func(market string, b broker.Broker, src clock.SessionSource, hours clock.IBKRHours,
	st hoursnotice.Store, clk clock.Clock, log *slog.Logger) (contracts.Job, error)
