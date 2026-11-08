package contracts_test

import "testing"

const diagnosisPageV1 = "page.diagnosis_page.v1.json"

func diagnosisMinimal(t *testing.T) map[string]any {
	t.Helper()
	return pageV1Minimal(t, diagnosisPageV1, nil)
}

func diagGroup(key any, rows []any) map[string]any {
	g := map[string]any{"title": "g", "columns": []any{"c"}, "rows": rows, "legend": []any{"l"}}
	if key != nil {
		g["key"] = key
	}
	return g
}

func diagYear() map[string]any {
	return map[string]any{"fy": "FY24", "revenue": "1", "margin": "2", "extra": "3", "bar_percent": 40}
}

func diagRow(years []any) map[string]any {
	return map[string]any{"symbol": "AMD", "latest_fy": "FY24", "latest": "1", "rank_text": "3/10",
		"extra": "e", "usual": "u", "years": years}
}

func withGroup(inst map[string]any, g map[string]any) map[string]any {
	inst["groups"] = []any{g}
	return inst
}

func TestDiagnosisPageMinimalValid(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, diagnosisPageV1, diagnosisMinimal(t)) {
		t.Fatal("minimal instance is invalid")
	}
}

func TestDiagnosisPageVersion2Fails(t *testing.T) {
	t.Parallel()
	inst := diagnosisMinimal(t)
	inst["schema_version"] = 2
	if pageV1Valid(t, diagnosisPageV1, inst) {
		t.Fatal("version 2 validates")
	}
}

func TestDiagnosisPageGroupKey(t *testing.T) {
	t.Parallel()
	if pageV1Valid(t, diagnosisPageV1, withGroup(diagnosisMinimal(t), diagGroup(nil, []any{}))) {
		t.Fatal("DiagGroup without key validates")
	}
	if !pageV1Valid(t, diagnosisPageV1, withGroup(diagnosisMinimal(t), diagGroup("other", []any{}))) {
		t.Fatal("DiagGroup with key other is invalid")
	}
}

func TestDiagnosisPageYearsBound(t *testing.T) {
	t.Parallel()
	for n, want := range map[int]bool{12: true, 13: false} {
		years := make([]any, n)
		for i := range years {
			years[i] = diagYear()
		}
		inst := withGroup(diagnosisMinimal(t), diagGroup("k", []any{diagRow(years)}))
		if got := pageV1Valid(t, diagnosisPageV1, inst); got != want {
			t.Errorf("%d years: valid = %v, want %v", n, got, want)
		}
	}
}

func TestDiagnosisPageRowAsGapValid(t *testing.T) {
	t.Parallel()
	inst := withGroup(diagnosisMinimal(t), diagGroup("k", []any{pageV1Gap()}))
	if !pageV1Valid(t, diagnosisPageV1, inst) {
		t.Fatal("DiagRow as a Gap is invalid")
	}
}
