// Package coverage declares the read-only view of the runner's ledger that the data health page reads: one
// Cell per (session day, dataset), the session days of a calendar and the newest observation of each series
// (docs/design/data-health.md, Interface, entry 1). It holds declarations only: the implementation is
// internal/store/ops. It is a package of its own because internal/store/api.go is at its ceiling.
package coverage

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// WindowSessions is the number of session days the page shows and the repair covers; the only place it is written.
const WindowSessions = 30

// LookbackDays is how many calendar days back the builder asks Sessions, enough for WindowSessions sessions.
const LookbackDays = 60

// State is the state of the keys of a cell, as the ledger records them.
type State string

const (
	// StateOK: present, and the source gave a value.
	StateOK State = "ok"
	// StateNoValue: marked no_value, and the newest run of the key finished clean.
	StateNoValue State = "no_value"
	// StateExpired: marked no_value, and the newest run of the key is not a clean finish.
	StateExpired State = "expired"
	// StateMissing: expected by the plan of the day, not present.
	StateMissing State = "missing"
)

// Cell is one (day, dataset) of the ledger, counted by state. Keys == OK + NoValue + Expired + Missing.
type Cell struct {
	Day         contracts.Day
	Dataset     string
	Keys        int
	OK          int
	NoValue     int
	Expired     int
	Missing     int
	MissingKeys []string // up to 20 missing subject keys, ascending
}

// Session is one row of rd.session_day_r1.
type Session struct {
	Calendar  string
	Day       contracts.Day
	IsSession bool
}

// Newest is the newest stored observation of one series, from rd.series_point_r1.
type Newest struct {
	SeriesID string
	Day      contracts.Day
}

// Reader reads the coverage as the page role (ot_ro_pages). Every slice is non-nil.
type Reader interface {
	// Cells returns the cells of the days from..to, ascending by day then dataset.
	Cells(ctx context.Context, from, to contracts.Day) ([]Cell, error)
	// Sessions returns the days of one calendar from..to.
	Sessions(ctx context.Context, calendar string, from, to contracts.Day) ([]Session, error)
	// NewestObservations returns the newest observation of every series.
	NewestObservations(ctx context.Context) ([]Newest, error)
	Close() error
}

// OpenFunc opens a Reader on the database named by dsn.
type OpenFunc func(ctx context.Context, dsn string) (Reader, error)
