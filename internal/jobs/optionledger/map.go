package optionledger

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

// errBadRow means a trade row has no transactionID or levelOfDetail; the window fails as a whole.
var errBadRow = errors.New("optionledger: trade row without transactionID or levelOfDetail")

// mapStatement maps the account of a statement to its alias through accounts and turns its trade rows into
// store rows carrying that alias. ok is false for an unmapped account: the caller counts the statement and drops it; the number is
// not returned in any error. accountId and acctAlias are removed from the attributes before they are encoded
// and hashed (AttributesSHA256 removes them too, the hash is over the same remaining attributes).
func mapStatement(accounts map[string]string, st sources.FlexStatement) (rows []store.FlexTradeRow, ok bool, err error) {
	alias, ok := accounts[st.AccountID]
	if !ok || alias == "" {
		return nil, false, nil
	}
	rows = make([]store.FlexTradeRow, 0, len(st.Trades))
	for _, t := range st.Trades {
		attrs := make(map[string]string, len(t))
		for k, v := range t {
			if k != "accountId" && k != "acctAlias" {
				attrs[k] = v
			}
		}
		if attrs["transactionID"] == "" || attrs["levelOfDetail"] == "" {
			return nil, false, errBadRow
		}
		sum, err := AttributesSHA256(attrs)
		if err != nil {
			return nil, false, err
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(attrs); err != nil { // keys come out sorted
			return nil, false, err
		}
		rows = append(rows, store.FlexTradeRow{
			Alias:            alias,
			LevelOfDetail:    attrs["levelOfDetail"],
			TransactionID:    attrs["transactionID"],
			Attributes:       string(bytes.TrimRight(buf.Bytes(), "\n")),
			AttributesSHA256: sum,
			Source:           "flex",
		})
	}
	return rows, true, nil
}
