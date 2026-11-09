package contracts_test

import "testing"

func TestOptionCandidateSettingsVersion(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "candidates.option_candidate")
	recs, _ := loadRecords(t, "candidates.option_candidate")
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d", len(recs))
	}
	for i, r := range recs {
		if err := s.Validate(r); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}
	if _, ok := recs[1]["settings_version"]; ok {
		t.Fatal("record 1 must omit settings_version")
	}
	with := map[string]any{}
	for k, v := range recs[0] {
		with[k] = v
	}
	for label, v := range map[string]any{"integer": 7, "null": nil} {
		with["settings_version"] = v
		if err := s.Validate(with); err != nil {
			t.Fatalf("settings_version %s must be accepted: %v", label, err)
		}
	}
	for label, v := range map[string]any{"text": "7", "negative": -1, "fraction": 1.5} {
		with["settings_version"] = v
		if err := s.Validate(with); err == nil {
			t.Fatalf("settings_version %s must be rejected", label)
		}
	}
}
