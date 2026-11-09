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
	"github.com/andrewrong/optdesk/internal/gaphistory"
	"github.com/andrewrong/optdesk/internal/store"
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
