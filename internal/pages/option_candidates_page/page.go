// Package option_candidates_page builds the view model of the option candidates page.
package option_candidates_page

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/compute/optionfigapi"
	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// ATMFunc returns the implied volatility and the strike of the at-the-money put of one expiry of one
// underlying on a chain day, as decimal text; empty texts with a nil error mean no usable quote.
type ATMFunc func(ctx context.Context, underlyingID int, day, expiry contracts.Day, spot string) (iv, strike string, err error)

// Config is what the option_candidates_page builder needs besides the series reader.
type Config struct {
	RunID, BuildID, Scope string
	Zone                  *time.Location              // generated_at is written in it; nil is an error of Build
	Candidates            store.CandidateReader       // nil gives the gap no_reader
	Figures               optionfigapi.OptionFigures  // the ITM and excess figures of the short-put rows; nil gives the gap no_reader
	Names                 store.InstrumentNameReader  // the ticker source of the display name; nil gives the stored issuer text
	Evidence              optionevidenceapi.Evaluator // nil gives the gap no_reader
	ATMFor                ATMFunc                     // nil gives the gap no_reader
	Readings              store.ReadingViewReader     // nil gives the gap no_reader
	Tiers                 optiontierapi.Tierer        // judges the short-put rows into tiers; nil gives the gap no_reader
	Account               store.AccountReader         // the cash and positions of each alias; nil gives the gap no_reader
	Aliases               []string                    // the account aliases whose cash is read; nil gives the gap no_reader
	Owner                 ownerset.Reader             // the stored rule parameters; nil gives the default 0.3
	RuleParams            settingsapi.RuleParams      // resolves the stored parameters; nil gives the default 0.3
	// Valuation reads the current valuation of each company; nil gives no valuation block on the strips.
	Valuation valuation.Reader
	// ValueLines computes B_hi from a review; nil gives no valuation block on the strips.
	ValueLines valuelinesapi.Calculator
}

// NewPage is the option_candidates_page page for c.
func NewPage(c Config) pages.Page {
	return pages.Page{
		Meta: pages.Meta{Name: "option_candidates_page", Schema: "page.option_candidates_page.v1"},
		Build: func(ctx context.Context, r store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
			return Build(ctx, c, r, day, at)
		},
	}
}
