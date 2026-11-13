package contracts_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func strategyResultRow() map[string]any {
	return map[string]any{
		"session_day": "2026-11-11", "run_id": "run-1", "strategy": "short_put", "rule_version": "put_v1",
		"param_hash": "abc123", "key": "1001", "underlying_id": 7, "status": "ok",
		"rank": 1, "rank_in_underlying": 1, "legs": "[]", "checks": "[]", "facts": "{}",
		"computed_at": "2026-11-11T21:00:00Z", "settings_version": nil,
	}
}

func TestStrategyResultRecordRoundTrip(t *testing.T) {
	t.Parallel()
	enc := func(m map[string]any) []byte {
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	var rec contracts.StrategyResult
	if err := json.Unmarshal(enc(strategyResultRow()), &rec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	var again contracts.StrategyResult
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatalf("unmarshal of marshalled record: %v", err)
	}
	if !reflect.DeepEqual(rec, again) {
		t.Fatalf("round trip differs: %+v vs %+v", rec, again)
	}
	if rec.Strategy != "short_put" || rec.Key != "1001" {
		t.Fatalf("fields not read: %+v", rec)
	}

	nullRank := strategyResultRow()
	nullRank["rank"] = nil
	var r contracts.StrategyResult
	if err := json.Unmarshal(enc(nullRank), &r); err != nil {
		t.Fatalf("null rank refused: %v", err)
	}
	if r.Rank != nil {
		t.Fatalf("null rank read as %v", *r.Rank)
	}

	nullStrategy := strategyResultRow()
	nullStrategy["strategy"] = nil
	var s contracts.StrategyResult
	if err := json.Unmarshal(enc(nullStrategy), &s); err == nil {
		t.Fatal("null strategy accepted")
	} else if !strings.Contains(err.Error(), "strategy") {
		t.Fatalf("error does not name the field: %v", err)
	}
}
