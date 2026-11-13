// Package price_lines_page builds the page.price_lines_page.v1 view model (board 14) from the stored underlying
// rows, the held symbols and the form token. It reads no store and no clock and writes nothing.
// Design: docs/design/price-lines.md#1 price-lines-page.
package price_lines_page

import (
	"cmp"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

const (
	postPath = "/plan/price-lines"

	titleText    = "Price lines"
	sentenceText = "Your buy price for each name you keep. The put gate reads it."
	introText    = "A saved price applies to the next option candidate run; clearing it removes the check for that name."
	addHintText  = "Type the code and market of a name on the underlying list."
	footnoteText = "The price is the buy_price of the name on the underlyings page; the note is shared with it."
	noReaderText = "The underlying list is not readable here."

	maxText = 200
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Form is the token of the form, made by the caller; nil in Input is read-only (the inert constant token).
type Form struct{ Expires, MAC string }

// Input is everything Build reads.
type Input struct {
	RunID, BuildID, Scope string
	Rows                  []underlying.Row // underlying.Reader.Current
	Held                  map[string]bool  // held stock symbols
	Reader                bool             // false: the reader is missing
	Form                  *Form            // nil: read-only
	Saved, Error          string           // empty: null
	Now                   time.Time        // generated_at; the page reads no clock
}

type gapJSON struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type formJSON struct {
	PostPath string `json:"post_path"`
	Expires  string `json:"expires"`
	MAC      string `json:"mac"`
}

type rowJSON struct {
	Symbol string  `json:"symbol"`
	Market string  `json:"market"`
	Price  *string `json:"price"`
	Note   *string `json:"note"`
}

type unsetJSON struct {
	Symbol string `json:"symbol"`
	Market string `json:"market"`
}

type addJSON struct {
	Markets []string `json:"markets"`
	Hint    string   `json:"hint"`
}

type pageJSON struct {
	Schema        string      `json:"schema"`
	SchemaVersion int         `json:"schema_version"`
	RunID         string      `json:"run_id"`
	BuildID       string      `json:"build_id"`
	Scope         string      `json:"scope"`
	GeneratedAt   string      `json:"generated_at"`
	DataTimes     []struct{}  `json:"data_times"`
	RunStatus     string      `json:"run_status"`
	Stale         *string     `json:"stale"`
	Title         string      `json:"title"`
	Sentence      string      `json:"sentence"`
	Notes         []string    `json:"notes"`
	Gaps          []gapJSON   `json:"gaps"`
	Intro         string      `json:"intro"`
	Saved         *string     `json:"saved"`
	Error         *string     `json:"error"`
	Form          formJSON    `json:"form"`
	Rows          []rowJSON   `json:"rows"`
	Add           addJSON     `json:"add"`
	Unset         []unsetJSON `json:"unset"`
	Footnote      string      `json:"footnote"`
}

// Build returns the JSON of page.price_lines_page.v1 for in. Rows with a buy price are listed sorted by market then
// symbol; unset lists held names with no price that are not retired, in the order of Rows. With Reader false the
// page has one no_reader gap, run_status partial, no rows and no unset. A bad id is an error.
func Build(in Input) ([]byte, error) {
	for name, v := range map[string]string{"run_id": in.RunID, "build_id": in.BuildID, "scope": in.Scope} {
		if !idPattern.MatchString(v) {
			return nil, fmt.Errorf("price_lines_page: bad %s %q", name, v)
		}
	}
	form := formJSON{PostPath: postPath, Expires: "0", MAC: strings.Repeat("0", 64)}
	if in.Form != nil {
		form.Expires, form.MAC = in.Form.Expires, in.Form.MAC
	}
	p := pageJSON{
		Schema: "price_lines_page", SchemaVersion: 1,
		RunID: in.RunID, BuildID: in.BuildID, Scope: in.Scope,
		GeneratedAt: in.Now.UTC().Format(time.RFC3339),
		DataTimes:   []struct{}{}, RunStatus: "ok",
		Title: titleText, Sentence: sentenceText, Notes: []string{}, Gaps: []gapJSON{},
		Intro: introText, Saved: text(in.Saved), Error: text(in.Error), Form: form,
		Rows: []rowJSON{}, Unset: []unsetJSON{},
		Add:      addJSON{Markets: []string{"US", "JP"}, Hint: addHintText},
		Footnote: footnoteText,
	}
	if !in.Reader {
		p.RunStatus = "partial"
		p.Gaps = append(p.Gaps, gapJSON{Kind: "gap", Reason: "no_reader", Text: noReaderText})
	} else {
		for _, r := range in.Rows {
			switch {
			case r.BuyPrice != "":
				p.Rows = append(p.Rows, rowJSON{Symbol: r.Symbol, Market: r.Market, Price: text(r.BuyPrice), Note: text(r.Note)})
			case in.Held[r.Symbol] && r.Status != underlying.StatusRetired:
				p.Unset = append(p.Unset, unsetJSON{Symbol: r.Symbol, Market: r.Market})
			}
		}
		slices.SortStableFunc(p.Rows, func(a, b rowJSON) int {
			return cmp.Or(cmp.Compare(a.Market, b.Market), cmp.Compare(a.Symbol, b.Symbol))
		})
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("price_lines_page: %w", err)
	}
	return out, nil
}

// text is s cut to the 200 characters of the schema, nil when empty.
func text(s string) *string {
	if s == "" {
		return nil
	}
	if r := []rune(s); len(r) > maxText {
		s = string(r[:maxText])
	}
	return &s
}
