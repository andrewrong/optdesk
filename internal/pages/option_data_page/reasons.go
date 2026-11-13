package option_data_page

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// The reason codes of the field reasons the page lists.
const (
	codeFieldMissing   = "field_missing"
	codeFieldsComplete = "fields_complete"
)

// reasonNotes returns the Detail, as stored, of each field reason of day whose subject is the page's symbol, in the
// order read; when there is none, the one note saying so. A nil reader or an empty symbol gives no note.
func reasonNotes(ctx context.Context, c Config, day contracts.Day) ([]string, error) {
	if c.Reasons == nil || c.Symbol == "" {
		return nil, nil
	}
	rs, err := c.Reasons.ReasonsOn(ctx, day)
	if err != nil {
		return nil, fmt.Errorf("option_data_page: read reasons of %s: %w", day, err)
	}
	var notes []string
	for _, r := range rs {
		if (r.Code == codeFieldMissing || r.Code == codeFieldsComplete) && r.Subject == c.Symbol {
			notes = append(notes, r.Detail)
		}
	}
	if len(notes) == 0 {
		return []string{fmt.Sprintf("no field reasons recorded for %s", day)}, nil
	}
	return notes, nil
}
