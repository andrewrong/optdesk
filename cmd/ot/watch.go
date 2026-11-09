package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/jobs"
)

// watchFileName is the owner's watch list in $OT_HOME (docs/design/event-sources.md Decision 1). This file is the
// one parser of it in cmd/ot; docs/contracts/ot.md defines the format.
const watchFileName = "watch.toml"

var (
	cikIssuer    = regexp.MustCompile(`^cik:[0-9]{10}$`)
	edinetIssuer = regexp.MustCompile(`^edinet:[A-Za-z0-9]+$`)
	priceText    = regexp.MustCompile(`^[0-9]+(\.[0-9]{1,4})?$`)
)

// watchRow is one [[underlying]]. The three owner fields are decoded as any so that a wrong type (a bare TOML
// date or number) is a refusal that names the issuer, not a decoder text.
type watchRow struct {
	Issuer             string `toml:"issuer"`
	Market             string `toml:"market"`
	Symbol             string `toml:"symbol"`
	SecType            string `toml:"sec_type"`
	Exchange           string `toml:"exchange"`
	Currency           string `toml:"currency"`
	ConID              int64  `toml:"con_id"`
	ReportDateOverride any    `toml:"report_date_override"`
	BuyPrice           any    `toml:"buy_price"`
	SalePrice          any    `toml:"sale_price"`
}

type watchFile struct {
	Underlying []watchRow `toml:"underlying"`
}

// loadWatch reads $OT_HOME/watch.toml through deps.readFile (os.ReadFile when nil). A missing file is an empty
// list. Errors name an issuer or the file name, never a path or a value other than the issuer.
func loadWatch(a *app) ([]jobs.WatchEntry, error) {
	home := a.getenv("OT_HOME")
	if home == "" {
		return nil, errors.New("OT_HOME is not set")
	}
	read := a.deps.readFile
	if read == nil {
		read = os.ReadFile
	}
	data, err := read(filepath.Join(home, watchFileName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s cannot be read", watchFileName)
	}
	return parseWatch(data)
}

// parseWatch checks every entry and returns them in file order.
func parseWatch(data []byte) ([]jobs.WatchEntry, error) {
	var f watchFile
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s is not valid: bad TOML, an unknown key or a value of the wrong type", watchFileName)
	}
	seen := map[string]bool{}
	out := make([]jobs.WatchEntry, 0, len(f.Underlying))
	for i, r := range f.Underlying {
		if !cikIssuer.MatchString(r.Issuer) && !edinetIssuer.MatchString(r.Issuer) {
			return nil, fmt.Errorf("%s: entry %d: issuer %q must be cik:<10 digits> or edinet:<code>", watchFileName, i+1, r.Issuer)
		}
		bad := func(format string, args ...any) error {
			return fmt.Errorf("%s: issuer %s: %s", watchFileName, r.Issuer, fmt.Sprintf(format, args...))
		}
		if seen[r.Issuer] {
			return nil, bad("listed twice")
		}
		seen[r.Issuer] = true
		if r.Market != "US" && r.Market != "JP" {
			return nil, bad("market must be US or JP")
		}
		override, err := textField(r.ReportDateOverride)
		if err != nil {
			return nil, bad("report_date_override must be quoted text YYYY-MM-DD")
		}
		if override != "" {
			if t, err := time.Parse("2006-01-02", override); err != nil || t.Format("2006-01-02") != override {
				return nil, bad("report_date_override must be YYYY-MM-DD")
			}
		}
		buy, sale, err := prices(r)
		if err != nil {
			return nil, bad("%s", err.Error())
		}
		out = append(out, jobs.WatchEntry{
			Issuer: r.Issuer, Market: r.Market, ReportDateOverride: override, BuyPrice: buy, SalePrice: sale,
			Contract: broker.Contract{
				SecType: r.SecType, Symbol: r.Symbol, Exchange: r.Exchange, Currency: r.Currency, ConID: r.ConID,
			},
		})
	}
	return out, nil
}

// textField is the string value of an optional field; absent is empty and any other type is an error.
func textField(v any) (string, error) {
	switch s := v.(type) {
	case nil:
		return "", nil
	case string:
		return s, nil
	}
	return "", errors.New("not text")
}

// prices checks buy_price and sale_price: positive decimal text with at most 4 places, empty is unset, and a
// sale price is not below the buy price of the same entry.
func prices(r watchRow) (buy, sale string, err error) {
	var rats [2]*big.Rat
	for i, f := range []struct {
		name string
		v    any
	}{{"buy_price", r.BuyPrice}, {"sale_price", r.SalePrice}} {
		s, terr := textField(f.v)
		if terr != nil {
			return "", "", fmt.Errorf("%s must be quoted decimal text", f.name)
		}
		if s == "" {
			continue
		}
		rat, ok := new(big.Rat).SetString(s)
		if !priceText.MatchString(s) || !ok || rat.Sign() <= 0 {
			return "", "", fmt.Errorf("%s must be a positive decimal with at most 4 places", f.name)
		}
		rats[i] = rat
		if i == 0 {
			buy = s
		} else {
			sale = s
		}
	}
	if rats[0] != nil && rats[1] != nil && rats[1].Cmp(rats[0]) < 0 {
		return "", "", errors.New("sale_price is below buy_price")
	}
	return buy, sale, nil
}
