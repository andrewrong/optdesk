package option_pnl_page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	timeLayout  = "2006-01-02T15:04:05-07:00"
	maxAccounts = 4
)

var (
	idPattern   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	unsafeChars = regexp.MustCompile(`[^A-Za-z0-9:_-]`)
)

type bookView struct {
	ID       string  `json:"id"`
	Currency string  `json:"currency"`
	Counts   string  `json:"counts"`
	LeftOut  *string `json:"left_out"`
	Headline line    `json:"headline"`
	Lines    []line  `json:"lines"`
}

type lotView struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	Lines    []line   `json:"lines"`
	DaysLine line     `json:"days_line"`
	IVLine   *line    `json:"iv_line"`
	EndLine  *line    `json:"end_line"`
	Recon    []line   `json:"recon"`
	Badges   []string `json:"badges"`
	Gap      *gap     `json:"gap"`
}

type accountView struct {
	Alias      string     `json:"alias"`
	Books      []bookView `json:"books"`
	Lots       []lotView  `json:"lots"`
	SampleNote string     `json:"sample_note"`
	Gap        *gap       `json:"gap"`
}

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

type viewModel struct {
	Schema        string        `json:"schema"`
	SchemaVersion int           `json:"schema_version"`
	RunID         string        `json:"run_id"`
	BuildID       string        `json:"build_id"`
	Scope         string        `json:"scope"`
	GeneratedAt   string        `json:"generated_at"`
	DataTimes     []sourceTime  `json:"data_times"`
	RunStatus     string        `json:"run_status"`
	Stale         *struct{}     `json:"stale"`
	Title         string        `json:"title"`
	Notes         []string      `json:"notes"`
	Gaps          []gap         `json:"gaps"`
	AsOf          string        `json:"as_of"`
	Accounts      []accountView `json:"accounts"`
}

// Build returns the page.option_pnl_page.v1 JSON for day. For each alias of the ledger (at most four)
// it gathers the rows, attributes each short put lot through c.Attributor, and sums through c.Booker:
// the page adds nothing. r is the source of the DGS3MO rates. A lot the page cannot split is a card
// with its reason; a reader error, or an error of the attributor other than ErrBadInput, is returned.
// Design: docs/design/options-pnl-page.md#Decisions.
func Build(ctx context.Context, c Config, r store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
	switch {
	case c.Zone == nil:
		return nil, errors.New("option_pnl_page: nil zone")
	case c.Attributor == nil:
		return nil, errors.New("option_pnl_page: nil attributor")
	case c.Booker == nil:
		return nil, errors.New("option_pnl_page: nil booker")
	case c.Replay == nil:
		return nil, errors.New("option_pnl_page: nil replay")
	}
	for name, v := range map[string]string{"run_id": c.RunID, "build_id": c.BuildID, "scope": c.Scope} {
		if !idPattern.MatchString(v) {
			return nil, fmt.Errorf("option_pnl_page: bad %s %q", name, v)
		}
	}
	if _, err := time.Parse(dayLayout, string(day)); err != nil {
		return nil, fmt.Errorf("option_pnl_page: bad day %q: %w", day, err)
	}
	vm := &viewModel{Schema: "option_pnl_page", SchemaVersion: 1, RunID: c.RunID, BuildID: c.BuildID, Scope: c.Scope,
		GeneratedAt: at.In(c.Zone).Format(timeLayout), DataTimes: []sourceTime{}, RunStatus: "ok", Title: pageTitle,
		Notes: []string{modelNote, quoteDayNote}, Gaps: []gap{}, AsOf: string(day), Accounts: []accountView{}}
	if c.Ledger == nil {
		g := codeGap(codeNoReader)
		vm.Gaps = append(vm.Gaps, g)
		vm.Accounts = append(vm.Accounts, gapAccount(defaultName, g))
	} else if err := buildAccounts(ctx, c, r, day, vm); err != nil {
		return nil, err
	}
	if len(vm.Gaps) > 0 {
		vm.RunStatus = "partial"
	}
	for _, a := range vm.Accounts {
		if a.Gap != nil {
			vm.RunStatus = "partial"
		}
	}
	return json.Marshal(vm)
}

func gapAccount(alias string, g gap) accountView {
	return accountView{Alias: alias, Books: []bookView{}, Lots: []lotView{}, SampleNote: g.Text, Gap: &g}
}

func buildAccounts(ctx context.Context, c Config, r store.SeriesReader, day contracts.Day, vm *viewModel) error {
	aliases, err := c.Ledger.StoredAliases(ctx)
	if err != nil {
		return fmt.Errorf("option_pnl_page: read aliases: %w", err)
	}
	if len(aliases) == 0 {
		vm.Accounts = append(vm.Accounts, gapAccount(defaultName, codeGap(codeNoFlexRows)))
		return nil
	}
	if len(aliases) > maxAccounts {
		aliases = aliases[:maxAccounts]
	}
	seen := map[string]bool{}
	for i, alias := range aliases {
		g, err := gather(ctx, c, alias, day, r)
		if err != nil {
			return err
		}
		if g.gap != "" {
			gp := codeGap(g.gap)
			if g.gap != codeNoFlexRows && !seen[g.gap] {
				seen[g.gap] = true
				vm.Gaps = append(vm.Gaps, gp)
			}
			vm.Accounts = append(vm.Accounts, gapAccount(alias, gp))
			continue
		}
		a, err := buildAccount(c, alias, i, g)
		if err != nil {
			return err
		}
		if g.calls > 0 {
			vm.Notes = append(vm.Notes, alias+": "+strconv.Itoa(g.calls)+callsTail)
		}
		vm.Accounts = append(vm.Accounts, a)
	}
	return nil
}

// buildAccount attributes the lots of one alias and writes its books and cards.
func buildAccount(c Config, alias string, idx int, g gathered) (accountView, error) {
	a := accountView{Alias: alias, Books: []bookView{}, Lots: []lotView{}}
	outs := make([]outcome, 0, len(g.lots))
	var results []optionattrapi.Result
	ended := 0
	for _, l := range g.lots {
		o, err := attribute(c, l, g.rates)
		if err != nil {
			return a, err
		}
		outs = append(outs, o)
		if !o.isGap() {
			results = append(results, o.res)
		}
		if l.pos.Status != statusOpen {
			ended++
		}
	}
	var books []optionattrapi.Book
	if len(results) > 0 {
		books = c.Booker.Combine(results)
	}
	for n, o := range outs {
		card, err := lotCard(c, fmt.Sprintf("a%dl%d", idx, n), alias, o, roll(o, outs))
		if err != nil {
			return a, err
		}
		a.Lots = append(a.Lots, card)
	}
	a.Books = bookCards(alias, outs, books)
	a.SampleNote = strconv.Itoa(ended) + " ended lots: " + sampleFew
	if ended >= 20 {
		a.SampleNote = strconv.Itoa(ended) + " ended lots: " + sampleEnuf
	}
	return a, nil
}

func safeID(s string, max int) string {
	s = unsafeChars.ReplaceAllString(s, "_")
	if len(s) > max {
		s = s[:max]
	}
	return s
}
