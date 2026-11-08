// Package compute turns typed series into figures. It is pure: no store, no clock, no file.
package compute

import "github.com/andrewrong/optdesk/internal/contracts"

import (
	"context"
	"io"
)

// Point is one value of a series on one day.
type Point struct {
	Day   contracts.Day
	Value float64
}

// Series is ascending by day, with one point per day.
type Series []Point

// Status tells whether a Figure holds a number.
type Status string

// The statuses of a Figure.
const (
	StatusOK      Status = "ok"
	StatusMissing Status = "missing"
)

// Reason says why a Figure is missing; it is empty for an ok Figure.
type Reason string

// The reasons of a missing Figure.
const (
	NoInput      Reason = "no_input"
	ShortHistory Reason = "short_history"
	StaleInput   Reason = "stale_input"
	ZeroRange    Reason = "zero_range"
	BadInput     Reason = "bad_input"
)

// Compare is the tie rule of a percent rank, fixed per figure id by its register entry.
type Compare string

// The tie rules of a percent rank.
const (
	Below     Compare = "below"
	AtOrBelow Compare = "at_or_below"
)

// MAType names the moving average of an indicator that takes one.
type MAType int

// The moving average types.
const (
	MASMA MAType = iota
	MAEMA
	MAWMA
)

// Rule holds the numbers of one register entry, keyed by ID and Version.
// Unit is one word of the reading_unit enum of schema v3: decimal, percent, vol_point,
// per_calendar_day, count, days, money, ratio, text_tag. Hash is the rule hash.
type Rule struct {
	ID          string
	Version     int
	Convention  string
	Unit        string
	Scale       int
	Compare     Compare
	N           int
	Window      int
	MinValid    int
	Years       int
	TradingDays int
	MaxAgeDays  int
	Hash        string
}

// Figure is the result of one figure method. A missing Figure has Value 0 and a Reason.
type Figure struct {
	ID           string
	Version      int
	Subject      string
	Day          contracts.Day // the as-of day the caller passed
	DataDay      contracts.Day // the day of the newest input point
	Value        float64
	Status       Status
	Reason       Reason
	Observations int
	StaleRate    bool
}

// StrikeQuote holds the IBKR model values of one strike. An IV of 0 means no model IV;
// a mid of 0 with Has true is not allowed.
type StrikeQuote struct {
	Strike, CallIV, PutIV, CallMid, PutMid float64
	HasCall, HasPut                        bool
}

// ExpiryQuotes holds all strikes of one expiry.
type ExpiryQuotes struct {
	MinutesToExpiry int
	Strikes         []StrikeQuote
}

// Indicators are the technical indicators. Each result has the input's length, with NaN
// before the first defined value. Errors are contracts.TypedError: Permanent for a bad
// period or unequal lengths.
type Indicators interface {
	SMA(values []float64, period int) ([]float64, error)
	EMA(values []float64, period int) ([]float64, error)
	WMA(values []float64, period int) ([]float64, error)
	RSI(values []float64, period int) ([]float64, error)
	ROC(values []float64, period int) ([]float64, error)
	// MACD returns the macd line, the signal line and the histogram.
	MACD(values []float64, fast, slow, signal int) (macd, sig, hist []float64, err error)
	// BBands returns the upper, middle and lower band.
	BBands(values []float64, period int, upDev, downDev float64, maType MAType) (upper, middle, lower []float64, err error)
	ATR(high, low, closes []float64, period int) ([]float64, error)
}

// Stats is the descriptive statistics. An empty input is a Permanent error, never NaN.
type Stats interface {
	Mean(values []float64) (float64, error)
	// StdDev is the sample standard deviation (ddof 1).
	StdDev(values []float64) (float64, error)
	// PercentileRank is the share of values below (or at or below) x.
	PercentileRank(values []float64, x float64, cmp Compare) (float64, error)
}

// Rolling is the window loop over Stats; each result has the input's length, NaN in warm-up.
type Rolling interface {
	Mean(values []float64, window int) ([]float64, error)
	StdDev(values []float64, window int) ([]float64, error)
	Min(values []float64, window int) ([]float64, error)
	Max(values []float64, window int) ([]float64, error)
	PercentileRank(values []float64, window int, cmp Compare) ([]float64, error)
}

// SeriesOps handles series as values.
type SeriesOps interface {
	// FromSeriesPoints keeps, per observation date, the point of the latest realtime_start
	// whose status is present and whose value is not null; no duplicate day results.
	FromSeriesPoints(points []contracts.SeriesPoint) Series
	// Align keeps the days both series have.
	Align(a, b Series) (Series, Series)
	// LogReturns is dated by the later point.
	LogReturns(s Series) Series
	Values(s Series) []float64
}

