package optionledger

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

// The five statuses of an imported file, and the reason codes of a failed one (docs/design/flex-xml-import.md
// Decision 3).
const (
	ImportStored       = "stored"
	ImportPresent      = "present"
	ImportNothingFound = "nothing_found"
	ImportUnconfigured = "unconfigured"
	ImportFailed       = "failed"

	ReasonNotReport = "not_report"
	ReasonStoreErr  = "store_error"
)

// FlexFileRef names one file of one post: RequestID is 8 hex characters made once per post, Ordinal the
// position of the file in the post counted from 1.
type FlexFileRef struct {
	RequestID string
	Ordinal   int
}

// FlexAliasCount is what one alias of a report did: Statements that had trade rows, RowsParsed the mapped rows,
// RowsNew those added, RowsPresent those already stored or repeated inside the report.
type FlexAliasCount struct {
	Alias                                        string
	Statements, RowsParsed, RowsNew, RowsPresent int
}

// FlexImportSummary is the result of ImportFlexReport: the records written in order, with RowsNew as the
// store counted it, and one count per record (Alias "-" for a record without an alias). Unmapped is the
// number of statements of an account that is not in the map; no account number is held.
type FlexImportSummary struct {
	Records  []store.FlexFetchRecord
	Aliases  []FlexAliasCount
	Unmapped int
}

// ImportError is the error of ImportFlexReport; Code is ReasonNotReport or ReasonStoreErr and holds no value
// of the file.
type ImportError struct{ Code string }

func (e *ImportError) Error() string { return "optionledger flex import: " + e.Code }

// Kind is Permanent: the same file fails the same way.
func (e *ImportError) Kind() contracts.ErrorKind { return contracts.Permanent }

type aliasGroup struct {
	alias      string
	statements []sources.FlexStatement
	rows       []store.FlexTradeRow
}

// ImportFlexReport maps every statement of rep with mapStatement and groups the rows by alias. For each alias
// in sorted order it reads the stored rows of the alias with the reader and hands to one RecordFlexFetch only the rows whose key
// (level of detail, transaction id) is absent and has not appeared earlier in the report; the rest are counted
// as already present, so no revision of a stored key is written. Unmapped statements are counted into the
// record of the first alias, or into one `-` record without rows when no alias has rows; a report with no
// trade and no unmapped statement is one `-` record with all counts 0. A statement mapStatement refuses
// returns ImportError not_report before anything is written; a reader or writer error returns store_error.
func ImportFlexReport(
	ctx context.Context, rep sources.FlexReport, accounts map[string]string,
	w store.OptionLedgerWriter, r store.OptionLedgerReader, ref FlexFileRef, now time.Time,
) (FlexImportSummary, error) {
	var sum FlexImportSummary
	groups := map[string]*aliasGroup{}
	for _, st := range rep.Statements {
		rows, ok, err := mapStatement(accounts, st)
		if err != nil {
			return sum, &ImportError{Code: ReasonNotReport}
		}
		if !ok {
			sum.Unmapped++
			continue
		}
		if len(rows) == 0 {
			continue
		}
		g := groups[rows[0].Alias]
		if g == nil {
			g = &aliasGroup{alias: rows[0].Alias}
			groups[g.alias] = g
		}
		g.statements = append(g.statements, st)
		g.rows = append(g.rows, rows...)
	}
	if len(groups) == 0 {
		rec := importRecord(ref, "", rep.RawSHA256, nil, nil, now)
		rec.Statements = sum.Unmapped
		rec.UnmappedStatement = sum.Unmapped
		return write(ctx, w, sum, rec, nil, 0)
	}
	aliases := make([]string, 0, len(groups))
	for a := range groups {
		aliases = append(aliases, a)
	}
	sort.Strings(aliases)
	for i, a := range aliases {
		g := groups[a]
		stored, err := r.FlexTradeRows(ctx, a)
		if err != nil {
			return sum, &ImportError{Code: ReasonStoreErr}
		}
		seen := make(map[[2]string]bool, len(stored)+len(g.rows))
		for _, s := range stored {
			seen[[2]string{s.LevelOfDetail, s.TransactionID}] = true
		}
		var fresh []store.FlexTradeRow
		for _, row := range g.rows {
			k := [2]string{row.LevelOfDetail, row.TransactionID}
			if !seen[k] {
				seen[k] = true
				fresh = append(fresh, row)
			}
		}
		rec := importRecord(ref, a, rep.RawSHA256, g.statements, g.rows, now)
		rec.Statements = len(g.statements)
		rec.RowsParsed = len(g.rows)
		if i == 0 {
			rec.UnmappedStatement = sum.Unmapped
		}
		var err2 error
		if sum, err2 = write(ctx, w, sum, rec, fresh, len(g.rows)); err2 != nil {
			return sum, err2
		}
	}
	return sum, nil
}

