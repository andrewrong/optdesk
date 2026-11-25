package optioncandidates

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optioncandapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/candidatecur"
)

// The option_candidates_jp job: yen put candidates from the open pull of a Tokyo session day
// (docs/design/candidates-jp.md Decisions 1, 3, 5, 10, 11 and 12).

//go:embed job_jp.toml
var manifestJPBytes []byte

// JPStores is the stores of the JP job: those of the US job (Open is required here, Candidates, Catalog, Earnings
// and Series are not read) and the writer that stores the currency of each row.
type JPStores struct {
	CandidateStores
	Rows candidatecur.Writer
}

// jpJob is the JP job: the US job's manifest reading, source and stores, with its own plan, run and writer.
type jpJob struct {
	*job
	rule  optioncandapi.CurrencyRule
	rows  candidatecur.Writer
	after []string
}

// NewJP builds the option_candidates_jp job from the embedded job_jp.toml.
func NewJP(st JPStores, rule optioncandapi.CurrencyRule, entries []jobs.WatchEntry, clk clock.Clock) (contracts.Job, error) {
	if st.OptionEod == nil || st.Terms == nil || st.Names == nil || st.Open == nil || st.SessionDays == nil ||
		st.Rows == nil || rule == nil || clk == nil {
		return nil, errors.New("optioncandidates: JP chain, terms, names, open, session day and candidate stores, the rule and the clock are required")
	}
	m, sl, params, err := parseManifest(manifestJPBytes)
	if err != nil {
		return nil, err
	}
	if err := checkSource(m.Source); err != nil {
		return nil, err
	}
	if err := checkCurrency(m.Source.Currency); err != nil {
		return nil, err
	}
	var extra struct {
		Schedule struct {
			After []string `toml:"after"`
		} `toml:"schedule"`
	}
	if err := toml.Unmarshal(manifestJPBytes, &extra); err != nil || len(extra.Schedule.After) == 0 {
		return nil, errors.New("optioncandidates: manifest: schedule.after is invalid")
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.Issuer == "" {
			continue
		}
		if seen[e.Issuer] {
			return nil, errors.New("optioncandidates: an entry needs a unique issuer")
		}
		seen[e.Issuer] = true
	}
	base := &job{m: m, slot: sl, params: params, st: st.CandidateStores, entries: entries, clk: clk}
	return &jpJob{job: base, rule: rule, rows: st.Rows, after: extra.Schedule.After}, nil
}

// After is the JP open pull of the same day.
func (j *jpJob) After() []string { return append([]string(nil), j.after...) }

// jpEntries are the JP stock entries with an issuer, sorted by symbol.
func (j *jpJob) jpEntries() []jobs.WatchEntry {
	var out []jobs.WatchEntry
	for _, e := range j.entries {
		if e.Issuer != "" && e.Market == "JP" && e.Contract.SecType == "STK" {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Contract.Symbol < out[b].Contract.Symbol })
	return out
}

// Plan returns one key per JP entry with an issuer and the hash of the plan. It reads no list of another job.
func (j *jpJob) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	es := j.jpEntries()
	keys := make([]contracts.ExpectedKey, 0, len(es))
	ids := make([]string, 0, len(es))
	for _, e := range es {
		keys = append(keys, contracts.ExpectedKey{DatasetID: datasetID, SubjectKey: e.Issuer})
		ids = append(ids, e.Issuer)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s\n%s", j.m.ID, j.m.Version, day, strings.Join(ids, "\n"))))
	return keys, hex.EncodeToString(sum[:]), nil
}

// locateJP finds the instrument id of each JP entry among the underlyings of the JPY open rows; an entry with no
// such row is returned in missing.
func (j *jpJob) locateJP(ctx context.Context, src *sourceRun, es []jobs.WatchEntry) (found []underlyingRef, missing []jobs.WatchEntry, err error) {
	ids := make([]int64, 0, len(src.open))
	for id := range src.open {
		ids = append(ids, int64(id))
	}
	slices.Sort(ids)
	bySymbol := map[string]int64{}
	for start := 0; start < len(ids); start += termsChunk {
		end := min(start+termsChunk, len(ids))
		names, err := j.st.Names.InstrumentNames(ctx, ids[start:end])
		if err != nil {
			return nil, nil, err
		}
		for _, n := range names {
			if n.Type == "stock" {
				bySymbol[strings.ToUpper(n.Name)] = n.InstrumentID
			}
		}
	}
	for _, e := range es {
		id, ok := bySymbol[strings.ToUpper(e.Contract.Symbol)]
		if !ok || len(src.open[int(id)]) == 0 {
			missing = append(missing, e)
			continue
		}
		found = append(found, underlyingRef{issuer: e.Issuer, symbol: e.Contract.Symbol, id: id, chainDay: src.day})
	}
	return found, missing, nil
}

// quoteCounts are the counts of one name's open puts that no stored row shows.
type quoteCounts struct{ outside, unusable int }

func (s *sourceRun) countPuts(quotes []compute.OptionQuote) quoteCounts {
	var c quoteCounts
	day, ok := parseDay(s.day)
	for _, q := range quotes {
		if q.Right != "P" {
			continue
		}
		exp, err := time.Parse(time.DateOnly, string(q.Expiry))
		if err != nil || !ok {
			c.outside++
			continue
		}
		dte := int(exp.Sub(day).Hours() / 24)
		switch {
		case dte < s.j.params.MinDTE || dte > s.j.params.MaxDTE:
			c.outside++
		case !q.QuoteOK:
			c.unusable++
		}
	}
	return c
}

