package valuation_import_page

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
)

// Build returns the JSON of page.valuation_import_page.v1. The view empty has no file, no rows and the post
// path; the views preview and result copy the plan, and the result adds the outcomes of the confirm. Nothing is
// computed or rounded: a figure is a field of the plan or of the input.
func Build(ctx context.Context, in Input) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch in.View {
	case ViewEmpty, ViewPreview, ViewResult:
	default:
		return nil, fmt.Errorf("valuation_import_page: unknown view %q", in.View)
	}
	p := &pageJSON{
		Schema: "valuation_import_page", SchemaVersion: 1, View: in.View, PostPath: postPath,
		MaxFileBytes: valueimportapi.MaxFileBytes,
		Rows:         []rowJSON{}, NotInFile: []string{}, Unvalued: []string{}, Removed: []string{},
		Result: []resultJSON{}, Problems: []problemJSON{}, Gaps: []gapJSON{},
		Error: optional(in.Error),
	}
	for _, t := range in.Problems {
		p.Problems = append(p.Problems, problemJSON{Text: cut(t)})
	}
	for _, t := range in.Gaps {
		p.Gaps = append(p.Gaps, gapJSON{Kind: "gap", Reason: reasonStoreNA, Text: cut(t)})
	}
	if in.View != ViewEmpty {
		fillPlan(p, in)
	}
	if in.View == ViewResult {
		for _, o := range in.Applied {
			p.Result = append(p.Result, resultJSON{Key: cut(o.Key), Outcome: o.Outcome, Text: cut(o.Text)})
		}
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("valuation_import_page: %w", err)
	}
	return out, nil
}

func fillPlan(p *pageJSON, in Input) {
	pl := in.Plan
	p.File = &fileJSON{Name: fileName(in.FileName), SHA256: pl.FileSHA256, AsOf: string(pl.AsOf)}
	p.R = &rJSON{
		Action: pl.R.Action, FileUS: pl.R.US, FileJP: pl.R.JP, FileHKCN: pl.R.HKCN,
		Note: cut(pl.R.Note), LatestYield: in.LatestYield, Problem: reason(pl.R.Problem),
	}
	p.Margin = &marginJSON{Action: pl.Margin.Action, Rows: []marginRowJSON{}}
	for _, r := range pl.Margin.Rows {
		p.Margin.Rows = append(p.Margin.Rows, marginRowJSON{PFrom: r.PFrom, M: r.M, W: r.W, H: r.H, S: r.S})
	}
	for _, r := range pl.Rows {
		p.Rows = append(p.Rows, rowDoc(r))
		count(&p.Counts, r.Action, r.NeedsTick)
	}
	p.NotInFile = append(p.NotInFile, pl.NotInFile...)
	p.Unvalued = append(p.Unvalued, pl.Unvalued...)
	p.Removed = append(p.Removed, pl.RemovedNames...)
	if pl.FlaggedCount > 0 {
		s := strconv.Itoa(pl.FlaggedCount) + textFlagged
		p.FlaggedNote = &s
	}
}

func rowDoc(r valueimportapi.RowPlan) rowJSON {
	why := reason(r.Reason)
	if why == "" {
		switch {
		case r.Action == valueimportapi.ActionOlder:
			why = TextOlder
		case r.Unmatched:
			why = TextUnmatched
		}
	}
	return rowJSON{
		Key: r.Key, Ticker: r.Ticker, Name: r.Name, Currency: r.Currency, RMarket: r.RMarket,
		NewProfile: r.NewProfile, Unmatched: r.Unmatched, ProposedKey: r.ProposedKey,
		Action: r.Action, Reason: why, ReviewType: reviewType(r.ReviewType),
		V: r.V, P: r.P, VersionDate: string(r.VersionDate),
		FileBuyTop: r.FileBuyTop, SiteBuyTop: r.SiteBuyTop, NeedsTick: r.NeedsTick,
	}
}

func count(c *countsJSON, action string, needsTick bool) {
	switch action {
	case valueimportapi.ActionInsert:
		c.Insert++
	case valueimportapi.ActionSame:
		c.Same++
	case valueimportapi.ActionOlder:
		c.Older++
	case valueimportapi.ActionRefused:
		c.Refused++
	}
	if needsTick {
		c.NeedsTick++
	}
}

func reviewType(t string) string {
	switch t {
	case "first":
		return TextFirst
	case "event":
		return TextEvent
	}
	return t
}

// reason gives the sentence with its hint for a reason of the importer; an unknown text is shown as it is.
func reason(s string) string {
	if t, ok := importerReasons[s]; ok {
		return t
	}
	return cut(s)
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	c := cut(s)
	return &c
}

// cut keeps a text within the 600 characters of the schema.
func cut(s string) string {
	r := []rune(s)
	if len(r) > 600 {
		return string(r[:600])
	}
	return s
}

// fileName drops any path and keeps at most 64 characters.
func fileName(s string) string {
	if i := strings.LastIndexAny(s, `/\`); i >= 0 {
		s = s[i+1:]
	}
	r := []rune(s)
	if len(r) > maxFileName {
		r = r[:maxFileName]
	}
	if len(r) == 0 {
		return defaultName
	}
	return string(r)
}
