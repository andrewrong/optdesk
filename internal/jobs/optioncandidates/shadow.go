package optioncandidates

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
	"github.com/andrewrong/optdesk/internal/strategy"
)

// shadowRun is the shadow of one Run (docs/design/strategy-framework.md Decision 6): it collects the snapshot of
// each underlying, as the run builds the input of the old path, and after the old insert evaluates, ranks and
// inserts the result rows. It is off (every method does nothing) unless Strategy and Results are both set, and it
// never fails the run.
type shadowRun struct {
	st    CandidateStores
	snaps []strategy.Snapshot
}

func newShadow(st CandidateStores) *shadowRun { return &shadowRun{st: st} }

func (s *shadowRun) logger() *slog.Logger {
	if s.st.log != nil {
		return s.st.log
	}
	return slog.Default()
}

func (s *shadowRun) on() bool { return s.st.Strategy != nil && s.st.Results != nil }

// add keeps the snapshot of one underlying, built from the values that built in.
func (s *shadowRun) add(in compute.CandidateInput, ul underlyingRef, ev chainEarnings) {
	if !s.on() {
		return
	}
	ctx := map[string]string{"earnings_state": ev.state}
	if ev.next != "" {
		ctx["next_earnings"] = string(ev.next)
	}
	s.snaps = append(s.snaps, strategy.Snapshot{
		RunDay: in.RunDay, ChainDay: in.ChainDay, SessionDays: in.SessionDays, Issuer: ul.issuer,
		Underlying: in.Underlying, UnderlyingID: ul.id, Spot: in.Spot, Quotes: in.Quotes, BuyPrice: in.BuyPrice,
		SalePrice: in.SalePrice, Earnings: in.Earnings, Dividend: in.Dividend, Rate: in.Rate, RateDay: in.RateDay,
		Accounts: in.Accounts, Context: ctx,
	})
}

// write inserts the result rows once, after the old insert. old is the rows of the run as written (any kind): the
// short_put rows give the run id and the instant. A failure is logged with the run id and a code, never a value.
func (s *shadowRun) write(ctx context.Context, old []store.Candidate) {
	if !s.on() {
		return
	}
	var puts []store.Candidate
	for _, r := range old {
		if r.Kind == string(compute.KindShortPut) {
			puts = append(puts, r)
		}
	}
	if len(puts) == 0 {
		return
	}
	runID := puts[0].RunID
	defer func() {
		if recover() != nil {
			s.logger().Error("shadow failed", "component", "jobs", "job", "option_candidates", "code", "shadow_panic", "run_id", runID)
		}
	}()
	rows, err := s.rows(puts[0])
	if err != nil {
		s.logger().Error("shadow failed", "component", "jobs", "job", "option_candidates", "code", "shadow_map_failed", "run_id", runID)
		return
	}
	if _, err := s.st.Results.InsertResults(ctx, rows); err != nil {
		s.logger().Error("shadow failed", "component", "jobs", "job", "option_candidates", "code", "shadow_insert_failed", "run_id", runID)
	}
}

// rows evaluates every snapshot, ranks the evaluations of the run and converts them.
func (s *shadowRun) rows(first store.Candidate) ([]strategyresult.Row, error) {
	var evals []strategy.Evaluation
	owner := map[string]int{}
	for _, snap := range s.snaps {
		for _, e := range s.st.Strategy.Evaluate(snap) {
			owner[e.Key] = int(snap.UnderlyingID)
			evals = append(evals, e)
		}
	}
	evals = s.st.Strategy.Rank(evals)
	p := s.st.Strategy.Params()
	out := make([]strategyresult.Row, 0, len(evals))
	for _, e := range evals {
		legs, err := legsText(e.Legs)
		if err != nil {
			return nil, err
		}
		checks, err := json.Marshal(checkList(e.Checks))
		if err != nil {
			return nil, err
		}
		facts := e.Facts
		if facts == nil {
			facts = map[string]string{}
		}
		factsText, err := json.Marshal(facts) // map keys are sorted by encoding/json
		if err != nil {
			return nil, err
		}
		row := strategyresult.Row{
			SessionDay: first.SessionDay, RunID: first.RunID, Strategy: s.st.Strategy.ID(),
			RuleVersion: s.st.Strategy.Version(), ParamHash: p.Hash, Key: e.Key, UnderlyingID: owner[e.Key],
			Status: e.Status, Legs: legs, Checks: string(checks), Facts: string(factsText), ComputedAt: first.ComputedAt,
			SettingsVersion: first.SettingsVersion,
		}
		if e.Rank > 0 {
			r := e.Rank
			row.Rank = &r
		}
		if e.RankInUnderlying > 0 {
			r := e.RankInUnderlying
			row.RankInUnderlying = &r
		}
		out = append(out, row)
	}
	return out, nil
}

type legJSON struct {
	InstrumentID string `json:"instrument_id"`
	Side         string `json:"side"`
	Right        string `json:"right"`
	Strike       string `json:"strike"`
	Expiry       string `json:"expiry"`
	Multiplier   string `json:"multiplier"`
	Quantity     int    `json:"quantity"`
}

func legsText(legs []strategy.Leg) (string, error) {
	out := make([]legJSON, 0, len(legs))
	for _, l := range legs {
		out = append(out, legJSON{l.InstrumentID, l.Side, l.Right, l.Strike, string(l.Expiry), l.Multiplier, l.Quantity})
	}
	b, err := json.Marshal(out)
	return string(b), err
}

func checkList(c []strategy.Check) []gateJSON {
	out := make([]gateJSON, 0, len(c))
	for _, x := range c {
		out = append(out, gateJSON{x.Name, x.Status, x.Reason})
	}
	return out
}

// settingsResults stamps the settings version of the run on every result row it writes, as settingsRows does for
// the old rows.
type settingsResults struct {
	inner strategyresult.Writer
	r     *settingsRun
}

func (w settingsResults) InsertResults(ctx context.Context, rows []strategyresult.Row) (int, error) {
	w.r.mu.Lock()
	v := w.r.v
	w.r.mu.Unlock()
	stamped := make([]strategyresult.Row, len(rows))
	copy(stamped, rows)
	for i := range stamped {
		stamped[i].SettingsVersion = v
	}
	return w.inner.InsertResults(ctx, stamped)
}

// checkStrategyNeeds is an error when a registered strategy id has no [[need]] row; a row without a registered
// strategy (covered_call today) is not one. The error names the id, never a value of a row.
func checkStrategyNeeds(ids []string, needs []jobs.Need) error {
	have := map[string]bool{}
	for _, n := range needs {
		have[n.Strategy] = true
	}
	for _, id := range ids {
		if !have[id] {
			return fmt.Errorf("optioncandidates: strategy %q has no [[need]] row in job.toml", id)
		}
	}
	return nil
}
