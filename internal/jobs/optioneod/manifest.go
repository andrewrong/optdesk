package optioneod

import (
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"
	// The zone database is embedded so that a zone name loads on every machine.
	_ "time/tzdata"

	"github.com/pelletier/go-toml/v2"
)

const (
	defaultZone        = "America/New_York"
	defaultFreezeAfter = "1615"
	defaultOpenAfter   = 90
	defaultIncrement   = "0.01"
	defaultMaxExpiries = 3
	defaultBand        = 0.10
	defaultPerSide     = 8
	defaultChunk       = 50
	defaultMaxStrikes  = 24
	defaultBudget      = 8
)

type window struct {
	Rule             string `toml:"rule"`
	OpenAfterMinutes *int   `toml:"open_after_minutes"`
	Calendar         string `toml:"calendar"`
}

type underlying struct {
	Symbol         string `toml:"symbol"`
	SecType        string `toml:"sec_type"`
	Exchange       string `toml:"exchange"`
	Currency       string `toml:"currency"`
	ConID          int64  `toml:"con_id"`
	MIC            string `toml:"mic"`
	CalendarID     string `toml:"calendar_id"`
	PriceIncrement string `toml:"price_increment"`
	LotSize        string `toml:"lot_size"`
}

// stockTable is the [stock] table of job_jp.toml: what a JP stock underlying gets besides its watch entry (symbol,
// sec_type, exchange, currency, con_id): the instrument values, the option side and its own selection values.
type stockTable struct {
	MIC               string `toml:"mic"`
	CalendarID        string `toml:"calendar_id"`
	LotSize           string `toml:"lot_size"`
	PriceIncrement    string `toml:"price_increment"`
	ChainExchange     string `toml:"chain_exchange"`
	Multiplier        string `toml:"multiplier"`
	OptionMIC         string `toml:"option_mic"`
	MinDTE            int    `toml:"min_dte"`
	MaxDTE            int    `toml:"max_dte"`
	MaxStrikes        int    `toml:"max_strikes"`
	MaxStrikesPerSide int    `toml:"max_strikes_per_side"`
}

// indexRow is one [[index]] row of job_index.toml: the class table of an index underlying (Decision 9 of
// docs/design/option-capture-need.md). SpotDivisor divides the stored close that stands in for a missing spot.
type indexRow struct {
	Symbol         string `toml:"symbol"`
	TradingClass   string `toml:"trading_class"`
	StrikeMultiple int    `toml:"strike_multiple"`
	MaxContracts   int    `toml:"max_contracts"`
	ExpiryKind     string `toml:"expiry_kind"`
	SigmaIndex     string `toml:"sigma_index"`
	SpotIndex      string `toml:"spot_index"`
	SpotDivisor    int    `toml:"spot_divisor"`
}

const (
	expiryListed      = "listed"
	expiryThirdFriday = "third_friday"
)

