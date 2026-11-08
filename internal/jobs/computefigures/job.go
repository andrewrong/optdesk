package computefigures

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
)

//go:embed job.toml
var manifestBytes []byte

const (
	datasetReadings = "readings.reading"
	slotName        = "session_close"
	methodRatio     = "ratio"
	methodPercent   = "percentile"
	methodMAGap     = "ma_gap"
)

type marketSpec struct {
	ID     string   `toml:"id"`
	Method string   `toml:"method"`
	Series []string `toml:"series"`
}

type manifest struct {
	ID         string        `toml:"id"`
	RateSeries string        `toml:"rate_series"`
	Iv30       string        `toml:"iv30"`
	Ivp        string        `toml:"ivp"`
	Ivr        string        `toml:"ivr"`
	Market     []marketSpec  `toml:"market"`
	Schedule   scheduleTable `toml:"schedule"`
}

type job struct {
	m      manifest
	sched  schedule
	stores jobs.ComputeStores
	figs   compute.Figures
	rules  compute.Rules
	units  compute.Units
	ops    compute.SeriesOps
	clk    clock.Clock
	rule   map[string]compute.Rule
}

type typedError struct {
	kind contracts.ErrorKind
	err  error
}

func (e *typedError) Error() string             { return e.err.Error() }
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

// New is a jobs.NewComputeFiguresFunc. It reads the embedded job.toml and finds every rule it names.
func New(stores jobs.ComputeStores, figs compute.Figures, rules compute.Rules, units compute.Units,
	ops compute.SeriesOps, clk clock.Clock) (contracts.Job, error) {
	if stores.Catalog == nil || stores.OptionEod == nil || stores.Terms == nil || stores.Names == nil ||
		stores.SessionDays == nil || stores.Series == nil || stores.Readings == nil {
		return nil, errors.New("computefigures: every store is required")
	}
	if figs == nil || rules == nil || units == nil || ops == nil || clk == nil {
		return nil, errors.New("computefigures: figures, rules, units, series ops and clock are required")
	}
	var m manifest
	if err := toml.Unmarshal(manifestBytes, &m); err != nil {
		return nil, fmt.Errorf("computefigures: manifest: %w", err)
	}
	if m.ID == "" || m.RateSeries == "" || len(m.Market) == 0 {
		return nil, errors.New("computefigures: manifest needs an id, a rate series and market figures")
	}
	sched, err := scheduleOf(m.Schedule)
	if err != nil {
		return nil, err
	}
	j := &job{m: m, sched: sched, stores: stores, figs: figs, rules: rules, units: units, ops: ops, clk: clk,
		rule: map[string]compute.Rule{}}
	if err := j.findRules(); err != nil {
		return nil, err
	}
	return j, nil
}

// findRules looks up every rule the manifest names and checks the series of each market figure.
func (j *job) findRules() error {
	ids := []string{j.m.Iv30, j.m.Ivp, j.m.Ivr}
	for _, ms := range j.m.Market {
		if len(ms.Series) == 0 || (ms.Method == methodRatio && len(ms.Series) != 2) {
			return fmt.Errorf("computefigures: manifest: bad series of %s", ms.ID)
		}
		if ms.Method != methodRatio && ms.Method != methodPercent && ms.Method != methodMAGap {
			return fmt.Errorf("computefigures: manifest: unknown method of %s", ms.ID)
		}
		ids = append(ids, ms.ID)
	}
	for _, id := range ids {
		r, ok := j.rules.Rule(id)
		if !ok {
			return fmt.Errorf("computefigures: no rule %q", id)
		}
		j.rule[id] = r
	}
	return nil
}

// ID is the stable name of the job.
func (j *job) ID() string { return j.m.ID }

func keyOf(name, kind, subj string) contracts.ExpectedKey {
	return contracts.ExpectedKey{DatasetID: datasetReadings, SubjectKey: name + "|" + kind + "|" + subj}
}

// parseKey splits "name|subject_kind|subject".
func parseKey(k string) (name, kind, subj string, ok bool) {
	p := strings.SplitN(k, "|", 3)
	if len(p) != 3 || p[0] == "" || p[1] == "" || p[2] == "" {
		return "", "", "", false
	}
	return p[0], p[1], p[2], true
}

