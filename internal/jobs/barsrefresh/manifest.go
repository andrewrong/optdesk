package barsrefresh

import (
	_ "embed"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"time"

	"github.com/pelletier/go-toml/v2"
)

//go:embed job.toml
var manifestBytes []byte

// Index is one [[index]] table: an index whose bars are pulled with the stocks of the watch list.
type Index struct {
	Symbol   string
	SecType  string
	Exchange string
	Currency string
	ConID    int64 // 0 is resolved once per run
}

// Stock is one [[stock]] table: a stock or fund whose bars are pulled under the key stock:<symbol>.
type Stock struct {
	Symbol   string
	Label    string
	Group    string // GroupProxy or GroupSector
	SecType  string
	Exchange string
	Currency string
	ConID    int64 // 0 is resolved once per run
}

// The two groups of a [[stock]] table.
const (
	GroupProxy  = "proxy"
	GroupSector = "sector"
)

// Schedule is the [schedule] table.
type Schedule struct {
	Calendar string
	Anchor   string
	Offset   time.Duration
	RunBound time.Duration
}

// Manifest is the job.toml of bars_refresh.
type Manifest struct {
	Version             int
	ID                  string
	BackfillDays        int
	DailyDays           int
	RestateTolerance    *big.Rat // exact compare, no float
	AdjustedCadenceDays int
	BackfillNamesPerRun int
	DeepenDays          int
	DeepenPieceDays     int
	DeepenNamesPerRun   int // 0 is no cap
	Indexes             []Index
	Stocks              []Stock
	Benchmark           string // a symbol of Stocks
	Schedule            Schedule
}

type rawIndex struct {
	Symbol   string `toml:"symbol"`
	SecType  string `toml:"sec_type"`
	Exchange string `toml:"exchange"`
	Currency string `toml:"currency"`
	ConID    int64  `toml:"con_id"`
}

type rawStock struct {
	Symbol   string `toml:"symbol"`
	Label    string `toml:"label"`
	Group    string `toml:"group"`
	SecType  string `toml:"sec_type"`
	Exchange string `toml:"exchange"`
	Currency string `toml:"currency"`
	ConID    int64  `toml:"con_id"`
}

type rawManifest struct {
	Version             int        `toml:"version"`
	ID                  string     `toml:"id"`
	BackfillDays        int        `toml:"backfill_days"`
	DailyDays           int        `toml:"daily_days"`
	RestateTolerance    string     `toml:"restate_tolerance"`
	AdjustedCadenceDays int        `toml:"adjusted_cadence_days"`
	BackfillNamesPerRun int        `toml:"backfill_names_per_run"`
	DeepenDays          int        `toml:"deepen_days"`
	DeepenPieceDays     int        `toml:"deepen_piece_days"`
	DeepenNamesPerRun   int        `toml:"deepen_names_per_run"`
	Index               []rawIndex `toml:"index"`
	Stock               []rawStock `toml:"stock"`
	GlobalAssets        struct {
		Benchmark string `toml:"benchmark"`
	} `toml:"global_assets"`
	Schedule            struct {
		Calendar string `toml:"calendar"`
		Anchor   string `toml:"anchor"`
		Offset   string `toml:"offset"`
		RunBound string `toml:"run_bound"`
	} `toml:"schedule"`
}

var decimal = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

func refuse(key, why string) error {
	return fmt.Errorf("barsrefresh: manifest: %s %s", key, why)
}

// DefaultManifest loads the embedded job.toml.
func DefaultManifest() (Manifest, error) { return LoadManifest(manifestBytes) }

