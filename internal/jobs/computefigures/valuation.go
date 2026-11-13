package computefigures

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	valuationBand      = "valuation.buy_band"
	valuationQuestion  = "valuation.question"
	valuationDiagnosis = "valuation.diagnosis"

	valuationSecSource    = "sec"
	valuationEdinetSource = "edinet"
	valuationEdinetPrefix = "edinet:"
	valuationFactsTable   = "rd.fundamental_fact_r1"
	valuationNoStandard   = "no_standard"
)

// valuationSourceOf is the fact source of an issuer: edinet: issuers are stored under edinet, the others (cik:)
// under sec.
func valuationSourceOf(issuer string) string {
	if strings.HasPrefix(issuer, valuationEdinetPrefix) {
		return valuationEdinetSource
	}
	return valuationSecSource
}

var valuationNames = []string{valuationBand, valuationQuestion, valuationDiagnosis}

// NewWithValuation is a NewComputeFiguresValuationFunc. It builds the job with NewWithEase and no ease; with
// any function of deps nil that job is returned as it is. With all set the job also plans and stores, for each
// entry with an issuer, the three readings of valuation.buy_band, valuation.question and valuation.diagnosis.
func NewWithValuation(stores ComputeStores, figs compute.Figures, rules compute.Rules, units compute.Units,
	ops compute.SeriesOps, deps ValuationDeps, clk clock.Clock) (contracts.Job, error) {
	c, err := NewWithEase(stores, figs, rules, units, ops, nil, clk)
	if err != nil {
		return nil, err
	}
	if deps.StandardOf == nil || deps.BuildFilings == nil || deps.Band == nil || deps.Questions == nil ||
		deps.Diagnosis == nil {
		return c, nil
	}
	if deps.Facts == nil {
		return nil, fmt.Errorf("computefigures: valuation needs a fact reader")
	}
	base, ok := c.(*job)
	if !ok {
		return nil, fmt.Errorf("computefigures: unexpected job type %T", c)
	}
	vr := map[string]compute.Rule{}
	for _, n := range valuationNames {
		r, found := rules.Rule(n)
		if !found {
			return nil, fmt.Errorf("computefigures: no rule %q", n)
		}
		vr[n] = r
	}
	return &valuationJob{job: base, deps: deps, vrule: vr}, nil
}

// valuationJob is the compute_figures job with the valuation step. Postcondition and the schedule are the
// embedded job's.
type valuationJob struct {
	*job
	deps  ValuationDeps
	vrule map[string]compute.Rule
}

// valuationEntries maps the contract symbol of each entry with an issuer to that issuer, the first entry winning.
func (v *valuationJob) valuationEntries() map[string]string {
	out := map[string]string{}
	for _, e := range v.deps.Entries {
		if e.Issuer == "" || e.Contract.Symbol == "" {
			continue
		}
		if _, dup := out[e.Contract.Symbol]; !dup {
			out[e.Contract.Symbol] = e.Issuer
		}
	}
	return out
}

func valuationIsKey(k contracts.ExpectedKey) bool {
	return strings.HasPrefix(k.SubjectKey, "valuation.")
}

// Plan is the plan of the embedded job plus the three valuation keys of each entry with an issuer.
func (v *valuationJob) Plan(ctx context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys, hash, err := v.job.Plan(ctx, day)
	if err != nil {
		return nil, "", err
	}
	entries := v.valuationEntries()
	if len(entries) == 0 {
		return keys, hash, nil
	}
	for sym := range entries {
		for _, n := range valuationNames {
			keys = append(keys, keyOf(n, kindUnderlying, sym))
		}
	}
	sort.Slice(keys, func(a, b int) bool { return keys[a].SubjectKey < keys[b].SubjectKey })
	h := sha256.New()
	fmt.Fprintf(h, "%s\n", hash)
	for _, k := range keys {
		if valuationIsKey(k) {
			name, _, _, _ := parseKey(k.SubjectKey)
			fmt.Fprintf(h, "%s|%s\n", k.SubjectKey, v.vrule[name].Hash)
		}
	}
	return keys, hex.EncodeToString(h.Sum(nil)), nil
}

// Run hands the keys that are not valuation keys to the embedded job and stores the valuation readings.
func (v *valuationJob) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	var rest, mine []contracts.ExpectedKey
	for _, k := range keys {
		if valuationIsKey(k) {
			mine = append(mine, k)
		} else {
			rest = append(rest, k)
		}
	}
	if err := needUpserter(v.stores); err != nil {
		return contracts.JobResult{Err: err}
	}
	var res contracts.JobResult
	if len(rest) > 0 {
		res = v.job.Run(ctx, day, rest)
	}
	if len(mine) == 0 {
		return res
	}
	vs := &valuationState{v: v, ctx: ctx, day: day, cache: map[string]*valuationCompany{}}
	vs.run(mine)
	res.Returned += len(mine)
	res.Usable += vs.usable
	if vs.err != nil && (res.Err == nil || (res.Err.Kind() != contracts.Retryable && vs.err.Kind() == contracts.Retryable)) {
		res.Err = vs.err
	}
	return res
}

