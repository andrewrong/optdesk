package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optioncandidates"
	"github.com/andrewrong/optdesk/internal/pages/candidate_text"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

func init() {
	register("candidates", candidatesCmd)
	// option_candidates is held when its line is missing; candidatesOff is the one source of that reason, so it
	// gives no startOff.
	registerJob(jobFamily{
		id: jobOptionCandidates, order: 110,
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			j, err := a.candidateJob(ctx, jc.cfg, jc.f, jc.extra)
			if j == nil || err != nil {
				return nil, "", err
			}
			return j, jc.cfg.candidatesOff(), nil
		},
	})
}

const (
	jobOptionCandidates = "option_candidates"

	// reasonNoCandidateDSN is the line logged when option_candidates is left out; it names the key, never a value.
	reasonNoCandidateDSN = store.DSNEnvCandidate + " is not set"

	// jpNoRoute is printed after the symbol of each JP name of the stored list: a JP name has no option route here.
	jpNoRoute = "no option route: see option-candidates-jp"
)

// candidateRW is the candidate store as the job writes it: the writer and the close of the connections.
type candidateRW interface {
	store.CandidateWriter
	io.Closer
}

// openCandidateRWFunc opens the candidate store with the line OT_PG_DSN_CANDIDATE (internal/store/candidate.Open).
type openCandidateRWFunc func(ctx context.Context, dsn string) (candidateRW, error)

// noCandidateWriter stands in for the candidate store when OT_PG_DSN_CANDIDATE is empty: the job is left out, so
// it is never called.
type noCandidateWriter struct{}

func (noCandidateWriter) InsertCandidates(context.Context, []store.Candidate) (int, error) {
	return 0, errors.New(reasonNoCandidateDSN)
}

// candidatesOff is the reason option_candidates is left out, or "" when it runs.
func (c runOnceConfig) candidatesOff() string {
	if c.candidateDSN == "" {
		return reasonNoCandidateDSN
	}
	return ""
}

// optionCandidates builds option_candidates over the stores of the factory.
func (f jobFactory) optionCandidates() (contracts.Job, error) {
	st := optioncandidates.CandidateStores{
		OptionEod: f.compute.OptionEod, Catalog: f.compute.Catalog, Terms: f.compute.Terms, Names: f.compute.Names,
		Series: f.compute.Series, Earnings: f.earnRd, Accounts: f.accounts, Aliases: f.candAliases, Candidates: f.candidates,
		SessionDays: f.compute.SessionDays,
		Strategy:    f.shadow.strategy, Results: f.shadow.results,
	}
	return f.listed(groupOptions, func(entries []jobs.WatchEntry) (contracts.Job, error) {
		j, err := f.d.newOptionCandidates(st, f.d.newCandidateRule(), entries, f.clk)
		if err != nil {
			return nil, fmt.Errorf("build option_candidates job failed: %w", err)
		}
		return j, nil
	})
}

// candidateJob opens the candidate store and the account reader, sets them on f and builds option_candidates.
// It is nil only for a deps without newOptionCandidates. With an empty OT_PG_DSN_CANDIDATE the job is built over
// the stand-in writer and left out by the caller. What it opens goes to extra.
func (a *app) candidateJob(ctx context.Context, cfg runOnceConfig, f *jobFactory, extra *closerSet) (contracts.Job, error) {
	if f.d.newOptionCandidates == nil {
		return nil, nil
	}
	if cfg.eventsDSN != "" {
		f.earnRd = f.events
	}
	f.candidates = noCandidateWriter{}
	if cfg.candidatesOff() == "" {
		w, err := f.d.openCandidateRW(ctx, cfg.candidateDSN)
		if err != nil {
			return nil, fmt.Errorf("open candidate store with %s failed", store.DSNEnvCandidate)
		}
		extra.addCloser(w)
		f.candidates = w
		sh, err := f.shadowOf(ctx, cfg.candidateDSN, extra)
		if err != nil {
			return nil, err
		}
		f.shadow = sh
		if cfg.accountDSN != "" && f.d.openAccount != nil {
			acct, err := f.d.openAccount(ctx, cfg.accountDSN)
			if err != nil {
				return nil, fmt.Errorf("open account store with %s failed", store.DSNEnvAccount)
			}
			extra.addCloser(acct)
			f.accounts = acct
			a.candidateAliases(f)
		}
	}
	return f.optionCandidates()
}

// candidateAliases sets f.candAliases from the book of accounts.toml (f.book, set by the wiring); a factory
// without a book leaves the list nil: the job then reads no account.
func (a *app) candidateAliases(f *jobFactory) {
	f.candAliases = withoutStatement(f.book, f.bookAliases())
}

// candidatesCmd is `ot candidates [--day DAY] [--all]`: it opens the candidate store with OT_PG_DSN_PAGES (role ot_ro_pages, the view rd.option_candidate_r1),
// prints candidate_text.Format of the short_put rows of the day, then one line per active JP name of the stored list.
// The text goes to standard output only.
func candidatesCmd(a *app, args []string) error {
	fs := flag.NewFlagSet("candidates", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dayFlag := fs.String("day", "", "day YYYY-MM-DD (default: today in "+marketZone+")")
	all := fs.Bool("all", false, "also list the blocked and unknown rows with their reasons")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if a.profile.Name == "demo" {
		return errors.New("candidates needs a real profile; the demo profile opens no store")
	}
	env, err := a.envFile()
	if err != nil {
		return err
	}
	dsn := env[pagesDSNName]
	if dsn == "" {
		return errors.New(pagesDSNName + " is not set")
	}
	ctx := context.Background()
	day, err := dayOrDefault(a.deps.newClock(), *dayFlag)
	if err != nil {
		return err
	}
	s, err := a.deps.openCandidate(ctx, dsn)
	if err != nil {
		return fmt.Errorf("open candidate store with %s failed", pagesDSNName)
	}
	defer func() { _ = s.Close() }()
	rows, err := s.Candidates(ctx, day, string(compute.KindShortPut))
	if err != nil {
		return errors.New("read candidates failed")
	}
	_, _ = fmt.Fprintln(a.stdout, candidate_text.Format(rows, *all))
	jp, err := jpListed(ctx, a, dsn)
	if err != nil {
		return err
	}
	for _, sym := range jp {
		_, _ = fmt.Fprintf(a.stdout, "%s: %s\n", sym, jpNoRoute)
	}
	return nil
}

// jpListed is the symbol of each active JP name of the stored list (read with the pages line), in key order. A deps
// without the opener gives none.
func jpListed(ctx context.Context, a *app, dsn string) ([]string, error) {
	if a.deps.openUnderlying == nil {
		return nil, nil
	}
	st, err := a.deps.openUnderlying(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open underlying list with %s failed", pagesDSNName)
	}
	defer func() { _ = st.Close() }()
	rows, err := st.Current(ctx)
	if err != nil {
		return nil, errors.New("read underlying list failed")
	}
	var out []string
	for _, r := range rows {
		if r.Market == "JP" && r.Status == underlying.StatusActive {
			out = append(out, r.Symbol)
		}
	}
	return out, nil
}
