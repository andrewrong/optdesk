package contracts_test

import "testing"

func tierCompaniesInst(t *testing.T, companies any) map[string]any {
	t.Helper()
	inst := candidatesMinimal(t)
	view := tiersView()
	if companies != nil {
		view["tiers"].([]any)[0].(map[string]any)["companies"] = companies
	}
	inst["short_put_tiers"] = view
	return inst
}

func TestTierCompaniesValid(t *testing.T) {
	t.Parallel()
	inst := tierCompaniesInst(t, []any{
		map[string]any{"company": "AAPL", "count": 2, "row_ids": []any{"r1", "r2"}},
		map[string]any{"company": "MSFT", "count": 0, "row_ids": []any{}},
	})
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a tier with companies is invalid")
	}
}

func TestTierCompaniesNeedsCount(t *testing.T) {
	t.Parallel()
	inst := tierCompaniesInst(t, []any{
		map[string]any{"company": "AAPL", "row_ids": []any{"r1"}},
	})
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a companies item without count validates")
	}
}

func TestNoTierCompaniesStillValid(t *testing.T) {
	t.Parallel()
	inst := tierCompaniesInst(t, nil)
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("a tier without companies is invalid")
	}
	if inst["schema_version"] != float64(1) && inst["schema_version"] != 1 {
		t.Fatalf("schema_version = %v, want 1", inst["schema_version"])
	}
}
