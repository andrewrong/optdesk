package companies_page

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type ownerFifth struct{ fakeText }

func (ownerFifth) FifthQuestion(context.Context, string) (string, bool, error) {
	return "Repaid debt every year.", true, nil
}

func TestQuestionsFromReadings(t *testing.T) {
	t.Parallel()
	cfg := coreConfig("120")
	cfg.Text = ownerFifth{}
	cfg.Subjects = append(cfg.Subjects, Subject{Market: "US", Symbol: "NOID"})
	cfg.Questions = func(_ valuationapi.Filings, _ contracts.Day) valuationapi.QuestionsResult {
		return valuationapi.QuestionsResult{Questions: []valuationapi.Question{
			{Key: "Q1", State: valuationapi.StateOK, Value: "0.1825"},
			{Key: "Q2", State: valuationapi.StateOK, Value: "0.4149"},
			{Key: "Q3", State: valuationapi.StateMissingInput, Reason: "margin_input_missing"},
		}}
	}
	var blk struct {
		Rows []struct {
			Symbol string          `json:"symbol"`
			Q1     map[string]any  `json:"q1"`
			Q2     map[string]any  `json:"q2"`
			Q3     map[string]any  `json:"q3"`
			Q4     map[string]any  `json:"q4"`
			Fifth  json.RawMessage `json:"fifth"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(buildBlocks(t, cfg)["questions"], &blk); err != nil {
		t.Fatal(err)
	}
	if len(blk.Rows) != 2 {
		t.Fatalf("rows = %+v", blk.Rows)
	}
	r := blk.Rows[0]
	if r.Q1["text"] != "18.2%" || r.Q1["unit"] != "percent" || r.Q1["value"] != 18.2 {
		t.Errorf("q1 = %v (0.1825 is 18.2 half-even)", r.Q1)
	}
	if r.Q2["text"] != "0.41" || r.Q2["unit"] != "ratio" {
		t.Errorf("q2 = %v", r.Q2)
	}
	if r.Q3["reason"] != "missing_input" || r.Q3["text"] != "margin input missing" {
		t.Errorf("q3 = %v", r.Q3)
	}
	if r.Q4["reason"] != "missing_input" {
		t.Errorf("q4 not given must be a gap, got %v", r.Q4)
	}
	if string(r.Fifth) != `"Repaid debt every year."` {
		t.Errorf("fifth = %s", r.Fifth)
	}
	n := blk.Rows[1]
	if n.Q1["reason"] != "missing_input" || n.Q1["text"] != "no issuer id" {
		t.Errorf("subject with no issuer: q1 = %v", n.Q1)
	}
}

func TestQuestionsNilFunctionGivesGap(t *testing.T) {
	t.Parallel()
	cfg := coreConfig("120")
	cfg.Questions = nil
	var blk map[string]any
	if err := json.Unmarshal(buildBlocks(t, cfg)["questions"], &blk); err != nil {
		t.Fatal(err)
	}
	if blk["reason"] != "no_reader" {
		t.Fatalf("questions = %v", blk)
	}
}
