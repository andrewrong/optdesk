package contracts_test

import "testing"

func TestOptionCandidatesCountsOptional(t *testing.T) {
	t.Parallel()
	inst := candidatesMinimal(t)
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("instance without source_counts is invalid")
	}
	inst["source_counts"] = map[string]any{"open": 3, "eod_fallback": 2}
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("source_counts without none is invalid")
	}
	inst["source_counts"] = map[string]any{"open": 3, "eod_fallback": 2, "none": 1}
	if !pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("source_counts with none is invalid")
	}
	inst["source_counts"] = map[string]any{"open": 3}
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("source_counts without eod_fallback validates")
	}
	inst["source_counts"] = map[string]any{"open": 3, "eod_fallback": 2, "x": 1}
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("source_counts with an unknown key validates")
	}
	inst["source_counts"] = map[string]any{"open": 3.5, "eod_fallback": 2}
	if pageV1Valid(t, optionCandidatesPageV1, inst) {
		t.Fatal("non-integer count validates")
	}
}