type valuationState struct {
	v      *valuationJob
	ctx    context.Context
	day    contracts.Day
	usable int
	err    contracts.TypedError
	cache  map[string]*valuationCompany
}

func (s *valuationState) fail(kind contracts.ErrorKind, err error) {
	if s.err == nil || (s.err.Kind() != contracts.Retryable && kind == contracts.Retryable) {
		s.err = &typedError{kind: kind, err: err}
	}
}

// valuationCompany is what the facts of one company give, built once per run and shared by its three keys.
type valuationCompany struct {
	rowsHash string
	source   string
	std      valuationapi.Standard
	res      valuationapi.FilingsResult
	refused  []string // set when no filings were built (no standard)
}

func (s *valuationState) run(keys []contracts.ExpectedKey) {
	entries := s.v.valuationEntries()
	for _, k := range keys {
		name, kind, sym, ok := parseKey(k.SubjectKey)
		issuer, known := entries[sym]
		if !ok || k.DatasetID != datasetReadings || kind != kindUnderlying || !known || s.v.vrule[name].ID == "" {
			s.fail(contracts.Permanent, fmt.Errorf("computefigures: bad key %q", k.SubjectKey))
			continue
		}
		if s.ctx.Err() != nil {
			s.fail(contracts.Retryable, s.ctx.Err())
			return
		}
		co, found := s.cache[issuer]
		if !found {
			var err error
			if co, err = s.company(issuer); err != nil {
				s.fail(contracts.Retryable, err)
				continue
			}
			s.cache[issuer] = co
		}
		s.store(name, sym, co, s.reading(name, co))
	}
}

// company reads the facts of the issuer and builds its filings.
func (s *valuationState) company(issuer string) (*valuationCompany, error) {
	source := valuationSourceOf(issuer)
	facts, err := s.v.deps.Facts.FactsFiledBy(s.ctx, source, issuer, s.day)
	if err != nil {
		return nil, fmt.Errorf("computefigures: facts of %s: %w", issuer, err)
	}
	lines := make([]string, 0, len(facts))
	for _, f := range facts {
		lines = append(lines, strings.Join([]string{f.DocID, f.Taxonomy, f.Concept, f.Unit, f.Value,
			f.Filed.Format(dayLayout), f.PeriodEnd.Format(dayLayout), periodStart(f)}, "|"))
	}
	co := &valuationCompany{rowsHash: hashLines(lines), source: source}
	co.std = s.v.deps.StandardOf(facts)
	if co.std == "" {
		co.refused = []string{valuationNoStandard}
		return co, nil
	}
	co.res = s.v.deps.BuildFilings(facts, s.day, co.std)
	return co, nil
}

func periodStart(f contracts.FundamentalFact) string {
	if f.PeriodStart == nil {
		return ""
	}
	return f.PeriodStart.Format(dayLayout)
}

// valuationOut is one reading before it is a record.
type valuationOut struct {
	value   *string
	unit    string
	ccy     string
	params  []param
	missing bool
}

func valuationTag(name, v string) param { return param{name: name, tag: &v} }

func valuationMissing(unit string, reasons []string) valuationOut {
	o := valuationOut{unit: unit, missing: true}
	for _, r := range reasons {
		o.params = append(o.params, valuationTag("reason", r))
	}
	return o
}

// reading is the reading of one key: the figure of the function when the filings are ok (for the diagnosis
// also when the company is refused as financial or cyclical), else missing_input with the reasons.
func (s *valuationState) reading(name string, co *valuationCompany) valuationOut {
	unit := s.v.vrule[name].Unit
	reasons := co.refused
	if co.refused == nil {
		reasons = co.res.Reasons
		switch {
		case co.res.State == valuationapi.StateOK:
		case name == valuationDiagnosis && co.res.State == valuationapi.StateUnvalued:
		default:
			return valuationMissing(unit, reasons)
		}
	} else {
		return valuationMissing(unit, reasons)
	}
	f := co.res.Filings
	switch name {
	case valuationBand:
		b := s.v.deps.Band(valuationBand, f, s.day)
		if b.State != valuationapi.StateOK || b.Value == "" {
			return valuationMissing(unit, b.Reasons)
		}
		o := valuationOut{value: &b.Value, unit: unit, ccy: b.Currency}
		o.params = append(o.params, valuationTag("low", b.Low), valuationTag("ref_high", b.RefHigh),
			valuationTag("base_fy_end", string(b.BaseFYEnd)))
		for _, fl := range b.Flags {
			o.params = append(o.params, valuationTag("flag", fl))
		}
		return o
	case valuationQuestion:
		qs := s.v.deps.Questions(f, s.day).Questions
		items := make([]valuationItem, 0, len(qs))
		for _, q := range qs {
			items = append(items, valuationItem{q.Key, q.State, q.Value, q.Reason})
		}
		return valuationItems(unit, "", items)
	default:
		d := s.v.deps.Diagnosis(f, s.day)
		items := make([]valuationItem, 0, len(d.Figures))
		for _, g := range d.Figures {
			items = append(items, valuationItem{g.Key, g.State, g.Value, g.Reason})
		}
		return valuationItems(unit, d.Group, items)
	}
}

