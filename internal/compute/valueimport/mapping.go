package valueimport

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valueimportapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

var keyPat = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:._-]{0,63}$`)

var suffixes = []string{".T", ".HK", ".SZ", ".SS"}

// cleanTicker upper-cases, cuts an exchange suffix and, for a suffix-free ticker, turns the dot into a space.
func cleanTicker(t string) string {
	t = strings.ToUpper(strings.TrimSpace(t))
	for _, s := range suffixes {
		if strings.HasSuffix(t, s) {
			return strings.TrimSuffix(t, s)
		}
	}
	return strings.ReplaceAll(t, ".", " ")
}

// listMarket maps a settings.underlying market to its R market column.
func listMarket(m string) (string, bool) {
	switch strings.ToUpper(m) {
	case "US":
		return valuelinesapi.MarketUS, true
	case "JP":
		return valuelinesapi.MarketJP, true
	}
	return "", false
}

// fileRMarket is the default R market of a file market.
func fileRMarket(m string) string {
	switch m {
	case "US":
		return valuelinesapi.MarketUS
	case "JP":
		return valuelinesapi.MarketJP
	}
	return valuelinesapi.MarketHKCN
}

// match finds the list row of a ticker; two matches are settled by the file's market.
func match(under []valueimportapi.StoredUnderlying, ticker, fileMarket string) (valueimportapi.StoredUnderlying, bool) {
	sym := cleanTicker(ticker)
	var found []valueimportapi.StoredUnderlying
	for _, u := range under {
		if strings.ToUpper(strings.TrimSpace(u.Symbol)) == sym {
			found = append(found, u)
		}
	}
	if len(found) == 1 {
		return found[0], true
	}
	var same []valueimportapi.StoredUnderlying
	for _, u := range found {
		if strings.EqualFold(u.Market, fileMarket) {
			same = append(same, u)
		}
	}
	if len(same) == 1 {
		return same[0], true
	}
	return valueimportapi.StoredUnderlying{}, false
}

// proposedKey is the file market as prefix and the cleaned symbol.
func proposedKey(market, ticker string) string { return market + ":" + cleanTicker(ticker) }

// checkPick validates the owner's key and R market for an unmatched name.
func checkPick(p valueimportapi.Pick, under []valueimportapi.StoredUnderlying) bool {
	if !keyPat.MatchString(p.Key) {
		return false
	}
	for _, u := range under {
		if u.Key == p.Key {
			return false
		}
	}
	switch p.RMarket {
	case valuelinesapi.MarketUS, valuelinesapi.MarketJP, valuelinesapi.MarketHKCN:
		return true
	}
	return false
}

// pFrom reads a P range text of params_by_P.
func pFrom(key string) (string, bool) {
	k := strings.TrimSpace(key)
	if strings.HasPrefix(k, "<=") {
		if _, err := decimal.NewFromString(strings.TrimSpace(k[2:])); err != nil {
			return "", false
		}
		return "0", true
	}
	k = strings.TrimPrefix(k, ">=")
	if i := strings.Index(k, "-"); i > 0 {
		k = k[:i]
	}
	d, err := decimal.NewFromString(strings.TrimSpace(k))
	if err != nil {
		return "", false
	}
	return d.StringFixed(1), true
}

// marginRows maps params_by_P to the margin rows, ascending by p_from; a key that cannot be read
// or two keys with one p_from are a bad file.
func marginRows(params []paramRow) ([]valuelinesapi.MarginRow, error) {
	seen := map[string]bool{}
	var rows []valuelinesapi.MarginRow
	for _, p := range params {
		from, ok := pFrom(p.key)
		if !ok || seen[from] {
			return nil, bad("meta.params_by_P." + p.key)
		}
		seen[from] = true
		rows = append(rows, valuelinesapi.MarginRow{PFrom: from, M: p.nums[0], W: p.nums[1], H: p.nums[2], S: p.nums[3]})
	}
	sort.Slice(rows, func(i, j int) bool {
		return decimal.RequireFromString(rows[i].PFrom).LessThan(decimal.RequireFromString(rows[j].PFrom))
	})
	return rows, nil
}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// reviewID is imp-<12 hex of the file>-<8 hex of the key>.
func reviewID(fileSHA, key string) string {
	return "imp-" + fileSHA[:12] + "-" + sha([]byte(key))[:8]
}

// changeMemo is the labelled lines note: then q_note:, an empty one left out.
func changeMemo(note, qNote string) string {
	var lines []string
	if t := strings.TrimSpace(note); t != "" {
		lines = append(lines, "note: "+t)
	}
	if t := strings.TrimSpace(qNote); t != "" {
		lines = append(lines, "q_note: "+t)
	}
	return strings.Join(lines, "\n")
}

// symbolFor is the profile symbol: the cleaned symbol for US and JP, empty for HKCN.
func symbolFor(rMarket, ticker string) string {
	if rMarket == valuelinesapi.MarketHKCN {
		return ""
	}
	return cleanTicker(ticker)
}
