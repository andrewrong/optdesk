package contracts_test

import "testing"

func flexFetchRecord(over map[string]any) map[string]any {
	rec := map[string]any{
		"fetch_id": "f1", "login": "main", "query_id": "123456",
		"first_date": "2026-01-02", "last_date": "2026-01-31",
		"started_at": "2026-02-01T10:00:00Z", "finished_at": "2026-02-01T10:00:05Z",
		"status": "ok", "error_kind": "", "error_code": "",
		"raw_sha256":     "",
		"parser_version": "1", "statements": 1, "statements_unmapped": 0,
		"rows_parsed": 2, "rows_new": 2, "rows_revised": 0, "rows_unchanged": 0,
	}
	for k, v := range over {
		rec[k] = v
	}
	return rec
}

func TestFlexFetchR2Accepts(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "optionledger.flex_fetch")
	cases := map[string]map[string]any{
		"daily record":   flexFetchRecord(nil),
		"import success": flexFetchRecord(map[string]any{"login": "import", "query_id": "file:0a1b2c3d:1:main"}),
		"import failure": flexFetchRecord(map[string]any{
			"login": "import", "query_id": "file:0a1b2c3d:3:acct-b",
			"first_date": "2026-02-01", "last_date": "2026-02-01",
			"status": "failed", "error_kind": "import",
		}),
	}
	for name, rec := range cases {
		if err := s.Validate(rec); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestFlexFetchR2Rejects(t *testing.T) {
	t.Parallel()
	s := compileDataset(t, "optionledger.flex_fetch")
	cases := map[string]map[string]any{
		"short request id":    {"query_id": "file:0a1b2c3:1:main"},
		"ordinal 0":           {"query_id": "file:0a1b2c3d:0:main"},
		"empty alias":         {"query_id": "file:0a1b2c3d:1:"},
		"colon in alias":      {"query_id": "file:0a1b2c3d:1:a:b"},
		"empty first date":    {"first_date": ""},
		"first date not date": {"first_date": "yesterday"},
		"query id letters":    {"query_id": "abc"},
		"status other":        {"status": "done"},
	}
	for name, over := range cases {
		if err := s.Validate(flexFetchRecord(over)); err == nil {
			t.Errorf("%s: record validated, want an error", name)
		}
	}
}
