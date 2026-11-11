package market_page

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestIdsExistInRulesAndJob(t *testing.T) {
	t.Parallel()
	rb, err := os.ReadFile("../../compute/rules/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var rules []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rb, &rules); err != nil {
		t.Fatal(err)
	}
	inRules := map[string]bool{}
	for _, r := range rules {
		inRules[r.ID] = true
	}
	jb, err := os.ReadFile("../../jobs/computefigures/job.toml")
	if err != nil {
		t.Fatal(err)
	}
	var job struct {
		Market []struct {
			ID     string   `toml:"id"`
			Series []string `toml:"series"`
		} `toml:"market"`
	}
	if err := toml.Unmarshal(jb, &job); err != nil {
		t.Fatal(err)
	}
	inJob := map[string][]string{}
	for _, m := range job.Market {
		inJob[m.ID] = m.Series
	}
	for _, id := range []string{figVixPctile, figVxnPctile, figRvxPctile, figSp500Vs200d, figVixTerm, figRv20} {
		if !inRules[id] {
			t.Errorf("%s is not in rules.json", id)
		}
		if _, ok := inJob[id]; !ok && id != figRv20 { // market.rv20 is keyed by rv20, not [[market]]
			t.Errorf("%s is not in job.toml", id)
		}
	}
	for _, s := range indexSpecs[:3] {
		if got := inJob[s.figure]; len(got) == 0 || got[0] != s.series {
			t.Errorf("%s reads %v, the row says %s", s.figure, got, s.series)
		}
	}
	if got := inJob[figVixTerm]; len(got) != 2 || got[0] != seriesVIX || got[1] != seriesVXV {
		t.Errorf("term ratio series = %v", got)
	}
	if got := inJob[figSp500Vs200d]; len(got) == 0 || got[0] != seriesSP500 {
		t.Errorf("sp500 series = %v", got)
	}
}

func TestSeriesAreStoredByFredseries(t *testing.T) {
	t.Parallel()
	fb, err := os.ReadFile("../../jobs/fredseries/job.toml")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range dataTimeSeries {
		if !strings.Contains(string(fb), `"`+s+`"`) {
			t.Errorf("%s is not stored by fredseries", s)
		}
	}
}

func TestRv20IsInManifest(t *testing.T) {
	t.Parallel()
	jb, err := os.ReadFile("../../jobs/computefigures/job.toml")
	if err != nil {
		t.Fatal(err)
	}
	var job struct {
		Rv20        string   `toml:"rv20"`
		Rv20Indices []string `toml:"rv20_indices"`
	}
	if err := toml.Unmarshal(jb, &job); err != nil {
		t.Fatal(err)
	}
	if got := job.Rv20; got != figRv20 {
		t.Errorf("job.toml rv20 = %q, want %s", got, figRv20)
	}
	if got := job.Rv20Indices; len(got) != 3 || got[0] != "SPX" || got[1] != "NDX" || got[2] != "RUT" {
		t.Errorf("job.toml rv20_indices = %v", got)
	}
}
