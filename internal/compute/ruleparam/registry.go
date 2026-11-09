package ruleparam

import (
	"errors"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
)

// ErrUnknownKey is returned for a key that is not one of the twelve.
var ErrUnknownKey = errors.New("ruleparam: unknown key")

// ErrRefused is returned for a text the kind or the range of its key refuses.
var ErrRefused = errors.New("ruleparam: value refused")

// Kinds of a parameter.
const (
	kindInt     = "int"
	kindDecimal = "decimal"
)

// specs is the one table of the rule parameters: defaults from
// option-candidates.md Decision 15 and option-positions.md Decision 13,
// bounds from Decision 3 of docs/design/settings-form.md. Defaults are
// canonical text. A key is added here, by a reviewed change, never by the form.
var specs = []settingsapi.RuleParamSpec{
	{Key: "min_dte", Label: "Minimum days to expiry", Unit: "days", Kind: kindInt, Default: "21", Min: "7", Max: "120"},
	{Key: "max_dte", Label: "Maximum days to expiry", Unit: "days", Kind: kindInt, Default: "45", Min: "14", Max: "180"},
	{Key: "min_bid_size", Label: "Minimum bid size", Unit: "contracts", Kind: kindInt, Default: "1", Min: "1", Max: "100"},
	{Key: "max_spread", Label: "Maximum bid-ask spread", Unit: "ratio", Kind: kindDecimal, Default: "0.1", Min: "0.01", Max: "0.3"},
	{Key: "min_open_interest", Label: "Minimum open interest", Unit: "contracts", Kind: kindInt, Default: "10", Min: "1", Max: "1000"},
	{Key: "div_guard_factor", Label: "Dividend guard factor", Unit: "factor", Kind: kindDecimal, Default: "1", Min: "0.5", Max: "3"},
	{Key: "max_rate_age_days", Label: "Maximum rate age", Unit: "days", Kind: kindInt, Default: "7", Min: "1", Max: "30"},
	{Key: "max_event_age_sessions", Label: "Maximum event age", Unit: "sessions", Kind: kindInt, Default: "2", Min: "1", Max: "10"},
	{Key: "min_report_gap_days", Label: "Minimum days between reports", Unit: "days", Kind: kindInt, Default: "60", Min: "30", Max: "100"},
	{Key: "max_strikes", Label: "Maximum strikes per expiry", Unit: "strikes", Kind: kindInt, Default: "24", Min: "4", Max: "60"},
	{Key: "capture_target", Label: "Premium capture target", Unit: "ratio", Kind: kindDecimal, Default: "0.5", Min: "0.1", Max: "0.95"},
	{Key: "exit_dte", Label: "Exit days to expiry", Unit: "days", Kind: kindInt, Default: "21", Min: "1", Max: "45"},
}

type registry struct{}

// New builds the registry; it is a settingsapi.NewRuleParamsFunc.
func New() settingsapi.RuleParams { return registry{} }

// Specs returns a copy of the twelve keys in their fixed order.
func (registry) Specs() []settingsapi.RuleParamSpec {
	return append([]settingsapi.RuleParamSpec(nil), specs...)
}

// Resolve gives every key the stored text, or the default when the text is
// missing or empty. Keys outside the table are dropped.
func (registry) Resolve(stored map[string]string) map[string]string {
	out := make(map[string]string, len(specs))
	for _, s := range specs {
		if v := stored[s.Key]; v != "" {
			out[s.Key] = v
		} else {
			out[s.Key] = s.Default
		}
	}
	return out
}

func lookup(key string) (settingsapi.RuleParamSpec, bool) {
	for _, s := range specs {
		if s.Key == key {
			return s, true
		}
	}
	return settingsapi.RuleParamSpec{}, false
}
