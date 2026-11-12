// Package underlyings builds the page.underlyings.v1 view model from the stored rows of the underlying store. It
// reads no store and writes nothing: the caller passes the rows, the held symbols, the history, the form token, the
// confirm block, the notice, the clock value and the owner zone. Every visible string is English and written by text.go.
// Design: docs/design/underlyings.md#4 underlyings-builder.
package underlyings

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

const (
	timeLayout   = "2006-01-02 15:04"
	historyMax   = 20
	postPath     = "/owner-underlyings"
	settleLimit  = 36 * time.Hour
	dayLayout    = "2006-01-02"
	gapKind      = "gap"
	gapNoReader  = "no_reader"
	gapNoData    = "no_data"
	editableBase = "status name issuer options fundamentals buy_price sale_price report_date_override note"
)

// Form is the token of the edit form, made by the caller; nil in Input is read-only.
type Form struct{ Expires, MAC string }

// Change is one old-to-new line of the confirm block.
type Change struct{ Label, Old, New string }

// Stop is one job group of one name in the confirm block: Stopped when the change removes the group from the name,
// Held when a held name keeps the group.
type Stop struct {
	Group, Symbol string
	Stopped, Held bool
}

// Confirm is the preview of a change; the stops lines are written by text.go.
type Confirm struct {
	Changes []Change
	Stops   []Stop
}

// Input is everything Build reads.
type Input struct {
	Rows []underlying.Row // underlying.Reader.Current, sorted by Key by Build again
	Held map[string]bool  // held symbols (shares)
	// HeldOptions are the symbols held through options; HeldRefused maps a held symbol that cannot be added to the
	// reason; HeldError is set when the positions could not be read.
	HeldOptions map[string]bool
	HeldRefused map[string]string
	HeldError   bool
	History     []underlying.Row // newest first; at most the first 20 are listed
	Form        *Form            // nil: read-only
	Confirm     *Confirm         // nil: confirm is null
	Notice      string
	Now         time.Time // generated_at, UTC; the page reads no clock
	// SessionDay is session_day, YYYY-MM-DD; empty means the day of Now in Zone.
	SessionDay string
	Zone       *time.Location // owner zone of set_at; nil is UTC
}

type pageJSON struct {
	Schema        string        `json:"schema"`
	SchemaVersion int           `json:"schema_version"`
	GeneratedAt   string        `json:"generated_at"`
	SessionDay    string        `json:"session_day"`
	Title         string        `json:"title"`
	Sentence      string        `json:"sentence"`
	Rows          []rowJSON     `json:"rows"`
	Form          *formJSON     `json:"form"`
	Confirm       *confirmJSON  `json:"confirm"`
	Notice        string        `json:"notice"`
	History       []historyJSON `json:"history"`
	Gaps          []gapJSON     `json:"gaps"`
}

type rowJSON struct {
	Key                string   `json:"key"`
	Symbol             string   `json:"symbol"`
	Market             string   `json:"market"`
	Kind               string   `json:"kind"`
	Name               string   `json:"name"`
	Status             string   `json:"status"`
	StatusText         string   `json:"status_text"`
	Reason             string   `json:"reason"`
	Issuer             string   `json:"issuer"`
	ConID              *int64   `json:"con_id"`
	Options            bool     `json:"options"`
	Fundamentals       bool     `json:"fundamentals"`
	Held               bool     `json:"held"`
	BuyPrice           string   `json:"buy_price"`
	SalePrice          string   `json:"sale_price"`
	ReportDateOverride string   `json:"report_date_override"`
	Note               string   `json:"note"`
	FollowedBy         []string `json:"followed_by"`
	FollowedText       string   `json:"followed_text"`
	SetAt              string   `json:"set_at"`
	Editable           []string `json:"editable"`
}

type formJSON struct {
	PostPath string `json:"post_path"`
	Expires  string `json:"expires"`
	MAC      string `json:"mac"`
}