type manifest struct {
	Version  int           `toml:"version"`
	ID       string        `toml:"id"`
	Zone     string        `toml:"zone"`
	Window   window        `toml:"window"`
	Schedule scheduleTable `toml:"schedule"`
	// Slot is the label "HHMM" of the run's slot, a manifest key only: it is in the capture id, never in the plan hash.
	Slot                 string  `toml:"slot"`
	MaxExpiries          int     `toml:"max_expiries"`
	StrikeBand           float64 `toml:"strike_band"`
	MaxStrikesPerSide    int     `toml:"max_strikes_per_side"`
	StrikeBandBelow      float64 `toml:"strike_band_below"`
	StrikeBandAbove      float64 `toml:"strike_band_above"`
	MinDTE               int     `toml:"min_dte"`
	MaxDTE               int     `toml:"max_dte"`
	MaxStrikes           int     `toml:"max_strikes"`
	Chunk                int     `toml:"chunk"`
	BudgetMinutes        int     `toml:"budget_minutes"`
	FreezeAfter          string  `toml:"freeze_after"`
	OptionPriceIncrement string  `toml:"option_price_increment"`
	// TradingClass, when set, makes the chain choice take the chain of that trading class on the underlying's exchange.
	TradingClass string `toml:"trading_class"`
	// BuyLow and BuyHigh, both or neither, are the multiples of a BuyPrice that bound the strikes of an entry with
	// a BuyPrice (decimal text, low below high). Without them such an entry keeps the band only.
	BuyLow  string `toml:"buy_low"`
	BuyHigh string `toml:"buy_high"`
	// SigmaDays, SigmaFallback and BandK are the index band keys (all trial); they apply to the [[index]] rows.
	SigmaDays     int     `toml:"sigma_days"`
	SigmaFallback float64 `toml:"sigma_fallback"`
	BandK         float64 `toml:"band_k"`
	// LedgerDataset is the dataset id of the plan keys; default obs.option_eod.
	LedgerDataset string       `toml:"ledger_dataset"`
	Underlying    []underlying `toml:"underlying"`
	Index         []indexRow   `toml:"index"`
	// Stock is the [stock] table; it applies to the stock underlyings a watch constructor adds (see isStock).
	Stock stockTable `toml:"stock"`

	sched schedule
	// buy maps a symbol to the BuyPrice text of its watch entry; empty for a manifest without a watch list.
	buy map[string]string
}

// parseManifest decodes data and gives every missing optional key its default.
func parseManifest(data []byte) (manifest, error) {
	var m manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("optioneod: manifest: %w", err)
	}
	if m.ID == "" || len(m.Underlying) == 0 {
		return manifest{}, fmt.Errorf("optioneod: manifest needs an id and an underlying")
	}
	m.applyDefaults()
	if err := m.validate(); err != nil {
		return manifest{}, err
	}
	sched, err := scheduleOf(m.Schedule)
	if err != nil {
		return manifest{}, err
	}
	m.sched = sched
	// An index manifest keeps the order of its file: the last key is the one a spent budget cuts.
	if len(m.Index) == 0 {
		sort.Slice(m.Underlying, func(i, j int) bool { return m.Underlying[i].Symbol < m.Underlying[j].Symbol })
	}
	return m, nil
}

func (m *manifest) applyDefaults() {
	if m.Zone == "" {
		m.Zone = defaultZone
	}
	if m.Window.Rule == "" {
		m.Window.Rule = "in_session"
	}
	if m.Window.OpenAfterMinutes == nil {
		v := defaultOpenAfter
		m.Window.OpenAfterMinutes = &v
	}
	if m.MaxExpiries == 0 {
		m.MaxExpiries = defaultMaxExpiries
	}
	if m.StrikeBand == 0 {
		m.StrikeBand = defaultBand
	}
	if m.MaxStrikesPerSide == 0 {
		m.MaxStrikesPerSide = defaultPerSide
	}
	if m.Chunk == 0 {
		m.Chunk = defaultChunk
	}
	if m.BudgetMinutes == 0 {
		m.BudgetMinutes = defaultBudget
	}
	if m.LedgerDataset == "" {
		m.LedgerDataset = datasetID
	}
	if m.FreezeAfter == "" {
		m.FreezeAfter = defaultFreezeAfter
	}
	if m.OptionPriceIncrement == "" {
		m.OptionPriceIncrement = defaultIncrement
	}
}

func (m manifest) validate() error {
	if m.MaxExpiries < 0 || m.MaxStrikesPerSide < 0 || m.Chunk < 0 || m.BudgetMinutes < 0 || m.StrikeBand < 0 || m.StrikeBandBelow < 0 || m.StrikeBandAbove < 0 {
		return fmt.Errorf("optioneod: manifest: counts and the band must not be negative")
	}
	if m.MinDTE < 0 || m.MaxDTE < 0 || m.MaxStrikes < 0 || (m.MaxDTE > 0 && m.MinDTE > m.MaxDTE) {
		return fmt.Errorf("optioneod: manifest: min_dte, max_dte and max_strikes must not be negative and min_dte must not exceed max_dte")
	}
	if err := m.validateBuyKeys(); err != nil {
		return err
	}
	if _, err := time.LoadLocation(m.Zone); err != nil {
		return fmt.Errorf("optioneod: manifest: zone is unknown")
	}
	if m.Slot != "" && !fourDigits(m.Slot) {
		return fmt.Errorf("optioneod: manifest: slot must be four digits")
	}
	if _, err := parseHHMM(m.FreezeAfter); err != nil {
		return fmt.Errorf("optioneod: manifest: freeze_after must be HHMM")
	}
	for _, u := range m.Underlying {
		if err := m.validateUnderlying(u); err != nil {
			return err
		}
	}
	return m.validateIndex()
}