func dayValue(d string) contracts.Day { return contracts.Day(d) }

// Plan lists one key per rule and subject, sorted, and the hash of the plan.
func (j *job) Plan(ctx context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	subs, dropped, err := listSubjects(ctx, j.stores, day)
	if err != nil {
		return nil, "", err
	}
	if dropped > 0 {
		slog.Info("computefigures: underlyings dropped for having only later rows", "day", string(day), "count", dropped)
	}
	var keys []contracts.ExpectedKey
	for _, s := range subs {
		for _, id := range []string{j.m.Iv30, j.m.Ivp, j.m.Ivr} {
			keys = append(keys, keyOf(id, kindUnderlying, s.name))
		}
	}
	for _, ms := range j.m.Market {
		keys = append(keys, keyOf(ms.ID, kindMarket, ms.Series[0]))
	}
	sort.Slice(keys, func(a, b int) bool { return keys[a].SubjectKey < keys[b].SubjectKey })
	h := sha256.New()
	fmt.Fprintf(h, "%s\n%s\n", j.m.ID, day)
	for _, k := range keys {
		name, _, _, _ := parseKey(k.SubjectKey)
		fmt.Fprintf(h, "%s|%s\n", k.SubjectKey, j.rule[name].Hash)
	}
	return keys, hex.EncodeToString(h.Sum(nil)), nil
}

// target is one planned key that Run resolved.
type target struct {
	rule compute.Rule
	subj subject
	spec marketSpec
}

// Run runs the steps in order: IV30, the market figures, then IVP and IVR on the stored IV30.
// A key that fails does not stop the others.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	r := &runState{j: j, ctx: ctx, day: day}
	var iv30, market, ivs []target
	var under map[string]subject
	for _, k := range keys {
		name, kind, subj, ok := parseKey(k.SubjectKey)
		if !ok || k.DatasetID != datasetReadings {
			r.fail(contracts.Permanent, fmt.Errorf("computefigures: bad key %q", k.SubjectKey))
			continue
		}
		rule, known := j.rule[name]
		switch {
		case known && kind == kindMarket:
			spec, found := j.marketSpec(name, subj)
			if !found {
				r.fail(contracts.Permanent, fmt.Errorf("computefigures: unknown key %q", k.SubjectKey))
				continue
			}
			market = append(market, target{rule: rule, subj: subject{kind: kind, name: subj}, spec: spec})
		case known && kind == kindUnderlying:
			if under == nil {
				subs, dropped, err := listSubjects(ctx, j.stores, day)
				if dropped > 0 {
					slog.Info("computefigures: underlyings dropped for having only later rows", "day", string(day), "count", dropped)
				}
				if err != nil {
					r.fail(contracts.Retryable, err)
					under = map[string]subject{}
				} else {
					under = map[string]subject{}
					for _, s := range subs {
						under[s.name] = s
					}
				}
			}
			s, found := under[subj]
			if !found {
				r.fail(contracts.Retryable, fmt.Errorf("computefigures: %s has no option rows on %s", subj, day))
				continue
			}
			if name == j.m.Iv30 {
				iv30 = append(iv30, target{rule: rule, subj: s})
			} else {
				ivs = append(ivs, target{rule: rule, subj: s})
			}
		default:
			r.fail(contracts.Permanent, fmt.Errorf("computefigures: unknown key %q", k.SubjectKey))
		}
	}
	r.returned = len(keys)
	r.runIv30(iv30)
	r.runMarket(market)
	r.runIvStats(ivs)
	if r.noTerms > 0 {
		slog.Info("computefigures: option rows dropped for want of current terms", "day", string(day), "count", r.noTerms)
	}
	res := contracts.JobResult{Returned: r.returned, Usable: r.usable}
	if r.firstErr != nil {
		res.Err = r.firstErr
	}
	return res
}

func (j *job) marketSpec(id, series string) (marketSpec, bool) {
	for _, ms := range j.m.Market {
		if ms.ID == id && ms.Series[0] == series {
			return ms, true
		}
	}
	return marketSpec{}, false
}

func dateOf(d contracts.Day) types.SerializableDate {
	t, _ := parseDay(d)
	return types.SerializableDate{Time: t}
}
