// Package notify says missed work once, as a text message. The scheduler process calls
// Notifier.NotifyGaps after each gap pass; notify is the one owner of ops.job_gap.reported_at.
package notify

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/jobrunapi"
	"github.com/andrewrong/optdesk/internal/gaphistory"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/jobrun"
	"github.com/jonboulle/clockwork"
)

const (
	// TokenEnv names the line of $OT_HOME/.env that holds the bot token.
	TokenEnv = "OT_TELEGRAM_TOKEN"
	// ChatIDEnv names the line of $OT_HOME/.env that holds the chat id.
	ChatIDEnv = "OT_TELEGRAM_CHAT_ID"
	// DefaultQuietFrom is the start of the default quiet hours, "HH:MM" in Config.Zone.
	DefaultQuietFrom = "23:00"
	// DefaultQuietTo is the end of the default quiet hours; the window may cross midnight.
	DefaultQuietTo = "07:00"
	// DefaultMorningFrom is the earliest time of the morning report, "HH:MM" in Config.Zone.
	DefaultMorningFrom = "08:00"
	// DefaultMorningUntil is the time from which the morning report is no longer sent that day.
	DefaultMorningUntil = "12:00"
	// MorningRuleID is the rule id of the day mark of the morning report in alerts.sent.
	MorningRuleID = "morning_report"
)

// ErrSend is the error class of a failed delivery. Its text never holds a token, a chat id or a URL.
var ErrSend = errors.New("notify: send failed")

// Sender delivers one text message. It returns nil only when the service confirmed the delivery.
type Sender interface {
	Send(ctx context.Context, text string) error
}

// GapSource is the consumer-side port of notify over the gap rows.
type GapSource interface {
	// OpenGaps returns the open gaps (store.SchedulerLedger.Status).
	OpenGaps(ctx context.Context) ([]store.Gap, error)
	// MarkGapReported sets ReportedAt of the gap with the key of g (store.SchedulerLedger.MarkGapReported).
	MarkGapReported(ctx context.Context, g store.Gap, at time.Time) error
}

// Notifier makes one pass over the open gaps.
//
// NotifyGaps sends at most one text for the open gaps not yet reported and marks only the gaps
// printed in it, after the send was confirmed (at-least-once delivery). It returns the number of
// gaps reported; there is no Run and no loop.
type Notifier interface {
	NotifyGaps(ctx context.Context) (int, error)
}

// Config holds the quiet hours. An empty QuietFrom or QuietTo takes the default.
type Config struct {
	// QuietFrom and QuietTo are "HH:MM" in Zone.
	QuietFrom, QuietTo string
	// MorningFrom and MorningUntil are "HH:MM" in Zone; empty means DefaultMorningFrom and DefaultMorningUntil.
	MorningFrom, MorningUntil string
	// Zone is the owner's zone (profile.Zone).
	Zone *time.Location
}

// TelegramOptions are the optional settings of the Telegram sender, for tests.
type TelegramOptions struct {
	// ServerURL replaces the Telegram server; empty means the real one.
	ServerURL string
	// HTTPClient replaces the HTTP client; nil means the default.
	HTTPClient *http.Client
}

// NewNotifierFunc is the constructor of the notifier (internal/notify/core.New); it fails only on a bad Config.
type NewNotifierFunc func(GapSource, Sender, clockwork.Clock, Config) (Notifier, error)

// NewTelegramSenderFunc is the constructor of the Telegram sender (internal/notify/telegram.New).
type NewTelegramSenderFunc func(token, chatID string, o TelegramOptions) (Sender, error)

// NewDryRunSenderFunc is the constructor of the sender that writes the text to w (internal/notify/dryrun.New).
type NewDryRunSenderFunc func(w io.Writer) Sender

// AlertDeps are the inputs of the alert pass besides the sender, the clock and the Config.
type AlertDeps struct {
	// Rules are the alert rules, with PublicSubjects.
	Rules compute.AlertRules
	// Evaluate evaluates the rules (internal/compute/alertrules.Evaluate).
	Evaluate compute.EvaluateAlertsFunc
	// Read is the winner reader of the report page.
	Read compute.ReadingFunc
	// Sent holds the marks of the alerts already said (alerts.sent).
	Sent store.SentStore
}

// AlertNotifier makes one pass over the market alerts, in the quiet window of Config.
//
// NotifyAlerts sends at most one text "optdesk: N market alert(s)" of the results that are
// fired and Public and have no mark yet, and calls Sent.MarkSent only for the alerts printed,
// after the send was confirmed (at-least-once delivery). Inside the quiet window it returns 0
// and reads nothing. It returns the number of alerts reported; there is no Run and no loop.
type AlertNotifier interface {
	NotifyAlerts(ctx context.Context) (int, error)
}

// NewAlertNotifierFunc is the constructor of the alert notifier (internal/notify/core.NewAlerts);
// it fails only on a bad Config or an incomplete AlertDeps.
type NewAlertNotifierFunc func(AlertDeps, Sender, clockwork.Clock, Config) (AlertNotifier, error)