// Units holds the unit conversions.
type Units interface {
	PercentToDecimal(p float64) float64
	// ThetaPerCalendarDay divides a per-year theta by 365.
	ThetaPerCalendarDay(perYear float64) float64
	// VegaPerVolPoint multiplies a per-unit-vol vega by 0.01.
	VegaPerVolPoint(perUnitVol float64) float64
	// RoundHalfEven rounds the exact binary value of v at scale places, as plain decimal
	// text. NaN, Inf and a negative scale are Permanent errors.
	RoundHalfEven(v float64, scale int) (string, error)
}

// Rules is the table of rules behind the register.
type Rules interface {
	Rule(id string) (Rule, bool)
	IDs() []string
}

// Figures computes one Figure per call from the rule of id. Day is asOf, ID and Version
// come from Rules.
type Figures interface {
	HistoricalVol(id string, closes Series, asOf contracts.Day) Figure
	// Iv30 interpolates the 30-day IV from IBKR model values only; StaleRate is true
	// when rate.Day != asOf.
	Iv30(id string, chain []ExpiryQuotes, rate Point, asOf contracts.Day) Figure
	IvPercentile(id string, iv30 Series, asOf contracts.Day) Figure
	IvRank(id string, iv30 Series, asOf contracts.Day) Figure
	// MovingAverageGap is the newest close over the mean of the last N, minus 1.
	MovingAverageGap(id string, closes Series, asOf contracts.Day) Figure
	PercentileInYears(id string, s Series, asOf contracts.Day) Figure
	// Ratio is num over den on the same day; a zero denominator is bad_input.
	Ratio(id string, num, den Series, asOf contracts.Day) Figure
}

// Constructor function types of the implementations.
type (
	NewIndicatorsFunc func() Indicators
	NewStatsFunc      func() Stats
	NewRollingFunc    func(Stats) Rolling
	NewSeriesOpsFunc  func() SeriesOps
	NewUnitsFunc      func() Units
	NewRulesFunc      func() Rules
	NewFiguresFunc    func(Stats, Rolling, SeriesOps, Rules) Figures
)

// AlertRule is one market alert rule over a stored figure, plain values only. Op is a
// comparison word and Threshold a decimal text; both are checked by the loader.
type AlertRule struct {
	ID, Title, Figure                                string
	FigureVersion                                    int
	SubjectKind, Subject, Unit, Op, Threshold, Level string
	MaxDataAgeDays                                   int
}

// AlertRules is a loaded rule set. PublicSubjects lists the subjects whose alerts may
// leave the process; Hash is the sha256 of the rule file bytes.
type AlertRules struct {
	Rules          []AlertRule
	PublicSubjects []string
	Hash           string
}

// AlertResult is the outcome of one rule. State is "fired", "clear" or "no_data". Reason
// is empty, or one of "no_reading", "missing_input", "unit_mismatch", "stale_data" or
// "no_data_day". Day is the row day and DataDay the day of the row's newest input. Detail
// is built from fixed words and the stored value, never from free text. Public is true
// when the rule's subject is in AlertRules.PublicSubjects.
type AlertResult struct {
	RuleID, Title, Level, Figure, Value, Detail, Reason string
	State                                               string
	Day, DataDay                                        contracts.Day
	Public                                              bool
}

// ReadingFunc reads the stored readings of one figure and subject for the days from to
// to. The function passed to EvaluateAlertsFunc applies the winner rule of the report
// page: per day a missing_input row shows only when the day has no ok row, and the
// highest rev of state ok wins.
type ReadingFunc func(ctx context.Context, name, subjectKind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error)

// EvaluateAlertsFunc evaluates every rule, purely. For each rule it calls read for the
// days today-5 to today and uses the newest row returned. A reader error is an error of
// the call, never a no_data result. The results follow the order of the rules.
type EvaluateAlertsFunc func(ctx context.Context, rules AlertRules, read ReadingFunc, today contracts.Day) ([]AlertResult, error)

// LoadAlertRulesFunc decodes and checks a rule file; an error names the key and the rule
// id and never a value.
type LoadAlertRulesFunc func(r io.Reader) (AlertRules, error)

// DefaultAlertRulesFunc returns the rules embedded in the program.
type DefaultAlertRulesFunc func() (AlertRules, error)

// SharpMarkFunc says whether a day change and a 5-day change, in percent, are sharp. A
// change is sharp when its absolute value reaches the limit of SharpLimitsFunc, the limit
// itself included. A nil change is a gap and is never sharp. The two results are
// independent of each other.
type SharpMarkFunc func(day, five *float64) (daySharp, fiveSharp bool)

// SharpLimitsFunc returns the limits, in percent, that SharpMarkFunc applies to the day
// change and to the 5-day change, for a builder's note.
type SharpLimitsFunc func() (day, fiveDay float64)
