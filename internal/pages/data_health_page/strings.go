package data_health_page

const (
	pageName    = "data_health_page"
	runID       = "data_health"
	timeLayout  = "2006-01-02T15:04:05-07:00"
	fredDataset = "obs.series_point"
	gapReason   = "coverage_unavailable"
)

// glossary is the list of Decision 12, plus the repair values of a cell.
func glossary() []GlossaryRow {
	return []GlossaryRow{
		{"ok", zhGlossOK},
		{"no_value", zhGlossNoValue},
		{"expired", zhGlossExpired},
		{"missing", zhGlossMissing},
		{"not_run", zhGlossNotRun},
		{"none", zhGlossNone},
		{"button", zhGlossButton},
		{"not refetchable", zhGlossNotRefetch},
		{"ledger", zhGlossLedger},
		{"session day", zhGlossSessionDay},
	}
}