// MorningDeps are the inputs of the morning report besides the sender, the clock and the Config.
type MorningDeps struct {
	// History reads the gaps cleared or seen since the previous morning (gaphistory.Reader).
	History gaphistory.Reader
	// Expect reads the open gaps and the expectations of the last 7 calendar days.
	Expect store.GapReader
	// Sent holds the day mark of the report under MorningRuleID (alerts.sent).
	Sent store.SentStore
}

// MorningNotifier makes one pass of the morning report.
//
// NotifyMorning sends at most one text per calendar day in Config.Zone, inside the window
// MorningFrom to MorningUntil, even when nothing happened, and marks (MorningRuleID, day) only
// after the send was confirmed (at-least-once delivery). It returns true when a message was
// sent; outside the window or after the mark it returns false and reads nothing.
type MorningNotifier interface {
	NotifyMorning(ctx context.Context) (bool, error)
}

// NewMorningNotifierFunc is the constructor of the morning notifier (internal/notify/core.NewMorning);
// it fails only on a bad Config or an incomplete MorningDeps.
type NewMorningNotifierFunc func(MorningDeps, Sender, clockwork.Clock, Config) (MorningNotifier, error)

// SlowMarker is the one method of store/jobrun.JobRunLedger that the slow notifier needs.
type SlowMarker interface {
	// MarkSlowNotified records that the slow notice of run id was sent at at.
	MarkSlowNotified(ctx context.Context, id string, at time.Time) error
}

// SlowDeps are the inputs of the slow-run pass besides the sender, the clock and the Config.
type SlowDeps struct {
	// Runs reads the open runs and the usual durations (store/jobrun.JobRunReader).
	Runs jobrun.JobRunReader
	// Mark records the notice of a run (the ledger's MarkSlowNotified).
	Mark SlowMarker
	// Figures says which runs are slow (compute/jobrunapi.FiguresFunc); the notifier copies Slow and applies no rule of its own.
	Figures jobrunapi.FiguresFunc
}

// SlowNotifier makes one pass over the open runs, in the quiet window of Config.
//
// NotifySlowRuns sends at most one text per slow run that has no slow_notified_at yet, and calls
// MarkSlowNotified only for the run printed, after the send was confirmed (at-least-once
// delivery). Inside the quiet window it returns 0 and reads nothing. It returns the number of runs
// reported; there is no Run and no loop. It never stops, cancels or refuses a run: a slow run is a notice only.
type SlowNotifier interface {
	NotifySlowRuns(ctx context.Context) (int, error)
}

// NewSlowNotifierFunc is the constructor of the slow notifier (internal/notify/core.NewSlow);
// it fails only on a bad Config or an incomplete SlowDeps.
type NewSlowNotifierFunc func(SlowDeps, Sender, clockwork.Clock, Config) (SlowNotifier, error)

// GatewayFact is what the scheduler tick knows of one gateway. Farm and Since are those of the
// first farm that is down, and empty when Down is false.
type GatewayFact struct {
	Gateway, Farm string
	Since         time.Time
	Down          bool
}

// GatewayFarm is one data farm of a gateway as FactOf reads it.
type GatewayFarm struct {
	Name  string
	Up    bool
	Since time.Time
}

// StateFarmBroken is the state word of broker.Health that makes a GatewayFact Down.
const StateFarmBroken = "farm_broken"

// FactOfFunc is the mapping from a health reading to a GatewayFact: Down is true when answered is
// true and state is StateFarmBroken; Farm and Since are those of the first farm not up.
type FactOfFunc func(alias string, answered bool, state string, farms []GatewayFarm) GatewayFact

// FactOf is the declared mapping, owned by notify (implemented in fact.go).
var FactOf FactOfFunc = factOf

// GatewayNotifier makes one pass over the gateway facts, one fact per gateway, on every tick.
//
// NotifyGateways sends one text for a Down fact whose incident (gateway, farm, Since) has no mark,
// and one text for a gateway with a marked incident when its fact is no longer Down. Marks are
// written through store.SentStore only after the send was confirmed (at-least-once delivery): a
// failed send marks nothing and the next pass sends again. It has no quiet window and no loop,
// and returns the number of texts sent. The incident key is gateway_<alias>_<farm>_<since_unix>
// and the recovery key is that key plus "_up"; aliases hold no underscore. Marks are looked up
// 30 days back, so an incident older than that gets no recovery text.
type GatewayNotifier interface {
	NotifyGateways(ctx context.Context, facts []GatewayFact) (int, error)
}

// NewGatewayNotifierFunc is the constructor of the gateway notifier (internal/notify/core.NewGateways);
// it fails only on a nil argument.
type NewGatewayNotifierFunc func(Sender, store.SentStore, clockwork.Clock) (GatewayNotifier, error)
