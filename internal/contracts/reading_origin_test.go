package contracts_test

import "testing"

func TestReadingOwnerCommandOrigin(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "readings.reading")
	recs, _ := loadRecords(t, "readings.reading")
	if len(recs) != 4 {
		t.Fatalf("want 4 records, got %d", len(recs))
	}
	r := recs[3]
	if r["name"] != "valuation.band_approval" || r["origin"] != "owner_command" || r["unit"] != "money" || r["state"] != "ok" {
		t.Fatalf("record 3 must be the band approval: %v", r)
	}
	if err := s.Validate(r); err != nil {
		t.Fatalf("band approval: %v", err)
	}
	bad := map[string]any{}
	for k, v := range r {
		bad[k] = v
	}
	bad["origin"] = "other"
	if err := s.Validate(bad); err == nil {
		t.Fatal("origin other must be rejected")
	}
}
