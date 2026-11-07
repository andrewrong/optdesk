package option_data_page

import (
	"fmt"
	"regexp"
)

const maxChoices = 50

var hrefPattern = regexp.MustCompile(`^/[A-Za-z0-9/_-]{1,100}$`)

type choiceView struct {
	ID      int64  `json:"id"`
	Label   string `json:"label"`
	Href    string `json:"href"`
	Current bool   `json:"current"`
}

// checkChoices rejects a Href outside the schema pattern among the choices that are kept.
func checkChoices(choices []Choice) error {
	for _, c := range limit(choices, maxChoices) {
		if !hrefPattern.MatchString(c.Href) {
			return fmt.Errorf("option_data_page: bad href %q of choice %d", c.Href, c.ID)
		}
	}
	return nil
}

// choiceViews keeps the first maxChoices choices in the given order and marks the one whose ID
// is current. No choices give nil.
func choiceViews(choices []Choice, current int64) []choiceView {
	var out []choiceView
	for _, c := range limit(choices, maxChoices) {
		out = append(out, choiceView{ID: c.ID, Label: c.Label, Href: c.Href, Current: c.ID == current})
	}
	return out
}
