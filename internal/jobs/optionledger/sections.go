package optionledger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store/flexsection"
)

const (
	statePresent = "present"
	stateAbsent  = "absent"
)

// fetchRef mints the fetch reference of the section rows (design Decision 2): the login, the query, the
// window of the request and the fetch instant of the report in UTC, joined with ":".
func fetchRef(login string, req sources.FlexRequest, fetchedAt time.Time) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s", login, req.QueryID, req.First, req.Last, fetchedAt.UTC().Format(time.RFC3339))
}

// buildSectionRows turns the rows of one section of one statement into store rows of the alias. The account
// attributes accountId and acctAlias are removed before the row is encoded and hashed with hash. A row
// without its required key attributes is refused and counted, and so are both rows of a duplicate key: a
// key is never guessed, merged or overwritten.
func buildSectionRows(
	spec sectionSpec, alias string, rows []map[string]string, hash FlexAttributesSHA256Func, at time.Time,
) (out []flexsection.Row, refused int, err error) {
	keys := make([]string, len(rows)) // "" marks a refused row
	count := map[string]int{}
	for i, r := range rows {
		k, kerr := rowKey(spec, r)
		if kerr != nil {
			refused++
			continue
		}
		keys[i] = k
		count[k]++
	}
	for i, r := range rows {
		k := keys[i]
		if k == "" {
			continue
		}
		if count[k] > 1 {
			refused++
			continue
		}
		attrs := make(map[string]string, len(r))
		for name, v := range r {
			if name != "accountId" && name != "acctAlias" {
				attrs[name] = v
			}
		}
		sum, herr := hash(attrs)
		if herr != nil {
			return nil, 0, herr
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if eerr := enc.Encode(attrs); eerr != nil { // keys come out sorted
			return nil, 0, eerr
		}
		out = append(out, flexsection.Row{
			Alias: alias, Section: spec.Element, RowKey: k, RowIndex: i,
			Attributes:       string(bytes.TrimRight(buf.Bytes(), "\n")),
			AttributesSHA256: sum, RecordedAt: at,
		})
	}
	return out, refused, nil
}

// recordSections writes the section rows and one status row per expected section for every mapped statement
// of the report, with one call of RecordSections. A statement of an unmapped account writes nothing. An
// absent section also gives one degraded log record naming the login and the section.
func (r *run) recordSections(ctx context.Context, l FlexRun, req sources.FlexRequest, rep sources.FlexReport) error {
	ref := fetchRef(l.Name, req, rep.FetchedAt)
	at := r.j.clk.Now()
	var rows []flexsection.Row
	var statuses []flexsection.Status
	for _, st := range rep.Statements {
		alias, ok := r.j.cfg.Accounts[st.AccountID]
		if !ok || alias == "" {
			continue
		}
		for _, spec := range registry {
			srcRows := st.Sections[spec.Element]
			built, refused, err := buildSectionRows(spec, alias, srcRows, AttributesSHA256, at)
			if err != nil {
				return err
			}
			state := statePresent
			if len(srcRows) == 0 {
				state = stateAbsent
				r.j.log.Warn("optionledger: degraded", "login", l.Name, "section", spec.Element)
			}
			rows = append(rows, built...)
			statuses = append(statuses, flexsection.Status{
				FetchID: ref, Alias: alias, Section: spec.Element, State: state,
				RowCount: len(built), RowsRefused: refused, RecordedAt: at,
			})
		}
	}
	if len(statuses) == 0 {
		return nil
	}
	for i := range rows {
		rows[i].FetchID = ref
	}
	_, err := r.j.stores.Sections.RecordSections(ctx, ref, rows, statuses)
	return err
}
