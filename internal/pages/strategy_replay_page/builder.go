package strategy_replay_page

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

const (
	postPath     = "strategy-replay-post"
	defaultKeep  = "0.50"
	defaultDTE   = "21"
	defaultMinLo = "20"
)

// DefaultForm is the form of the first view: the lots choice short_puts, every alias, today, and the trial values
// of the early rule and of the minimum.
func DefaultForm() FormInput {
	return FormInput{Lots: LotsShortPuts, EarlyKeep: defaultKeep, EarlyDTE: defaultDTE, MinLots: defaultMinLo}
}

type optionJSON struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type fieldJSON struct {
	Name    string       `json:"name"`
	Label   string       `json:"label"`
	Hint    string       `json:"hint"`
	Kind    string       `json:"kind"`
	Value   string       `json:"value"`
	Options []optionJSON `json:"options"`
}

type formJSON struct {
	Fields      []fieldJSON `json:"fields"`
	SubmitLabel string      `json:"submit_label"`
}

type noticeJSON struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

type variantJSON struct {
	Name       string `json:"name"`
	ParamsHash string `json:"params_hash"`
}

type countsJSON struct {
	Lots          int `json:"lots"`
	LotsWithQuote int `json:"lots_with_quote"`
	StillOpen     int `json:"still_open"`
	LotDays       int `json:"lot_days"`
	QuoteDays     int `json:"quote_days"`
}

type excludedJSON struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
	Lots   int    `json:"lots"`
}

type lineJSON struct {
	Set             string  `json:"set"`
	Variant         string  `json:"variant"`
	Currency        string  `json:"currency"`
	Lots            int     `json:"lots"`
	Cycles          int     `json:"cycles"`
	ClosedEarly     int     `json:"closed_early"`
	Held            int     `json:"held"`
	Assigned        int     `json:"assigned"`
	Rolled          int     `json:"rolled"`
	InTheMoney      int     `json:"in_the_money"`
	KeptShare       *string `json:"kept_share"`
	MeanGross       *string `json:"mean_gross"`
	WorstGross      *string `json:"worst_gross"`
	HoldKeptShare   *string `json:"hold_kept_share"`
	ActualKeptShare *string `json:"actual_kept_share"`
	UnderMinimum    bool    `json:"under_minimum"`
	MinimumText     *string `json:"minimum_text"`
}

type reportJSON struct {
	AsOf          string         `json:"as_of"`
	LotsLabel     string         `json:"lots_label"`
	FirstQuoteDay *string        `json:"first_quote_day"`
	LastQuoteDay  *string        `json:"last_quote_day"`
	Variants      []variantJSON  `json:"variants"`
	Counts        countsJSON     `json:"counts"`
	Excluded      []excludedJSON `json:"excluded"`
	Lines         []lineJSON     `json:"lines"`
	Notes         []string       `json:"notes"`
	EmptyText     *string        `json:"empty_text"`
}

type pageJSON struct {
	Schema        string       `json:"schema"`
	SchemaVersion int          `json:"schema_version"`
	GeneratedAt   string       `json:"generated_at"`
	Title         string       `json:"title"`
	View          string       `json:"view"`
	PostPath      string       `json:"post_path"`
	Intro         string       `json:"intro"`
	Form          *formJSON    `json:"form"`
	RestartHint   string       `json:"restart_hint"`
	Notices       []noticeJSON `json:"notices"`
	Report        *reportJSON  `json:"report"`
}

var noticeTexts = map[string]string{
	NoticeFormOff:      noticeFormOff,
	NoticeTimedOut:     noticeTimedOut,
	NoticeBusy:         noticeBusy,
	NoticeReadFailed:   noticeReadFailed,
	NoticeBadLots:      noticeBadLots,
	NoticeBadAsOf:      noticeBadAsOf,
	NoticeBadEarlyKeep: noticeBadEarlyKeep,
	NoticeBadEarlyDTE:  noticeBadEarlyDTE,
	NoticeBadMinLots:   noticeBadMinLots,
}

var lotsLabels = map[string]string{
	LotsShortPuts: textLotsShortPut,
	LotsShortLots: textLotsShortLot,
}

