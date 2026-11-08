package home_page

import (
	"encoding/json"
	"fmt"
	"strings"
)

const maxGaps = 20

// CardLinks holds the page each of the five cards links to; every value starts
// with "/".
type CardLinks struct {
	Macro, Market, Household, Companies, Options string
}

type overviewLine struct {
	Label string `json:"label"`
	Text  string `json:"text"`
	Href  string `json:"href"`
}

type gapItem struct {
	Card string `json:"card"`
	Text string `json:"text"`
}

type overview struct {
	Items []overviewLine `json:"items"`
	Gaps  []gapItem      `json:"gaps"`
}

// cardView holds the fields of a member's card that the overview copies.
type cardView struct {
	Card struct {
		Title    string `json:"title"`
		Say      string `json:"say"`
		Subtitle string `json:"subtitle"`
		Gaps     []struct {
			Text string `json:"text"`
		} `json:"gaps"`
	} `json:"card"`
}

// buildOverview copies one line per card (macro, market, household, companies,
// options) and every member gap, in that order, from the members. The text of a
// line is card.say, else card.subtitle; a card with neither is an error naming
// its slot, and nothing is invented.
func buildOverview(in Input) (overview, error) {
	parts := []struct {
		name, href string
		doc        []byte
	}{
		{"macro", in.CardLinks.Macro, in.Macro},
		{"market", in.CardLinks.Market, in.Market},
		{"household", in.CardLinks.Household, in.Household},
		{"companies", in.CardLinks.Companies, in.Companies},
		{"options", in.CardLinks.Options, in.Options},
	}
	ov := overview{Items: []overviewLine{}, Gaps: []gapItem{}}
	for _, p := range parts {
		if !strings.HasPrefix(p.href, "/") {
			return overview{}, fmt.Errorf("home_page: %s: card link %q does not start with /", p.name, p.href)
		}
		var c cardView
		if err := json.Unmarshal(p.doc, &c); err != nil {
			return overview{}, fmt.Errorf("home_page: %s: not a JSON object: %w", p.name, err)
		}
		if c.Card.Title == "" {
			return overview{}, fmt.Errorf("home_page: %s: card has no title", p.name)
		}
		text := c.Card.Say
		if text == "" {
			text = c.Card.Subtitle
		}
		if text == "" {
			return overview{}, fmt.Errorf("home_page: %s: card has neither say nor subtitle", p.name)
		}
		ov.Items = append(ov.Items, overviewLine{Label: c.Card.Title, Text: text, Href: p.href})
		for _, g := range c.Card.Gaps {
			ov.Gaps = append(ov.Gaps, gapItem{Card: c.Card.Title, Text: g.Text})
		}
	}
	if len(ov.Gaps) > maxGaps {
		return overview{}, fmt.Errorf("home_page: overview.gaps: %d gaps, at most %d", len(ov.Gaps), maxGaps)
	}
	return ov, nil
}

// gapLine words the number of gaps for the page's gap_line.
func gapLine(n int) string {
	if n == 0 {
		return "今天没有缺口"
	}
	return fmt.Sprintf("%d 处缺口，见下", n)
}
