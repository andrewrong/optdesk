// Package settings_page builds the page.settings_page.v2 view model: the owner's rule parameters, the option
// exclusions, the preview of a change and the history. It reads the owner settings through ownerset.Reader and
// writes nothing; it compares nothing, formats no number and signs nothing, the token arrives made.
package settings_page

import (
	"context"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

// Config carries the identity of the run, the readers and what the caller made for this answer.
// Zone is the owner's zone, given by the caller; times are written in it.
type Config struct {
	RunID, BuildID, Scope string
	Zone                  *time.Location
	// Params is the registry that gives the rows of the parameter table.
	Params settingsapi.RuleParams
	// Owner reads the settings; nil gives the page with the gap no_reader.
	Owner   ownerset.Reader
	Catalog store.OptionEodCatalog
	Names   store.InstrumentNameReader
	// Form is the token of the form; nil is read-only (form is null).
	Form *settingsapi.FormToken
	// Confirm is the old-to-new table and the token of its form, built by the caller; nil: confirm is null.
	Confirm *Confirm
	// HistoryN is the number of history rows read; at most 20 are listed, 0 or less lists none.
	HistoryN int
	// Notice is the result line after a post; nil: notice is null.
	Notice *Notice
}

// Confirm is the preview of a change: the rows to confirm and the token of the form that posts confirm=1.
type Confirm struct {
	Rows []ConfirmRow
	Form settingsapi.FormToken
}

// ConfirmRow is one change of the preview, all text.
type ConfirmRow struct{ Label, Old, New string }

// Notice is the result line after a post; Kind is saved, nothing_changed or error.
type Notice struct{ Kind, Text string }

// NewPage is the settings_page page for c. It keeps a copy of c, with its own Confirm; its Build ignores the series
// reader it is given and reads the readers of c.
func NewPage(c Config) pages.Page {
	if c.Confirm != nil {
		cc := *c.Confirm
		cc.Rows = slices.Clone(cc.Rows)
		c.Confirm = &cc
	}
	return pages.Page{
		Meta: pages.Meta{Name: "settings_page", Schema: "page.settings_page.v2"},
		Build: func(ctx context.Context, r store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return Build(ctx, c, r, day, at)
		},
	}
}
