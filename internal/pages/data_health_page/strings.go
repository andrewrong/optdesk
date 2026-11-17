package data_health_page

const (
	pageName      = "data_health_page"
	runID         = "data_health"
	timeLayout    = "2006-01-02T15:04:05-07:00"
	fredDataset   = "obs.series_point"
	gapReason     = "coverage_unavailable"
	gapText       = "The coverage of the ledger could not be read, so no dataset is shown."
	overwriteNote = "a repair overwrites by key and keeps no earlier value"
)

// glossary is the list of Decision 12, plus the repair values of a cell.
func glossary() []GlossaryRow {
	return []GlossaryRow{
		{"ok", "The runner recorded the key as stored. It is not a count of data rows."},
		{"no_value", "The source answered that it has nothing for the key."},
		{"expired", "The runner closed the key at its window after a failed, partial or crashed run: a real gap, not an answer."},
		{"missing", "The runner expected the key and it is not there."},
		{"not_run", "The runner recorded no plan for that day."},
		{"none", "The cell offers no repair."},
		{"button", "The cell can be repaired by one run of its job; the button names the runs it starts."},
		{"not refetchable", "There is no button: the text says lost when the source gives only the present, and not planned when repair of that day is not built."},
		{"ledger", "The runner's record of the keys it expected and the keys it stored, per session day."},
		{"session day", "A day on which the exchange calendar of the dataset is open."},
	}
}