func write(
	ctx context.Context, w store.OptionLedgerWriter, sum FlexImportSummary, rec store.FlexFetchRecord, fresh []store.FlexTradeRow, parsed int,
) (FlexImportSummary, error) {
	ins, err := w.RecordFlexFetch(ctx, rec, fresh)
	if err != nil {
		return sum, &ImportError{Code: ReasonStoreErr}
	}
	rec.RowsNew = ins.New
	sum.Records = append(sum.Records, rec)
	alias := ImportAlias(rec.QueryID)
	if alias == "" {
		alias = "-"
	}
	sum.Aliases = append(sum.Aliases, FlexAliasCount{
		Alias: alias, Statements: rec.Statements, RowsParsed: parsed, RowsNew: ins.New, RowsPresent: parsed - ins.New,
	})
	return sum, nil
}

// RecordImportFailure writes one failed record for the file ref with ErrorCode code, alias part `-`, no rows
// and no value of the file; its days are the day of now.
func RecordImportFailure(ctx context.Context, w store.OptionLedgerWriter, ref FlexFileRef, code string, now time.Time) error {
	rec := importRecord(ref, "", "", nil, nil, now)
	rec.Status = statusFailed
	rec.ErrorKind = "import"
	rec.ErrorCode = code
	if _, err := w.RecordFlexFetch(ctx, rec, nil); err != nil {
		return &ImportError{Code: ReasonStoreErr}
	}
	return nil
}

// importRecord is the one builder of every record of the import. alias "" gives the part `-`. The days are
// the period of sts when all of them read, else the trade dates of rows, else the day of now in UTC.
func importRecord(ref FlexFileRef, alias, rawSHA string, sts []sources.FlexStatement, rows []store.FlexTradeRow, now time.Time) store.FlexFetchRecord {
	part := alias
	if part == "" {
		part = "-"
	}
	first, last, ok := statementPeriod(sts)
	if !ok {
		first, last, ok = tradeDatePeriod(rows)
	}
	if !ok {
		d := contracts.Day(now.UTC().Format(dayLayout))
		first, last = d, d
	}
	return store.FlexFetchRecord{
		Login:         "import",
		QueryID:       fmt.Sprintf("%s%s:%d:%s", "file:", ref.RequestID, ref.Ordinal, part),
		FirstDay:      first,
		LastDay:       last,
		StartedAt:     now,
		FinishedAt:    now,
		Status:        statusOK,
		RawSHA256:     rawSHA,
		ParserVersion: parserVersion,
	}
}

// digitDay reads a date by keeping its digits (yyyyMMdd or yyyy-MM-dd); ok is false unless they are a date.
func digitDay(s string) (contracts.Day, bool) {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	t, err := time.Parse("20060102", b.String())
	if err != nil || b.Len() != 8 {
		return "", false
	}
	return contracts.Day(t.Format(dayLayout)), true
}

func statementPeriod(sts []sources.FlexStatement) (first, last contracts.Day, ok bool) {
	if len(sts) == 0 {
		return "", "", false
	}
	for i, st := range sts {
		f, ok1 := digitDay(st.FromDate)
		l, ok2 := digitDay(st.ToDate)
		if !ok1 || !ok2 {
			return "", "", false
		}
		if i == 0 || f < first {
			first = f
		}
		if i == 0 || l > last {
			last = l
		}
	}
	return first, last, true
}

func tradeDatePeriod(rows []store.FlexTradeRow) (first, last contracts.Day, ok bool) {
	for _, row := range rows {
		var attrs map[string]string
		if err := decodeAttrs(row.Attributes, &attrs); err != nil {
			continue
		}
		d, good := digitDay(attrs["tradeDate"])
		if !good {
			continue
		}
		if !ok || d < first {
			first = d
		}
		if !ok || d > last {
			last = d
		}
		ok = true
	}
	return first, last, ok
}

func decodeAttrs(text string, into *map[string]string) error {
	return json.Unmarshal([]byte(text), into)
}

// DeriveImportStatus is the one derivation of the five statuses from a stored record: failed when Status is
// failed (reason is ErrorCode), stored when rows_new + rows_revised is above 0, present when rows_parsed is
// above 0, unconfigured when statements_unmapped is above 0 and rows_parsed is 0, else nothing_found.
func DeriveImportStatus(rec store.FlexFetchRecord) (status, reason string) {
	switch {
	case rec.Status == statusFailed:
		return ImportFailed, rec.ErrorCode
	case rec.RowsNew+rec.RowsRevised > 0:
		return ImportStored, ""
	case rec.RowsParsed > 0:
		return ImportPresent, ""
	case rec.UnmappedStatement > 0:
		return ImportUnconfigured, ""
	}
	return ImportNothingFound, ""
}

// ImportAlias is the last part of an import query id (split into at most four parts at ':'), empty for `-`.
func ImportAlias(queryID string) string {
	parts := strings.SplitN(queryID, ":", 4)
	if len(parts) < 4 || parts[3] == "-" {
		return ""
	}
	return parts[3]
}
