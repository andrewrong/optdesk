package household_page_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/pages/household_page"
)

// oldEnglishTexts are English sentences the account page wrote before the owner rule: none is written now.
var oldEnglishTexts = []string{
	"Net liquidation of the account", "Market value of held stocks", "Which positions drove",
	"Net liquidation on recent trading days", "Cash after all short puts", "Premium collected",
	"Net liquidation from the last trading day", "Cash minus the cash needed", "Cash to pay if each",
	"Share of net value held", "Net liquidation, cash, stocks and options",
	"Net value, cash and option obligations", "no data yet", "No data", "No reader", "Unknown account",
	"Needs position prices", "Input data is missing", "Exchange rate is missing", "Pending: ",
	"No holding-period source", "The change today needs",
}

var hanText = regexp.MustCompile(`\p{Han}`)

// explanationTexts collects the sentence, every meaning line and every gap text of a page.
func explanationTexts(m map[string]any) []string {
	var out []string
	if s, ok := m["sentence"].(string); ok {
		out = append(out, s)
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				if s, ok := e.(string); ok && (k == "meaning" || k == "gap" || k == "text" && x["reason"] != nil) {
					out = append(out, s)
				}
				walk(e)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(m)
	return out
}

func TestHouseholdTextsAreChinese(t *testing.T) {
	t.Parallel()
	noReader, _ := build(t, household_page.Config{RunID: "run1", BuildID: "b1", Zone: zone, Aliases: []string{"main"}}, "main")
	var got blockapi.HouseholdInput
	noPrices, _ := build(t, config(account(), &got), "main")
	var texts []string
	for _, m := range []map[string]any{noReader, noPrices} {
		texts = append(texts, explanationTexts(m)...)
	}
	texts = append(texts, household_page.MeaningLines...)
	texts = append(texts, household_page.PageSentence)
	for _, text := range household_page.GapTexts() {
		texts = append(texts, text)
	}
	if len(texts) < 20 {
		t.Fatalf("only %d texts collected: %v", len(texts), texts)
	}
	for _, text := range texts {
		if !hanText.MatchString(text) {
			t.Errorf("text %q holds no Chinese character", text)
		}
		for _, o := range oldEnglishTexts {
			if strings.Contains(text, o) {
				t.Errorf("text %q holds the old English sentence %q", text, o)
			}
		}
	}
}
