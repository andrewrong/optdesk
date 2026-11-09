package optionledger

// The SQL of the store. Nothing here updates or deletes: the role has INSERT and SELECT only.

const tradeColumns = `alias, level_of_detail, transaction_id, revision, attributes, attributes_sha256, source, recorded_at, fetch_id, row_index`

const fetchColumns = `login, query_id, first_date, last_date, started_at, finished_at, status, error_kind, error_code, ` +
	`raw_sha256, parser_version, statements, statements_unmapped, rows_parsed, rows_new, rows_revised, rows_unchanged, fetch_id`

const (
	// latestRevisionSQL returns the highest stored revision of one transaction and its hash.
	latestRevisionSQL = `SELECT revision, attributes_sha256 FROM optionledger.flex_trade_row ` +
		`WHERE alias = $1 AND level_of_detail = $2 AND transaction_id = $3 ORDER BY revision DESC LIMIT 1`

	// allRevisionsSQL returns every stored revision of one transaction and its hash.
	allRevisionsSQL = `SELECT revision, attributes_sha256 FROM optionledger.flex_trade_row ` +
		`WHERE alias = $1 AND level_of_detail = $2 AND transaction_id = $3`

	// insertRowSQL: the second guard against a duplicate key; the revision is chosen in the same transaction.
	// $8 is the recording time, NULL for now(); $9 is the fetch id and $10 the position of the row in its report.
	insertRowSQL = `INSERT INTO optionledger.flex_trade_row (` + tradeColumns + `) ` +
		`VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8, now()), $9, $10) ON CONFLICT DO NOTHING`

	insertFetchSQL = `INSERT INTO optionledger.flex_fetch (` + fetchColumns + `) ` +
		`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18) ON CONFLICT DO NOTHING`

	// selectRowsSQL reads the view: the highest revision of every transaction of the alias.
	selectRowsSQL = `SELECT DISTINCT ON (transaction_id, level_of_detail) ` + tradeColumns +
		` FROM rd.flex_trade_row_r1 WHERE alias = $1 ORDER BY transaction_id, level_of_detail, revision DESC`

	selectLatestOkSQL = `SELECT ` + fetchColumns + ` FROM rd.flex_fetch_r1 ` +
		`WHERE login = $1 AND query_id = $2 AND status = 'ok' ORDER BY finished_at DESC LIMIT 1`

	selectAliasesSQL = `SELECT DISTINCT alias FROM rd.flex_trade_row_r1 ORDER BY alias`
)
