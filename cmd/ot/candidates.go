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
	"github.com/andrewrong/optdesk/internal/pages/candidate_text"
	"github.com/andrewrong/optdesk/internal/store"
)

func init() { register("candidates", candidatesCmd) }

const (
	jobOptionCandidates = "option_candidates"

	// reasonNoCandidateDSN is the line logged when option_candidates is left out; it names the key, never a value.
	reasonNoCandidateDSN = store.DSNEnvCandidate + " is not set"

	// jpNoRoute is printed after the symbol of each JP entry of the watch list: a JP entry has no option route here.
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
	st := jobs.CandidateStores{
		OptionEod: f.compute.OptionEod, Catalog: f.compute.Catalog, Terms: f.compute.Terms, Names: f.compute.Names,
		Series: f.compute.Series, Earnings: f.earnRd, Accounts: f.accounts, Candidates: f.candidates,
		SessionDays: f.compute.SessionDays,
	}
	j, err := f.d.newOptionCandidates(st, f.d.newCandidateRule(), f.watch, f.clk)
	if err != nil {
		return nil, errors.New("build option_candidates job failed")
	}
	return j, nil
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
		if cfg.accountDSN != "" && f.d.openAccount != nil {
			acct, err := f.d.openAccount(ctx, cfg.accountDSN)
			if err != nil {
				return nil, fmt.Errorf("open account store with %s failed", store.DSNEnvAccount)
			}
			extra.addCloser(acct)
			f.accounts = acct
		}
	}
	return f.optionCandidates()
}

// candidatesCmd is `ot candidates [--day DAY] [--all]`: it opens the candidate store with OT_PG_DSN_PAGES (role ot_ro_pages, the view rd.option_candidate_r1),
// prints candidate_text.Format of the short_put rows of the day, then one line per JP entry of the watch list.
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
	watch, err := loadWatch(a)
	if err != nil {
		return err
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
	for _, e := range watch {
		if e.Market == "JP" {
			_, _ = fmt.Fprintf(a.stdout, "%s: %s\n", e.Contract.Symbol, jpNoRoute)
		}
	}
	return nil
}