// Build returns the JSON of page.strategy_replay_page.v1. With no Form the page is the route-off page. An
// unknown view, notice key or lots value, or a report view with no Report, is an error. Nothing is computed.
func Build(ctx context.Context, in Input) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if in.View != ViewForm && in.View != ViewReport {
		return nil, fmt.Errorf("strategy_replay_page: unknown view %q", in.View)
	}
	p := &pageJSON{
		Schema: "strategy_replay_page", SchemaVersion: 1, GeneratedAt: in.GeneratedAt, View: in.View,
		PostPath: postPath, Title: textTitle, Intro: textIntro, RestartHint: textRestartHint,
		Notices: []noticeJSON{},
	}
	keys := in.Notices
	if in.Form == nil {
		p.View = ViewForm
		keys = append([]string{NoticeFormOff}, dropKey(in.Notices, NoticeFormOff)...)
	}
	for _, k := range keys {
		t, ok := noticeTexts[k]
		if !ok {
			return nil, fmt.Errorf("strategy_replay_page: unknown notice key %q", k)
		}
		p.Notices = append(p.Notices, noticeJSON{Key: k, Text: t})
	}
	if in.Form != nil {
		p.Form = formDoc(*in.Form)
		if in.View == ViewReport {
			if in.Report == nil {
				return nil, fmt.Errorf("strategy_replay_page: view report without a report")
			}
			r, err := reportDoc(*in.Report)
			if err != nil {
				return nil, err
			}
			p.Report = r
		}
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("strategy_replay_page: %w", err)
	}
	return out, nil
}

func dropKey(keys []string, drop string) []string {
	var out []string
	for _, k := range keys {
		if k != drop {
			out = append(out, k)
		}
	}
	return out
}

func formDoc(f FormInput) *formJSON {
	text := func(name, label, hint, value string) fieldJSON {
		return fieldJSON{Name: name, Label: label, Hint: hint, Kind: "text", Value: value, Options: []optionJSON{}}
	}
	return &formJSON{
		Fields: []fieldJSON{
			{Name: "lots", Label: labelLots, Hint: hintLots, Kind: "choice", Value: f.Lots, Options: []optionJSON{
				{Value: LotsShortPuts, Label: textLotsShortPut},
				{Value: LotsShortLots, Label: textLotsShortLot},
			}},
			text("alias", labelAlias, hintAlias, f.Alias),
			text("as_of", labelAsOf, hintAsOf, f.AsOf),
			text("early_keep", labelEarlyKeep, hintEarlyKeep, f.EarlyKeep),
			text("early_dte", labelEarlyDTE, hintEarlyDTE, f.EarlyDTE),
			text("min_lots", labelMinLots, hintMinLots, f.MinLots),
		},
		SubmitLabel: textSubmit,
	}
}

// opt turns an empty text into null.
func opt(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func reportDoc(r ReportInput) (*reportJSON, error) {
	label, ok := lotsLabels[r.Lots]
	if !ok {
		return nil, fmt.Errorf("strategy_replay_page: unknown lots value %q", r.Lots)
	}
	d := &reportJSON{
		AsOf: r.AsOf, LotsLabel: label, FirstQuoteDay: opt(r.FirstQuoteDay), LastQuoteDay: opt(r.LastQuoteDay),
		Variants: []variantJSON{}, Excluded: []excludedJSON{}, Lines: []lineJSON{},
		Counts: countsJSON{Lots: r.LotCount, LotsWithQuote: r.LotsWithQuote, StillOpen: r.StillOpen,
			LotDays: r.LotDays, QuoteDays: r.QuoteDays},
		Notes: []string{noteAssignment, noteGap, noteSnapshot, noteRolled},
	}
	for _, v := range r.Variants {
		d.Variants = append(d.Variants, variantJSON{Name: v.Name, ParamsHash: v.ParamsHash})
	}
	for _, e := range r.Excluded {
		d.Excluded = append(d.Excluded, excludedJSON{Reason: e.Reason, Text: e.Text, Lots: e.Lots})
	}
	for _, l := range r.Lines {
		lj := lineJSON{
			Set: l.Set, Variant: l.Variant, Currency: l.Currency, Lots: l.Lots, Cycles: l.Cycles,
			ClosedEarly: l.ClosedEarly, Held: l.Held, Assigned: l.Assigned, Rolled: l.Rolled, InTheMoney: l.InMoney,
			KeptShare: opt(l.KeptShare), MeanGross: opt(l.MeanGross), WorstGross: opt(l.WorstGross),
			HoldKeptShare: opt(l.HoldKeptShare), ActualKeptShare: opt(l.ActualKeptShare),
			UnderMinimum: l.UnderMinimum,
		}
		if l.UnderMinimum {
			s := minimumHead + strconv.Itoa(r.MinLots) + minimumTail
			lj.MinimumText = &s
		}
		d.Lines = append(d.Lines, lj)
	}
	if len(r.Lines) == 0 {
		s := emptyHead + strconv.Itoa(r.LotCount) + emptyLots + strconv.Itoa(r.QuoteDays) + emptyDays + span(r)
		d.EmptyText = &s
	}
	return d, nil
}

func span(r ReportInput) string {
	if r.FirstQuoteDay == "" && r.LastQuoteDay == "" {
		return emptyNone
	}
	first, last := r.FirstQuoteDay, r.LastQuoteDay
	if first == "" {
		first = emptyNone
	}
	if last == "" {
		last = emptyNone
	}
	return first + emptyTo + last
}