// validateIndex checks the [[index]] rows and the band keys they use; a manifest without a row has nothing to check.
func (m manifest) validateIndex() error {
	if len(m.Index) == 0 {
		return nil
	}
	if !(m.BandK > 0) {
		return fmt.Errorf("optioneod: manifest: band_k must be above 0")
	}
	if !(m.SigmaFallback > 0) {
		return fmt.Errorf("optioneod: manifest: sigma_fallback must be above 0")
	}
	if m.SigmaDays < 2 {
		return fmt.Errorf("optioneod: manifest: sigma_days must be at least 2")
	}
	for _, r := range m.Index {
		has := false
		for _, u := range m.Underlying {
			has = has || u.Symbol == r.Symbol
		}
		switch {
		case !has:
			return fmt.Errorf("optioneod: manifest: [[index]] %q has no matching [[underlying]]", r.Symbol)
		case r.StrikeMultiple < 1:
			return fmt.Errorf("optioneod: manifest: [[index]] %s: strike_multiple must be at least 1", r.Symbol)
		case r.MaxContracts < 1:
			return fmt.Errorf("optioneod: manifest: [[index]] %s: max_contracts must be at least 1", r.Symbol)
		case r.SpotDivisor < 1:
			return fmt.Errorf("optioneod: manifest: [[index]] %s: spot_divisor must be at least 1", r.Symbol)
		case r.ExpiryKind != expiryListed && r.ExpiryKind != expiryThirdFriday:
			return fmt.Errorf("optioneod: manifest: [[index]] %s: expiry_kind must be listed or third_friday", r.Symbol)
		case r.SigmaIndex == "":
			return fmt.Errorf("optioneod: manifest: [[index]] %s: sigma_index must not be empty", r.Symbol)
		case r.SpotIndex == "":
			return fmt.Errorf("optioneod: manifest: [[index]] %s: spot_index must not be empty", r.Symbol)
		}
	}
	return nil
}

// indexOf is the [[index]] row of symbol.
func (m manifest) indexOf(symbol string) (indexRow, bool) {
	for _, r := range m.Index {
		if r.Symbol == symbol {
			return r, true
		}
	}
	return indexRow{}, false
}

// validateBuyKeys wants buy_low and buy_high both or neither, each a positive decimal, low below high.
func (m manifest) validateBuyKeys() error {
	if m.BuyLow == "" && m.BuyHigh == "" {
		return nil
	}
	if m.BuyLow == "" {
		return fmt.Errorf("optioneod: manifest: buy_low is missing, buy_high needs it")
	}
	if m.BuyHigh == "" {
		return fmt.Errorf("optioneod: manifest: buy_high is missing, buy_low needs it")
	}
	lo, ok := positiveDecimal(m.BuyLow)
	if !ok {
		return fmt.Errorf("optioneod: manifest: buy_low must be a positive decimal")
	}
	hi, ok := positiveDecimal(m.BuyHigh)
	if !ok {
		return fmt.Errorf("optioneod: manifest: buy_high must be a positive decimal")
	}
	if lo.Cmp(hi) >= 0 {
		return fmt.Errorf("optioneod: manifest: buy_low must be below buy_high")
	}
	return nil
}