type valuationItem struct{ key, state, value, reason string }

// valuationItems is a reading of many figures: the value is the count of figures that are ok, each figure is a
// parameter named by its key (its value) or `<key>.reason`; none ok is missing_input.
func valuationItems(unit string, group string, items []valuationItem) valuationOut {
	o := valuationOut{unit: unit}
	if group != "" {
		o.params = append(o.params, valuationTag("group", group))
	}
	n := 0
	for _, it := range items {
		if it.state == valuationapi.StateOK {
			n++
			o.params = append(o.params, valuationTag(it.key, it.value))
		} else {
			o.params = append(o.params, valuationTag(it.key+".reason", it.reason))
		}
	}
	if n == 0 {
		o.missing = true
		return o
	}
	text := strconv.Itoa(n)
	o.value = &text
	return o
}

// store writes the reading as the next rev; an equal state, reason and rows hash of the newest rev is skipped.
func (s *valuationState) store(name, sym string, co *valuationCompany, o valuationOut) {
	rule := s.v.vrule[name]
	rec := contracts.ReadingRecord{
		Name: rule.ID, Version: rule.Version, SubjectKind: kindUnderlying, Subject: sym,
		Day: dateOf(s.day), Currency: o.ccy, Origin: "compute_figures", Cutoff: slotName,
		JobRunID:   uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(s.day)+"|"+s.v.m.ID+"|"+slotName)).String(),
		RecordedAt: s.v.clk.Now().UTC(), RowsSha256: co.rowsHash, Rev: 1,
	}
	setString(&rec.Unit, o.unit)
	if o.missing {
		code := reasonCode(compute.NoInput)
		rec.State, rec.ReasonCode = "missing_input", &code
	} else {
		rec.State, rec.Value = "ok", o.value
	}
	e := grow(&rec.Inputs)
	e.InputTable, e.Keys, e.CodeVersion, e.ConventionVersion = valuationFactsTable, []string{co.source}, rule.Hash, rule.Convention
	for _, p := range o.params {
		e := grow(&rec.Parameters)
		e.Param, e.ValueNum, e.ValueTag = p.name, p.num, p.tag
	}
	wantText := valuationParamsText(o.params)
	prev, err := s.v.stores.Readings.LatestReadings(s.ctx, rec.Name, rec.SubjectKind, rec.Subject, s.day, s.day)
	if err != nil {
		s.fail(contracts.Retryable, fmt.Errorf("computefigures: readings: %w", err))
		return
	}
	if n := len(prev); n > 0 {
		p := prev[n-1]
		old := make([]param, 0, len(p.Parameters))
		for _, q := range p.Parameters {
			old = append(old, param{name: q.Param, tag: q.ValueTag})
		}
		prevText := valuationParamsText(old)
		if p.State == rec.State && p.RowsSha256 == rec.RowsSha256 && sameReason(p.ReasonCode, rec.ReasonCode) &&
			prevText == wantText {
			s.usable++
			return
		}
		if p.State == "ok" && rec.State == "missing_input" {
			slog.Warn("computefigures: the earlier ok is kept against a missing reading", "name", rec.Name,
				"subject", rec.Subject, "day", string(s.day))
			s.usable++
			return
		}
		rec.Rev = p.Rev + 1
	}
	if _, err := upsertReadings(s.ctx, s.v.stores, []contracts.ReadingRecord{rec}); err != nil {
		s.fail(contracts.Retryable, fmt.Errorf("computefigures: insert readings: %w", err))
		return
	}
	s.usable++
}

// valuationParamsText is the tag parameters in one text, to compare two readings.
func valuationParamsText(ps []param) string {
	var b strings.Builder
	for _, q := range ps {
		tag := ""
		if q.tag != nil {
			tag = *q.tag
		}
		b.WriteString(q.name + "=" + tag + ";")
	}
	return b.String()
}