type changeJSON struct {
	Label string `json:"label"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

type confirmJSON struct {
	Changes []changeJSON `json:"changes"`
	Stops   []string     `json:"stops"`
}

type historyJSON struct {
	At   string `json:"at"`
	Key  string `json:"key"`
	Text string `json:"text"`
}

type gapJSON struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

// Build returns the JSON of page.underlyings.v1 for in. Rows are sorted by key, a nil Rows gives rows [] and the
// sentence "0 names."; with a nil Form every editable list is empty, form is null and one no_reader gap is added.
func Build(in Input) ([]byte, error) {
	zone := in.Zone
	if zone == nil {
		zone = time.UTC
	}
	day := in.SessionDay
	if day == "" {
		day = in.Now.In(zone).Format(dayLayout)
	}
	p := pageJSON{
		Schema:        "underlyings",
		SchemaVersion: 1,
		GeneratedAt:   in.Now.UTC().Format(time.RFC3339),
		SessionDay:    day,
		Title:         titleText,
		Notice:        in.Notice,
		Rows:          make([]rowJSON, 0, len(in.Rows)),
		History:       []historyJSON{},
		Gaps:          []gapJSON{},
	}
	rows := slices.Clone(in.Rows)
	slices.SortStableFunc(rows, func(a, b underlying.Row) int {
		switch {
		case a.Key < b.Key:
			return -1
		case a.Key > b.Key:
			return 1
		}
		return 0
	})
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Status]++
		p.Rows = append(p.Rows, buildRow(r, in, zone))
	}
	p.Sentence = sentenceText(len(rows), counts)
	if in.Form != nil {
		p.Form = &formJSON{PostPath: postPath, Expires: in.Form.Expires, MAC: in.Form.MAC}
	} else {
		p.Gaps = append(p.Gaps, gapJSON{Kind: gapKind, Reason: gapNoReader, Text: readOnlyText})
	}
	p.Gaps = append(p.Gaps, heldGaps(in, rows)...)
	if in.Confirm != nil {
		p.Confirm = buildConfirm(*in.Confirm)
	}
	for i, h := range in.History {
		if i == historyMax {
			break
		}
		p.History = append(p.History, historyJSON{
			At: h.SetAt.In(zone).Format(timeLayout), Key: h.Key, Text: historyText(h, in.History[i+1:]),
		})
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("underlyings: %w", err)
	}
	return out, nil
}

func buildRow(r underlying.Row, in Input, zone *time.Location) rowJSON {
	held := in.Held[r.Symbol]
	groups := Follows(r.Status, r.Options, r.Fundamentals, r.Issuer != "", held)
	name := r.Name
	if name == "" {
		name = r.Symbol
	}
	var conID *int64
	if r.ConID != 0 {
		c := r.ConID
		conID = &c
	}
	stale := r.Status == underlying.StatusPending && in.Now.Sub(r.SetAt) > settleLimit
	return rowJSON{
		Key: r.Key, Symbol: r.Symbol, Market: r.Market, Kind: r.Kind, Name: name,
		Status: r.Status, StatusText: statusText(r.Status, r.Reason, stale), Reason: r.Reason,
		Issuer: r.Issuer, ConID: conID, Options: r.Options, Fundamentals: r.Fundamentals, Held: held,
		BuyPrice: r.BuyPrice, SalePrice: r.SalePrice, ReportDateOverride: r.ReportDateOverride, Note: rowNote(r),
		FollowedBy: groups, FollowedText: followedText(groups),
		SetAt:    r.SetAt.In(zone).Format(timeLayout),
		Editable: editable(r.Status, in.Form != nil),
	}
}

// editable lists the fields the form may change; exchange and currency only while the name is unsettled.
func editable(status string, form bool) []string {
	out := []string{}
	if !form {
		return out
	}
	out = append(out, "status", "name", "issuer", "options", "fundamentals", "buy_price", "sale_price", "report_date_override", "note")
	if status == underlying.StatusPending || status == underlying.StatusUnresolved {
		out = append(out, "exchange", "currency")
	}
	return out
}

func buildConfirm(c Confirm) *confirmJSON {
	out := &confirmJSON{Changes: make([]changeJSON, 0, len(c.Changes)), Stops: make([]string, 0, len(c.Stops)+1)}
	for _, ch := range c.Changes {
		out.Changes = append(out.Changes, changeJSON(ch))
	}
	stopped := false
	for _, s := range c.Stops {
		out.Stops = append(out.Stops, stopText(s))
		stopped = stopped || s.Stopped
	}
	if stopped {
		out.Stops = append(out.Stops, refillText)
	}
	return out
}

// heldGaps names every held symbol (shares or options) that is not followed, sorted by symbol; with HeldError it is
// the one read-failure gap. A row of any market counts for the symbol; an active row ends the gap, a paused or
// retired row is the owner's choice and gives none.
func heldGaps(in Input, rows []underlying.Row) []gapJSON {
	if in.HeldError {
		return []gapJSON{{Kind: gapKind, Reason: gapNoData, Text: heldReadText}}
	}
	status := map[string]string{} // best status per symbol: active > pending > unresolved > other
	rank := map[string]int{underlying.StatusActive: 3, underlying.StatusPending: 2, underlying.StatusUnresolved: 1}
	reason := map[string]string{}
	for _, r := range rows {
		if _, ok := status[r.Symbol]; !ok || rank[r.Status] > rank[status[r.Symbol]] {
			status[r.Symbol] = r.Status
			reason[r.Symbol] = r.Reason
		}
	}
	syms := map[string]bool{}
	for s, ok := range in.Held {
		syms[s] = syms[s] || ok
	}
	for s, ok := range in.HeldOptions {
		syms[s] = syms[s] || ok
	}
	var names []string
	for s, ok := range syms {
		if ok {
			names = append(names, s)
		}
	}
	slices.Sort(names)
	var gaps []gapJSON
	for _, s := range names {
		text := ""
		switch st, has := status[s]; {
		case has && st == underlying.StatusPending:
			text = heldPendingText(s)
		case has && st == underlying.StatusUnresolved:
			text = heldUnresolvedText(s, reason[s])
		case has:
		case in.HeldRefused[s] != "":
			text = heldRefusedText(s, in.HeldRefused[s])
		default:
			text = heldNoRowText(s)
		}
		if text != "" {
			gaps = append(gaps, gapJSON{Kind: gapKind, Reason: gapNoData, Text: text})
		}
	}
	return gaps
}
