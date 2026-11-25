package contracts_test

import "testing"

func nameOutcomesInst(t *testing.T, outcome any) map[string]any {
	t.Helper()
	inst := candidatesMinimal(t)
	if outcome != nil {
		candidatesGroup(inst)["name_outcomes"] = []any{map[string]any{
			"name": "Toyota", "outcome": outcome, "text": "无报价", "counts": "0 行",
		}}
	}
	return inst
}

func TestNameOutcomesValid(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, optionCandidatesPageV1, nameOutcomesInst(t, "no_quote")) {
		t.Fatal("a group with name_outcomes is invalid")
	}
}

func TestNameOutcomesOpenString(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, optionCandidatesPageV1, nameOutcomesInst(t, "some_future_outcome")) {
		t.Fatal("an unknown outcome string is invalid")
	}
	if pageV1Valid(t, optionCandidatesPageV1, nameOutcomesInst(t, 3)) {
		t.Fatal("a number outcome validates")
	}
}

func TestNoNameOutcomesStillValid(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, optionCandidatesPageV1, nameOutcomesInst(t, nil)) {
		t.Fatal("a group without name_outcomes is invalid")
	}
}
