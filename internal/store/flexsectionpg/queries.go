package flexsectionpg

// The SQL of the store. It inserts and selects only: the role holds INSERT and SELECT on the two tables and
// SELECT on the two views. Every read goes through a view.

const (
	rowColumns    = `alias, section, row_key, revision, fetch_id, row_index, attributes, attributes_sha256, recorded_at`
	statusColumns = `fetch_id, alias, section, state, row_count, rows_refused, recorded_at`
)

const (
	// latestRevisionSQL returns the highest stored revision of one key and its hash.
	latestRevisionSQL = `SELECT revision, attributes_sha256 FROM rd.flex_section_row_r1 ` +
		`WHERE alias = $1 AND section = $2 AND row_key = $3 ORDER BY revision DESC LIMIT 1`

	// insertRowSQL is the second guard against a racer that wrote the same revision first. $9 is the recording
	// time, NULL for now().
	insertRowSQL = `INSERT INTO optionledger.flex_section_row (` + rowColumns + `) ` +
		`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9, now())) ` +
		`ON CONFLICT (alias, section, row_key, revision) DO NOTHING`

	// insertStatusSQL: $7 is the recording time, NULL for now().
	insertStatusSQL = `INSERT INTO optionledger.flex_section_status (` + statusColumns + `) ` +
		`VALUES ($1, $2, $3, $4, $5, $6, COALESCE($7, now())) ` +
		`ON CONFLICT (fetch_id, alias, section) DO NOTHING`

	// selectRowsSQL reads the view: the highest revision of every key of the section of the alias.
	selectRowsSQL = `SELECT DISTINCT ON (row_key) ` + rowColumns + ` FROM rd.flex_section_row_r1 ` +
		`WHERE alias = $1 AND section = $2 ORDER BY row_key, revision DESC`

	// selectStatusesSQL reads the view: the newest status of every section of the alias.
	selectStatusesSQL = `SELECT DISTINCT ON (section) ` + statusColumns + ` FROM rd.flex_section_status_r1 ` +
		`WHERE alias = $1 ORDER BY section, recorded_at DESC, fetch_id DESC`
)