// LoadManifest reads a job.toml. A malformed value is refused with an error that names the key, never the value.
func LoadManifest(data []byte) (Manifest, error) {
	var r rawManifest
	if err := toml.Unmarshal(data, &r); err != nil {
		var de *toml.DecodeError
		if errors.As(err, &de) {
			row, col := de.Position()
			return Manifest{}, fmt.Errorf("barsrefresh: manifest: not valid TOML at line %d column %d", row, col)
		}
		return Manifest{}, errors.New("barsrefresh: manifest: not valid TOML")
	}
	if r.ID == "" {
		return Manifest{}, refuse("id", "is missing")
	}
	for _, p := range []struct {
		key string
		v   int
	}{
		{"backfill_days", r.BackfillDays}, {"daily_days", r.DailyDays},
		{"adjusted_cadence_days", r.AdjustedCadenceDays}, {"backfill_names_per_run", r.BackfillNamesPerRun},
		{"deepen_days", r.DeepenDays}, {"deepen_piece_days", r.DeepenPieceDays},
	} {
		if p.v <= 0 {
			return Manifest{}, refuse(p.key, "must be positive")
		}
	}
	if r.DeepenNamesPerRun < 0 {
		return Manifest{}, refuse("deepen_names_per_run", "must not be negative")
	}
	if !decimal.MatchString(r.RestateTolerance) {
		return Manifest{}, refuse("restate_tolerance", "is not a decimal")
	}
	tol, ok := new(big.Rat).SetString(r.RestateTolerance)
	if !ok {
		return Manifest{}, refuse("restate_tolerance", "is not a decimal")
	}
	m := Manifest{
		Version: r.Version, ID: r.ID, BackfillDays: r.BackfillDays, DailyDays: r.DailyDays,
		RestateTolerance: tol, AdjustedCadenceDays: r.AdjustedCadenceDays,
		BackfillNamesPerRun: r.BackfillNamesPerRun, DeepenDays: r.DeepenDays,
		DeepenPieceDays: r.DeepenPieceDays, DeepenNamesPerRun: r.DeepenNamesPerRun,
	}
	for _, ix := range r.Index {
		for _, f := range []struct{ key, v string }{
			{"index.symbol", ix.Symbol}, {"index.exchange", ix.Exchange}, {"index.currency", ix.Currency},
		} {
			if f.v == "" {
				return Manifest{}, refuse(f.key, "is missing")
			}
		}
		m.Indexes = append(m.Indexes, Index(ix))
	}
	stocks, err := loadStocks(r.Stock, r.GlobalAssets.Benchmark)
	if err != nil {
		return Manifest{}, err
	}
	m.Stocks, m.Benchmark = stocks, r.GlobalAssets.Benchmark
	if r.Schedule.Calendar == "" {
		return Manifest{}, refuse("schedule.calendar", "is missing")
	}
	s := Schedule{Calendar: r.Schedule.Calendar, Anchor: r.Schedule.Anchor}
	if r.Schedule.Offset != "" {
		if s.Offset, err = time.ParseDuration(r.Schedule.Offset); err != nil {
			return Manifest{}, refuse("schedule.offset", "is not a duration")
		}
	}
	if r.Schedule.RunBound != "" {
		if s.RunBound, err = time.ParseDuration(r.Schedule.RunBound); err != nil {
			return Manifest{}, refuse("schedule.run_bound", "is not a duration")
		}
	}
	m.Schedule = s
	return m, nil
}

func loadStocks(raws []rawStock, benchmark string) ([]Stock, error) {
	var out []Stock
	seen := map[string]bool{}
	for _, s := range raws {
		for _, f := range []struct{ key, v string }{
			{"stock.symbol", s.Symbol}, {"stock.label", s.Label}, {"stock.exchange", s.Exchange}, {"stock.currency", s.Currency},
		} {
			if f.v == "" {
				return nil, refuse(f.key, "is missing")
			}
		}
		if s.Group != GroupProxy && s.Group != GroupSector {
			return nil, refuse("stock.group", "must be proxy or sector")
		}
		if seen[s.Symbol] {
			return nil, refuse("stock.symbol", "is repeated")
		}
		seen[s.Symbol] = true
		out = append(out, Stock(s))
	}
	if !seen[benchmark] {
		return nil, refuse("global_assets.benchmark", "is not a listed stock")
	}
	return out, nil
}
