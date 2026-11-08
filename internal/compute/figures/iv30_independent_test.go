package figures_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/contracttest"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type independentIn struct {
	Rule struct {
		ID         string `json:"id"`
		Version    int    `json:"version"`
		Unit       string `json:"unit"`
		Scale      int    `json:"scale"`
		MaxAgeDays int    `json:"max_age_days"`
	} `json:"rule"`
	AsOf string `json:"as_of"`
	Rate struct {
		Day   string  `json:"day"`
		Value float64 `json:"value"`
	} `json:"rate"`
	Chain []struct {
		Minutes int `json:"minutes_to_expiry"`
		Strikes []struct {
			Strike  float64 `json:"strike"`
			CallIV  float64 `json:"call_iv"`
			PutIV   float64 `json:"put_iv"`
			CallMid float64 `json:"call_mid"`
			PutMid  float64 `json:"put_mid"`
			HasCall bool    `json:"has_call"`
			HasPut  bool    `json:"has_put"`
		} `json:"strikes"`
	} `json:"chain"`
}

type independentWant struct {
	Status       compute.Status `json:"status"`
	Reason       compute.Reason `json:"reason"`
	Value        float64        `json:"value"`
	DataDay      string         `json:"data_day"`
	Observations int            `json:"observations"`
	StaleRate    bool           `json:"stale_rate"`
}

func TestIv30MatchesIndependent(t *testing.T) {
	t.Parallel()
	f, err := contracttest.Load("iv30_independent.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var in independentIn
			var want independentWant
			if err := json.Unmarshal(c.Input, &in); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(c.Expected, &want); err != nil {
				t.Fatal(err)
			}
			chain := make([]compute.ExpiryQuotes, len(in.Chain))
			for i, e := range in.Chain {
				chain[i].MinutesToExpiry = e.Minutes
				for _, s := range e.Strikes {
					chain[i].Strikes = append(chain[i].Strikes, compute.StrikeQuote(s))
				}
			}
			rule := compute.Rule{ID: in.Rule.ID, Version: in.Rule.Version, Unit: in.Rule.Unit, Scale: in.Rule.Scale, MaxAgeDays: in.Rule.MaxAgeDays}
			got := build(rule).Iv30(rule.ID, chain, compute.Point{Day: contracts.Day(in.Rate.Day), Value: in.Rate.Value}, contracts.Day(in.AsOf))
			if got.Status != want.Status || got.Reason != want.Reason || got.Observations != want.Observations ||
				string(got.DataDay) != want.DataDay || got.StaleRate != want.StaleRate ||
				math.Abs(got.Value-want.Value) > c.Tolerance {
				t.Errorf("got %+v, want %+v (tolerance %g)", got, want, c.Tolerance)
			}
		})
	}
}
