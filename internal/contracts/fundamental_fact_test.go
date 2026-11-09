package contracts_test

import "testing"

func TestFundamentalFactFixture(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "obs.fundamental_fact")
	recs, _ := loadRecords(t, "obs.fundamental_fact")
	if len(recs) != 4 {
		t.Fatalf("want 4 records, got %d", len(recs))
	}
	for i, r := range recs {
		if err := s.Validate(r); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}
	if recs[3]["source"] != "edinet" || recs[3]["currency"] != "JPY" {
		t.Fatal("record 3 must be the JPY edinet record")
	}

	mutate := func(base map[string]any, f func(map[string]any)) map[string]any {
		m := map[string]any{}
		for k, v := range base {
			m[k] = v
		}
		f(m)
		return m
	}
	cases := map[string]map[string]any{
		"source fmp":                 mutate(recs[0], func(m map[string]any) { m["source"] = "fmp" }),
		"duration without start":     mutate(recs[1], func(m map[string]any) { delete(m, "period_start") }),
		"no subject":                 mutate(recs[0], func(m map[string]any) { delete(m, "subject") }),
		"subject not an issuer text": mutate(recs[0], func(m map[string]any) { m["subject"] = "AAPL" }),
		"value not decimal text":     mutate(recs[0], func(m map[string]any) { m["value"] = 1.5 }),
		"unknown field":              mutate(recs[0], func(m map[string]any) { m["extra"] = 1 }),
	}
	for label, rec := range cases {
		if err := s.Validate(rec); err == nil {
			t.Fatalf("%s must be rejected", label)
		}
	}
}