// positiveDecimal reads s as a decimal number above zero: digits with at most one point, no sign, exponent,
// fraction or base prefix.
func positiveDecimal(s string) (*big.Rat, bool) {
	dots := 0
	for _, c := range s {
		switch {
		case c == '.':
			dots++
		case c < '0' || c > '9':
			return nil, false
		}
	}
	if dots > 1 {
		return nil, false
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok || r.Sign() <= 0 {
		return nil, false
	}
	return r, true
}

// validateUnderlying asks a stock underlying for a symbol only (runKey ends one without con_id or currency as
// NoValue) and every other underlying for symbol, con_id and currency.
func (m manifest) validateUnderlying(u underlying) error {
	if m.isStock(u) {
		if u.Symbol == "" {
			return fmt.Errorf("optioneod: manifest: a stock underlying needs a symbol")
		}
		return nil
	}
	if u.Symbol == "" || u.ConID == 0 || u.Currency == "" {
		return fmt.Errorf("optioneod: manifest: an underlying needs symbol, con_id and currency")
	}
	return nil
}

// parseHHMM reads "HHMM" as minutes after midnight.
func fourDigits(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func parseHHMM(s string) (int, error) {
	if len(s) != 4 {
		return 0, fmt.Errorf("bad time")
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n/100 > 23 || n%100 > 59 {
		return 0, fmt.Errorf("bad time")
	}
	return n/100*60 + n%100, nil
}

// canonical is a deterministic text of every setting, for the plan hash.
func (m manifest) canonical() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d|%s|%s|%s|%d|%s|%d|%g|%d|%d|%d|%s|%s", m.Version, m.ID, m.Zone, m.Window.Rule, *m.Window.OpenAfterMinutes,
		m.Window.Calendar, m.MaxExpiries, m.StrikeBand, m.MaxStrikesPerSide, m.Chunk, m.BudgetMinutes, m.FreezeAfter, m.OptionPriceIncrement)
	if m.StrikeBandBelow != 0 || m.StrikeBandAbove != 0 {
		fmt.Fprintf(&b, "|band%g-%g", m.StrikeBandBelow, m.StrikeBandAbove)
	}
	if m.MinDTE != 0 || m.MaxDTE != 0 || m.MaxStrikes != 0 {
		fmt.Fprintf(&b, "|dte%d-%d|%d", m.MinDTE, m.MaxDTE, m.MaxStrikes)
	}
	if m.TradingClass != "" {
		fmt.Fprintf(&b, "|class%s", m.TradingClass)
	}
	if m.LedgerDataset != datasetID {
		fmt.Fprintf(&b, "|ledger%s", m.LedgerDataset)
	}
	if m.BuyLow != "" {
		fmt.Fprintf(&b, "|buyband%s-%s", m.BuyLow, m.BuyHigh)
	}
	if len(m.Index) > 0 {
		fmt.Fprintf(&b, "|index%d-%g-%g", m.SigmaDays, m.SigmaFallback, m.BandK)
		for _, r := range m.Index {
			fmt.Fprintf(&b, "\n%+v", r)
		}
	}
	hasStock := false
	for _, u := range m.Underlying {
		fmt.Fprintf(&b, "\n%+v", u)
		if bp := m.buy[u.Symbol]; bp != "" {
			fmt.Fprintf(&b, "|buy%s", bp)
		}
		hasStock = hasStock || m.isStock(u)
	}
	if hasStock {
		fmt.Fprintf(&b, "\nstock%+v", m.Stock)
	}
	return b.String()
}

// isStock tells a JP stock underlying from the index: the manifest holds a [stock] table and u is a stock.
func (m manifest) isStock(u underlying) bool {
	return m.Stock.ChainExchange != "" && u.SecType == "STK"
}

// forStock is m with the selection values of the [stock] table in place of the manifest-wide ones.
func (m manifest) forStock() manifest {
	m.MinDTE, m.MaxDTE, m.MaxStrikes = m.Stock.MinDTE, m.Stock.MaxDTE, m.Stock.MaxStrikes
	if m.Stock.MaxStrikesPerSide > 0 {
		m.MaxStrikesPerSide = m.Stock.MaxStrikesPerSide
	}
	return m
}
