package optioneod

import (
	"fmt"
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

type manifest struct {
	Version              int           `toml:"version"`
	ID                   string        `toml:"id"`
	Zone                 string        `toml:"zone"`
	Window               window        `toml:"window"`
	Schedule             scheduleTable `toml:"schedule"`
	MaxExpiries          int           `toml:"max_expiries"`
	StrikeBand           float64       `toml:"strike_band"`
	MaxStrikesPerSide    int           `toml:"max_strikes_per_side"`
	StrikeBandBelow      float64       `toml:"strike_band_below"`
	StrikeBandAbove      float64       `toml:"strike_band_above"`
	MinDTE               int           `toml:"min_dte"`
	MaxDTE               int           `toml:"max_dte"`
	MaxStrikes           int           `toml:"max_strikes"`
	Chunk                int           `toml:"chunk"`
	BudgetMinutes        int           `toml:"budget_minutes"`
	FreezeAfter          string        `toml:"freeze_after"`
	OptionPriceIncrement string        `toml:"option_price_increment"`
	// TradingClass, when set, makes the chain choice take the chain of that trading class on the underlying's exchange.
	TradingClass string `toml:"trading_class"`
	// LedgerDataset is the dataset id of the plan keys; default obs.option_eod.
	LedgerDataset string       `toml:"ledger_dataset"`
	Underlying    []underlying `toml:"underlying"`
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
	sort.Slice(m.Underlying, func(i, j int) bool { return m.Underlying[i].Symbol < m.Underlying[j].Symbol })
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
	if _, err := time.LoadLocation(m.Zone); err != nil {
		return fmt.Errorf("optioneod: manifest: zone is unknown")
	}
	if _, err := parseHHMM(m.FreezeAfter); err != nil {
		return fmt.Errorf("optioneod: manifest: freeze_after must be HHMM")
	}
	for _, u := range m.Underlying {
		if err := m.validateUnderlying(u); err != nil {
			return err
		}
	}
	return nil
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