// gateCounts counts, per gate and reason, the rows the rule blocked. A key is the
// gate name, a colon and the reason (the gate name alone when the reason is empty);
// the keys are sorted and printed as key=count joined by commas.
func gateCounts(cs []compute.Candidate) string {
	n := map[string]int{}
	for _, c := range cs {
		for _, g := range c.Gates {
			if g.Status == compute.GateBlock {
				k := g.Gate
				if g.Reason != "" {
					k += ":" + g.Reason
				}
				n[k]++
			}
		}
	}
	keys := make([]string, 0, len(n))
	for k := range n {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+strconv.Itoa(n[k]))
	}
	return strings.Join(parts, ",")
}

func multipliers(quotes []compute.OptionQuote) string {
	set := map[string]bool{}
	for _, q := range quotes {
		set[q.Multiplier] = true
	}
	out := make([]string, 0, len(set))
	for m := range set {
		out = append(out, m)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// Run reads the JPY open rows of the day, evaluates every JP name in yen and writes the rows under a new run id
// with currency JPY. A name with no open row or no usable open quote gets no row and is named in Skipped(); a failed
// key of the pull leaves its name with no row and fails nothing.
//
//nolint:funlen // one straight pass over the names
func (j *jpJob) Run(ctx context.Context, day contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	es := j.jpEntries()
	src := newSourceRun(j.job, day)
	var skipped []string
	defer func() {
		skipped = append(skipped, src.finish()...)
		j.mu.Lock()
		j.skipped = skipped
		j.mu.Unlock()
	}()
	days, err := sessionDays(ctx, j.st.SessionDays, j.slot.calendar, day)
	if err != nil {
		return fail("read_session_days_failed", err)
	}
	src.begin(ctx, days)
	if src.openErr != nil {
		return fail("read_open_failed", src.openErr)
	}
	ulist, missing, err := j.locateJP(ctx, src, es)
	if err != nil {
		return fail("read_names_failed", err)
	}
	for _, e := range missing {
		skipped = append(skipped, "JP entry "+e.Issuer+": no option chain, no row")
	}
	src.nNone = len(missing)
	now := j.clk.Now().UTC()
	currency := j.m.Source.Currency
	var rows []store.Candidate
	for _, ul := range ulist {
		entry := entryOf(es, ul.issuer)
		ch, err := src.chain(ctx, ul, entry, false)
		if err != nil {
			return fail("read_chain_failed", err)
		}
		counts := src.countPuts(ch.quotes)
		if ch.unusable {
			skipped = append(skipped, "JP entry "+ul.issuer+": no usable open quote, no row")
			j.logger().Info("option_candidates_jp name", "component", "jobs", "job", j.m.ID, "symbol", ul.symbol,
				"rows", 0, "outside_window", counts.outside, "unusable_quote", counts.unusable)
			continue
		}
		info, ev, err := j.earnings(ctx, entry, ul.chainDay)
		if err != nil {
			return fail("read_earnings_failed", err)
		}
		accts, err := j.accounts(ctx, ul.symbol, days)
		if err != nil {
			return fail("read_account_failed", err)
		}
		inputs := []contracts.Day{ul.chainDay}
		if info != nil {
			inputs = append(inputs, info.RowDay)
		}
		for _, a := range accts {
			inputs = append(inputs, a.CaptureDay)
		}
		in := optioncandapi.CurrencyInput{
			RunDay: day, ChainDay: ul.chainDay, SessionDays: trimSessionDays(days, inputs...), Underlying: ul.symbol,
			Spot: ch.spot, Quotes: ch.quotes, BuyPrice: entry.BuyPrice, SalePrice: entry.SalePrice, Earnings: info,
			Accounts: accts, Kinds: []compute.CandidateKind{compute.KindShortPut}, Currency: currency}
		cands := j.rule.EvaluateIn(in, j.params)
		spreadMin, spreadMed := spreadFigures(ch.quotes, day, j.params.MinDTE, j.params.MaxDTE)
		j.logger().Info("option_candidates_jp name", "component", "jobs", "job", j.m.ID, "symbol", ul.symbol,
			"rows", len(cands), "outside_window", counts.outside, "unusable_quote", counts.unusable,
			"gates_blocked", gateCounts(cands), "spread_min", spreadMin, "spread_median", spreadMed,
			"multiplier", multipliers(ch.quotes))
		for _, c := range cands {
			ch.addMarks(&c)
			addBuySourceMark(&c, entry)
			row, err := toStoreRow(c, day, ul, ev, now)
			if err != nil {
				return fail("map_candidate_failed", err)
			}
			rows = append(rows, row)
		}
	}
	if err := rankRun(rows); err != nil {
		return fail("rank_candidates_failed", err)
	}
	setRunID(day, rows)
	out := make([]candidatecur.Row, len(rows))
	for i, r := range rows {
		out[i] = candidatecur.Row{Candidate: r, Currency: currency}
	}
	if _, err := j.rows.InsertCandidatesIn(ctx, out); err != nil {
		return fail("write_candidates_failed", err)
	}
	res := contracts.JobResult{Returned: len(rows)}
	for _, r := range rows {
		if r.Status == "ok" {
			res.Usable++
		}
	}
	return res
}
