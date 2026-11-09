package optionledger

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

// sourceOTImport is the source tag of an imported row (docs/design/option-ledger.md Decision 5).
const sourceOTImport = "ot_import"

var _ jobs.ImportOptionTraderFunc = ImportOptionTrader

type importError struct{ msg string }

func (e *importError) Error() string             { return "optionledger import: " + e.msg }
func (e *importError) Kind() contracts.ErrorKind { return contracts.Permanent }

func refuse(format string, a ...any) error {
	return &importError{msg: fmt.Sprintf(format, a...)}
}

// exportLine is one line of the export of tools/ot-flex-export (Decision 7). exported attributes_sha256 and
// recorded_at are not read: the hash is recomputed and the recording instant is the caller's.
type exportLine struct {
	Alias         *string `json:"alias"`
	LevelOfDetail *string `json:"level_of_detail"`
	TransactionID *string `json:"transaction_id"`
	Revision      *int    `json:"revision"`
	Attributes    *string `json:"attributes"`
}

// ImportOptionTrader implements jobs.ImportOptionTraderFunc. It reads every line of r, maps the alias of each
// row with aliasMap (an alias with no entry refuses the file), recomputes attributes_sha256 with
// AttributesSHA256, and writes all rows with one ImportFlexTradeRows call, source ot_import. Blank lines are
// skipped. Nothing is written when any line is refused. A refused line is named by its number only, never its
// content; the result holds counts only.
func ImportOptionTrader(ctx context.Context, r io.Reader, w store.OptionLedgerWriter, aliasMap map[string]string, now time.Time) (jobs.ImportResult, error) {
	var res jobs.ImportResult
	if r == nil || w == nil {
		return res, refuse("reader and writer are required")
	}
	var rows []store.FlexTradeRow
	br := bufio.NewReader(r)
	for n := 1; ; n++ {
		line, err := br.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return res, refuse("read failed at line %d", n)
		}
		if t := bytes.TrimSpace(line); len(t) > 0 {
			row, perr := parseLine(t, aliasMap, now)
			if perr != nil {
				return res, refuse("line %d: %s", n, perr.Error())
			}
			rows = append(rows, row)
		}
		if err != nil {
			break
		}
	}
	res.Lines = len(rows)
	if len(rows) == 0 {
		return res, nil
	}
	out, err := w.ImportFlexTradeRows(ctx, rows)
	if err != nil {
		return jobs.ImportResult{Lines: res.Lines}, refuse("write failed")
	}
	res.Inserted, res.Same, res.Conflict = out.Inserted, out.Same, out.Conflict
	return res, nil
}

func parseLine(line []byte, aliasMap map[string]string, now time.Time) (store.FlexTradeRow, error) {
	var zero store.FlexTradeRow
	if line[0] != '{' {
		return zero, errors.New("not a JSON object")
	}
	var l exportLine
	if err := json.Unmarshal(line, &l); err != nil {
		return zero, errors.New("not a JSON object of the export fields")
	}
	if l.Alias == nil || l.LevelOfDetail == nil || l.TransactionID == nil || l.Revision == nil || l.Attributes == nil {
		return zero, errors.New("a field is missing")
	}
	if *l.LevelOfDetail == "" || *l.TransactionID == "" || *l.Revision < 1 {
		return zero, errors.New("a key field is empty or out of range")
	}
	text := *l.Attributes
	if len(text) == 0 || text[0] != '{' {
		return zero, errors.New("attributes is not a JSON object text")
	}
	var attrs map[string]string
	if err := json.Unmarshal([]byte(text), &attrs); err != nil || attrs == nil {
		return zero, errors.New("attributes is not a JSON object text of strings")
	}
	if _, ok := attrs["accountId"]; ok {
		return zero, errors.New("attributes holds an account key")
	}
	if _, ok := attrs["acctAlias"]; ok {
		return zero, errors.New("attributes holds an account key")
	}
	alias, ok := aliasMap[*l.Alias]
	if !ok || alias == "" {
		return zero, errors.New("alias is not mapped")
	}
	sum, err := AttributesSHA256(attrs)
	if err != nil {
		return zero, errors.New("attributes cannot be hashed")
	}
	return store.FlexTradeRow{
		Alias:            alias,
		LevelOfDetail:    *l.LevelOfDetail,
		TransactionID:    *l.TransactionID,
		Revision:         *l.Revision,
		Attributes:       text,
		AttributesSHA256: sum,
		Source:           sourceOTImport,
		RecordedAt:       now,
	}, nil
}
