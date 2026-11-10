package cutover

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

// importRunKey is the run key of an imported capture: its id is <alias>:<session_day>:import.
const importRunKey = "import"

const dayLayout = "2006-01-02"

var (
	_ ImportAccountFunc = ImportAccount
	_ VerifyAccountFunc = VerifyAccount
)

// accountLine is one line of account.jsonl: the last snapshot of one alias and session day. Decimals are text.
type accountLine struct {
	Alias      string         `json:"alias"`
	SessionDay string         `json:"session_day"`
	At         time.Time      `json:"at"`
	Status     string         `json:"status"`
	Values     []lineValue    `json:"values"`
	Positions  []linePosition `json:"positions"`
}

type lineValue struct {
	Tag      string `json:"tag"`
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

type linePosition struct {
	Symbol     string `json:"symbol"`
	SecType    string `json:"sec_type"`
	Exchange   string `json:"exchange"`
	Currency   string `json:"currency"`
	Expiry     string `json:"expiry"`
	Strike     string `json:"strike"`
	Multiplier string `json:"multiplier"`
	Right      string `json:"right"`
	ConID      int64  `json:"con_id"`
	Quantity   string `json:"quantity"`
	AvgCost    string `json:"avg_cost"`
}

// entry is a validated line: the capture to store and the hash of its positions and values.
type entry struct {
	capture store.Capture
	hash    string
	// empty is true for a line with no positions and no values. The reader cannot tell a stored capture with
	// no rows from no capture, so such a line is equal to "nothing stored": Same on import, not Missing.
	empty bool
}

// loadAccount reads and validates the whole file before anything is stored or written: a line after through,
// an alias the map does not name, a line that is not an account line and a second line of one alias and day
// refuse the file. Errors carry the line number and a fixed text, never the content of the line.
func loadAccount(r io.Reader, aliasMap map[string]string, through contracts.Day) ([]entry, error) {
	if _, err := time.Parse(dayLayout, string(through)); err != nil {
		return nil, errors.New("through is not a day")
	}
	var out []entry
	seen := map[string]bool{}
	err := ReadLines(r, func(_ int, raw []byte) error {
		var l accountLine
		if err := json.Unmarshal(raw, &l); err != nil {
			return errors.New("not an account line")
		}
		if _, err := time.Parse(dayLayout, l.SessionDay); err != nil {
			return errors.New("session_day is not a day")
		}
		if contracts.Day(l.SessionDay) > through {
			return errors.New("session_day is after through")
		}
		alias, err := mapAlias(aliasMap, l.Alias)
		if err != nil {
			return err
		}
		if alias == "" {
			return errors.New("alias is empty")
		}
		key := alias + ":" + l.SessionDay
		if seen[key] {
			return errors.New("second line of one alias and day")
		}
		seen[key] = true
		c := store.Capture{
			ID: key + ":" + importRunKey, Alias: alias, At: l.At, SessionDay: l.SessionDay, Status: l.Status,
		}
		for _, v := range l.Values {
			c.Values = append(c.Values, store.AccountValue{
				CaptureID: c.ID, Alias: alias, CapturedAt: l.At, SessionDay: l.SessionDay,
				Tag: v.Tag, Value: v.Value, Currency: v.Currency,
			})
		}
		for _, p := range l.Positions {
			c.Positions = append(c.Positions, store.Position{
				CaptureID: c.ID, Alias: alias, CapturedAt: l.At, SessionDay: l.SessionDay,
				Symbol: p.Symbol, SecType: p.SecType, Exchange: p.Exchange, Currency: p.Currency,
				Expiry: p.Expiry, Strike: p.Strike, Multiplier: p.Multiplier, Right: p.Right,
				ConID: p.ConID, Quantity: p.Quantity, AvgCost: p.AvgCost,
			})
		}
		h, err := contentHash(c.Positions, c.Values)
		if err != nil {
			return err
		}
		out = append(out, entry{capture: c, hash: h, empty: len(c.Positions) == 0 && len(c.Values) == 0})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// contentHash is the SHA-256 of the canonical text of the positions and values of one alias and day, without
// the capture id, the alias and the instants, rows sorted by their own canonical text. The file side and the
// stored side both go through it, so an equal content gives an equal hash.
func contentHash(positions []store.Position, values []store.AccountValue) (string, error) {
	ps := make([]linePosition, 0, len(positions))
	for _, p := range positions {
		ps = append(ps, linePosition{
			Symbol: p.Symbol, SecType: p.SecType, Exchange: p.Exchange, Currency: p.Currency,
			Expiry: p.Expiry, Strike: p.Strike, Multiplier: p.Multiplier, Right: p.Right,
			ConID: p.ConID, Quantity: p.Quantity, AvgCost: p.AvgCost,
		})
	}
	vs := make([]lineValue, 0, len(values))
	for _, v := range values {
		vs = append(vs, lineValue{Tag: v.Tag, Value: v.Value, Currency: v.Currency})
	}
	if err := sortByText(ps); err != nil {
		return "", err
	}
	if err := sortByText(vs); err != nil {
		return "", err
	}
	raw, err := json.Marshal(struct {
		Positions []linePosition `json:"positions"`
		Values    []lineValue    `json:"values"`
	}{ps, vs})
	if err != nil {
		return "", errors.New("cannot encode the rows")
	}
	return CanonicalHash(raw)
}

func sortByText[T any](rows []T) error {
	type kv struct {
		key string
		row T
	}
	items := make([]kv, len(rows))
	for i, r := range rows {
		b, err := json.Marshal(r)
		if err != nil {
			return errors.New("cannot encode a row")
		}
		items[i] = kv{string(b), r}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].key < items[j].key })
	for i := range items {
		rows[i] = items[i].row
	}
	return nil
}

// storedHash reads the stored positions and values of alias and day and hashes them; found is false when
// nothing is stored.
func storedHash(ctx context.Context, rd store.AccountReader, alias, day string) (hash string, found bool, err error) {
	ps, err := rd.Positions(ctx, alias, day)
	if err != nil {
		return "", false, errors.New("reading stored positions failed")
	}
	vs, err := rd.Values(ctx, alias, day)
	if err != nil {
		return "", false, errors.New("reading stored values failed")
	}
	if len(ps) == 0 && len(vs) == 0 {
		return "", false, nil
	}
	h, err := contentHash(ps, vs)
	return h, true, err
}

// ImportAccount imports account.jsonl: one store.Capture per line with the id <alias>:<session_day>:import.
// The whole file is checked before any write. For each line the stored capture of the alias and day is read
// back first (WriteCapture returns no error for an id that exists): none stored is Inserted and written, an
// equal hash is Same and an other hash is Conflict, neither written. It implements ImportAccountFunc.
func ImportAccount(ctx context.Context, r io.Reader, st store.AccountStore, aliasMap map[string]string, through contracts.Day) (jobs.ImportResult, error) {
	entries, err := loadAccount(r, aliasMap, through)
	if err != nil {
		return jobs.ImportResult{}, err
	}
	res := jobs.ImportResult{Lines: len(entries)}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		h, found, err := storedHash(ctx, st, e.capture.Alias, e.capture.SessionDay)
		if err != nil {
			return res, err
		}
		switch {
		case !found && e.empty:
			res.Same++
		case !found:
			if err := st.WriteCapture(ctx, e.capture); err != nil {
				return res, errors.New("storing a capture failed")
			}
			res.Inserted++
		case h == e.hash:
			res.Same++
		default:
			res.Conflict++
		}
	}
	return res, nil
}
