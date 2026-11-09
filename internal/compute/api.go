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

// EaseSeries holds the seven series that Ease reads, as FromSeriesPoints gives them. The
// three liquidity series WALCL, WDTGAL and RRPONTSYD are in whole dollars; DGS10, DFF and
// BAMLH0A0HYM2 are in percent; SP500 is in index points and only its dates are used.
type EaseSeries struct {
	WALCL, WDTGAL, RRPONTSYD, DGS10, DFF, SP500, BAMLH0A0HYM2 Series
}

// EaseBand is the word for the score of an ease figure.
type EaseBand string

// The bands of an EaseFigure.
const (
	EaseTighter EaseBand = "tighter"
	EaseSteady  EaseBand = "steady"
	EaseLooser  EaseBand = "looser"
)

// EasePart is one of the four inputs of the ease figure. Input is "net_liquidity",
// "us10y", "fed_funds" or "hy_oas", in that order. Status is ok or missing. Reason is
// no_input (the series has no point), short_history (it has points, fewer than the
// window; for net_liquidity, one of the eight summed Wednesdays has no figure) or
// stale_input (the figure is older than its age). Score is rounded half-even to 2
// places. A missing part with no figure has an empty DataDay and a Figure of 0.
type EasePart struct {
	Input   string
	DataDay contracts.Day
	Figure  float64
	Score   float64
	Status  Status
	Reason  Reason
}

// EaseFigure is the ease figure: Figure.DataDay is the newest data day of the parts used,
// Value the score to 2 places, Reason no_input when fewer than 3 parts are present and
// Observations the number of parts used. Band is empty when the value is missing. Parts
// always has four entries, in the order of EasePart.
type EaseFigure struct {
	Figure
	Band  EaseBand
	Parts []EasePart
}

// Ease computes the ease figure, purely: no I/O and no clock. id is "macro.ease" and the
// rule comes from Rules; an unknown id gives a missing figure with reason bad_input.
type Ease interface {
	Ease(id string, s EaseSeries, asOf contracts.Day) EaseFigure
}

// NewEaseFunc builds an Ease that reads its rule from the given Rules.
type NewEaseFunc func(Rules) Ease

// CandidateKind says which rule a Candidate row comes from: "short_put", "covered_call"
// or "held_call_guard" (a short call the owner already holds, judged by the guard).
type CandidateKind string

const (
	KindShortPut      CandidateKind = "short_put"
	KindCoveredCall   CandidateKind = "covered_call"
	KindHeldCallGuard CandidateKind = "held_call_guard"
)

// GateStatus is the verdict of one gate: "pass", "block" or "unknown". Unknown is never a
// pass: it marks the row and keeps its reason.
type GateStatus string

const (
	GatePass    GateStatus = "pass"
	GateBlock   GateStatus = "block"
	GateUnknown GateStatus = "unknown"
)

// GateResult is the verdict of one gate on one row. Gate is the gate word:
//
//	short_put:       buy_price, quote, events, funding, cash_rate (in this order)
//	covered_call:    shares, strike_floor, quote, events, guard (in this order)
//	held_call_guard: guard
//
// Reason is the first reason code of the gate when Status is not pass, else empty. Codes:
//
//	buy_price:    no_buy_price (block: no BuyPrice), above_buy_price (block)
//	quote:        no_quote, no_bid, no_ask, small_bid_size, wide_spread, low_open_interest
//	              (block); delayed_data (unknown: market data type 3 or 4)
//	events:       earnings_before_expiry (block); earnings_unsupported, earnings_error,
//	              no_earnings, stale_earnings, beyond_horizon, stale_chain (unknown)
//	funding:      no_fit (block), no_fx, stale_account, no_account (unknown)
//	cash_rate:    yield_below_cash (block; a yield equal to the rate blocks too);
//	              no_rate, stale_rate (unknown)
//	shares:       no_free_shares (block)
//	strike_floor: below_floor (block); no_floor (unknown)
//	guard:        early_assignment_risk (block); no_dividend_info, stale_dividend (unknown)
type GateResult struct {
	Gate   string
	Status GateStatus
	Reason string
}

// OptionQuote is one captured option contract of the chain day. Decimals are text:
// the caller converts each float with strconv.FormatFloat(v, 'f', -1, 64) and rounds
// half-even to 4 places; an empty text means the value is absent.
type OptionQuote struct {
	InstrumentID string
	Expiry       contracts.Day
	Right        string // "P" or "C"
	Strike       string
	Multiplier   string
	Bid          string
	Ask          string
	BidSize      int
	OpenInterest int
	Delta        string
	IV           string
	// MarketDataType is the IBKR market data type: 1 live, 2 frozen, 3 delayed, 4 delayed frozen.
	MarketDataType int
	// QuoteOK is the row status of the capture: true when the row is "ok".
	QuoteOK bool
}

// EarningsInfo is the earnings row of an underlying (as the reader of 606 gives it).
// State is "listed", "absent", "unsupported" or "error"; Next is the next results date,
// Previous the newest earlier listed date, RowDay the day the row was written; each is
// empty when unknown. Override replaces the vendor date when set.
type EarningsInfo struct {
	State    string
	Next     contracts.Day
	Previous contracts.Day
	RowDay   contracts.Day
	Override contracts.Day
}

// DividendInfo is the dividend row of an underlying: State is "present", "no_value",
// "error" or "unsupported"; NextExDate is the next ex-date and Amount the amount per
// share (decimal text); RowDay is the day the row was written.
type DividendInfo struct {
	State      string
	NextExDate contracts.Day
	Amount     string
	RowDay     contracts.Day
}

// AliasFunds is the newest account capture of one alias: Funds is AvailableFunds (decimal
// text) in Currency, Shares the stock shares held, AvgCost their average cost (decimal
// text), OpenShortCalls the contracts of open short calls on the underlying and
// CaptureDay the capture day. Alias is "A" or "B".
type AliasFunds struct {
	Alias          string
	Funds          string
	Currency       string
	Shares         int
	AvgCost        string
	OpenShortCalls int
	CaptureDay     contracts.Day
}

// CandidateInput is everything Evaluate needs for one underlying; the rule has no calendar,
// no clock and no store. Days are YYYY-MM-DD texts; money, rates and prices are decimal
// text. RunDay is the day of the run, ChainDay the day of the quotes (gate events and
// every row: a ChainDay that is not the latest session day on or before RunDay gives
// unknown stale_chain). SessionDays are the XNYS session days in ascending order, from
// the oldest input date (earnings row, account capture, chain day) to RunDay, supplied by
// the caller; every age in sessions is a difference of positions in this list. Rate is
// DGS3MO in percent (4.20 means 4.2 percent) with RateDay its day; empty when absent.
// BuyPrice and SalePrice are the owner's prices, empty when unset. Spot is the underlying
// price on ChainDay. Nil Dividend or empty Accounts make their gates unknown.
type CandidateInput struct {
	RunDay       contracts.Day
	ChainDay     contracts.Day
	SessionDays  []contracts.Day
	Underlying   string
	Spot         string
	Quotes       []OptionQuote
	BuyPrice     string
	SalePrice    string
	Earnings     *EarningsInfo
	Dividend     *DividendInfo
	Rate         string
	RateDay      contracts.Day
	Accounts     []AliasFunds
	Kinds        []CandidateKind
	OpenShortIDs []string // instrument ids of open short calls, for held_call_guard
}

// Candidate is one evaluated row. Status is "ok" (every gate passed), "unknown" (no gate
// blocked, one is unknown) or "blocked" (a gate blocked). Gates holds the results in the
// order of GateResult. Rank is the 1-based position among the ok rows of the run and
// RankInUnderlying within the underlying; both are 0 for a row that is not ok. Marks are
// words: itm, earnings_unconfirmed, earnings_beyond_horizon, no_dividend. Fits maps an
// alias to fit, no_fit or unknown. AnnualizedYield and BreakEven are decimal text
// (yield to 6 places). GuardVerdict is risk, ok or unknown for held_call_guard rows.
type Candidate struct {
	Kind             CandidateKind
	Underlying       string
	InstrumentID     string
	Alias            string
	Expiry           contracts.Day
	DTE              int
	Right            string
	Strike           string
	Multiplier       string
	Bid              string
	Ask              string
	Spot             string
	BuyPrice         string
	SalePrice        string
	BreakEven        string
	AnnualizedYield  string
	Delta            string
	IV               string
	Extrinsic        string
	Status           string
	Gates            []GateResult
	Marks            []string
	Fits             map[string]string
	GuardVerdict     string
	Rank             int
	RankInUnderlying int
}

// CandidateParams are the limits of the rules (keys of job.toml): MinDTE and MaxDTE
// (calendar days), MinBidSize, MaxSpread (decimal text, ratio), MinOpenInterest,
// DivGuardFactor (decimal text), MaxRateAgeDays, MaxEventAgeSessions, MinReportGapDays.
type CandidateParams struct {
	MinDTE              int
	MaxDTE              int
	MinBidSize          int
	MaxSpread           string
	MinOpenInterest     int
	DivGuardFactor      string
	MaxRateAgeDays      int
	MaxEventAgeSessions int
	MinReportGapDays    int
}

// OptionCandidates evaluates the contracts of one underlying, purely: no I/O and no
// clock, exact arithmetic on decimal text. The result holds one row per contract and
// kind (per alias for calls), with every gate in order; ok rows are ranked by annualized
// yield descending, then DTE, strike and instrument id ascending; other rows have no rank.
type OptionCandidates interface {
	Evaluate(CandidateInput, CandidateParams) []Candidate
}

// NewOptionCandidatesFunc builds an OptionCandidates.
type NewOptionCandidatesFunc func() OptionCandidates

// LedgerTrade is one stored Flex trade row: the latest revision of one transaction. Every
// attribute value is text, as Flex sent it.
type LedgerTrade struct {
	TransactionID string
	Revision      int
	Attributes    map[string]string
}

// LedgerMark is the newest option_eod quote of one contract: Bid and Ask and the
// underlying close are decimal text, Day is the quote day.
type LedgerMark struct {
	Contract        string
	Day             contracts.Day
	Bid             string
	Ask             string
	UnderlyingClose string
}

// LedgerInput is everything Replay needs for one alias; the rule has no calendar, no
// clock and no store. Trades holds the latest revision of each transaction. SessionDays
// are the XNYS session days in ascending order, supplied by the caller; ages of marks are
// differences of positions in this list. AsOf is the day of the replay.
type LedgerInput struct {
	Alias       string
	Trades      []LedgerTrade
	Marks       []LedgerMark
	SessionDays []contracts.Day
	AsOf        contracts.Day
}

// LedgerParams are the limits of the rule: MarkMaxAgeSessions is the oldest mark, in
// session days, that is still used (a key of job.toml).
type LedgerParams struct {
	MarkMaxAgeSessions int
}

// LedgerPosition is an open lot or the part a close left. Money and quantities are decimal
// text in the Currency of the contract, rounded half-even to 4 places; no figure adds two
// currencies. Status is open, closed, expired, assigned or exercised; Early marks
// an assignment before the expiry date. MarkStatus is ok or no_mark; Mark, MarkDay and
// UnrealizedPnL are empty without a mark, RealizedPnL is empty while the position is open.
// PremiumIncome is the cash of its opens plus its closes, Commissions the sum of its rows.
type LedgerPosition struct {
	PositionID string
	Alias      string
	Underlying string
	Contract   string
	Status     string
	Early      bool

	// Terms: Right is P or C; ClosedOn is empty while open; Fills are the rows in order;
	// BrokerRealized is the broker's realized figure, empty when it reports none.
	Right          string
	Expiry         string
	Strike         string
	OpenedOn       contracts.Day
	ClosedOn       contracts.Day
	OpenPrice      string
	Fills          []LedgerFill
	BrokerRealized string

	Quantity      string
	Multiplier    string
	Currency      string
	PremiumIncome string
	Commissions   string
	RealizedPnL   string
	Mark          string
	MarkDay       contracts.Day
	MarkStatus    string
	UnrealizedPnL string
}

// LedgerCycle is one wheel cycle of an alias and underlying, from the first share event to
// flat. The numbers are separate and never totalled: StockRealizedPnL, PremiumIncome (the
// positions that belong to the cycle) and StockCommissions. CostBasisPerShare is the price
// of the buy row, the strike for an assignment; no premium is subtracted from it.
// UnrealizedStockPnL is empty without a stock mark. PremiumOutsideCycles is the premium
// income of the alias and name that belongs to no cycle.
type LedgerCycle struct {
	CycleID              string
	Alias                string
	Underlying           string
	Currency             string
	Shares               string
	CostBasisPerShare    string
	StockRealizedPnL     string
	UnrealizedStockPnL   string
	PremiumIncome        string
	StockCommissions     string
	PremiumOutsideCycles string
}

// LedgerResult is the replay of one alias. RuleVersion starts at 1. Unresolved lists the
// transaction ids of closes with no open before them; Review lists the transaction ids of
// cancel or correction rows. Neither enters a figure.
type LedgerResult struct {
	RuleVersion int
	Positions   []LedgerPosition
	Cycles      []LedgerCycle
	Unresolved  []string
	Review      []string
}

// OptionLedger replays the stored trade rows of one alias, purely: no I/O and no clock,
// exact arithmetic on decimal text, no float. The same input gives the same result.
type OptionLedger interface {
	Replay(LedgerInput, LedgerParams) LedgerResult
}

// NewOptionLedgerFunc builds an OptionLedger.
type NewOptionLedgerFunc func() OptionLedger

// LedgerFill is one row of a position; Action is the kind of the row, numbers are decimal text.
type LedgerFill struct {
	Day                                 contracts.Day
	Action, Quantity, Price, Commission string
}

// StructureQuote is one option quote of a day; numbers are decimal text, empty when absent.
type StructureQuote struct {
	Underlying, Right, Strike, Expiry string
	Day                               contracts.Day
	Bid, Ask, UnderlyingClose         string
	Delta, Gamma, Theta, Vega         string
}

// HeldOption is one option row of the newest capture.
type HeldOption struct{ Underlying, Right, Strike, Expiry, Quantity string }

// StructureInput is the input of one alias; SessionDays are ascending XNYS days. Pure.
type StructureInput struct {
	Alias       string
	Positions   []LedgerPosition
	Cycles      []LedgerCycle
	Quotes      []StructureQuote
	Held        []HeldOption
	HeldDay     contracts.Day
	AsOf        contracts.Day
	SessionDays []contracts.Day
}

// StructureParams holds the parameters of Evaluate.
type StructureParams struct {
	MarkMaxAgeSessions int
	CaptureTarget      string
	ExitDTE            int
	CommissionDays     int
}

// Structure is one recognised structure. Decide is true when the short leg expires within
// ExitDTE days or EarnedShare is at least CaptureTarget; the two Param texts are copied.
type Structure struct {
	ID, Alias, Underlying, Currency, Kind string
	Legs                                  []StructureLeg
	Figures                               StructureFigures
	Payoff                                []PayoffPoint
	Greeks                                GreekSet
	Levels                                []StructureLevel
	Decide                                bool
	CaptureParam, ExitDTEParam            string
}

// StructureLeg is one leg of a structure.
type StructureLeg struct {
	PositionID, Right, Strike, Expiry, Quantity, Multiplier, OpenPrice, Mid, LegPnL, MarkStatus string
}

// StructureFigures are the figures of a structure; each is a FigureValue.
type StructureFigures struct {
	MaxLoss, MaxGain, BreakEven, UnrealizedPnL, EarnedShare, ExpiryDistance FigureValue
	CloseCost, RollCredit, RollNet, RecentCommission, CashAfter             FigureValue
}

// FigureValue is a number or the reason it is absent. Gap is empty or one of missing_quote,
// stale_input, unsupported_currency, no_rule, no_data, missing_input; exactly one of Value
// and Gap is set.
type FigureValue struct {
	Value string
	Gap   string
}

// PayoffPoint is one point of the payoff at expiry.
type PayoffPoint struct{ Price, PnL string }

// GreekSet is the net greeks of a structure on Day.
type GreekSet struct {
	Delta, Gamma, Theta, Vega FigureValue
	Day                       contracts.Day
}

// StructureLevel is a named price level and its distance.
type StructureLevel struct{ Name, Price, Distance string }

// StructureResult holds Structures in the order of Decision 12; Other the unrecognised kinds.
type StructureResult struct {
	Structures []Structure
	Other      []Structure
	Diff       CaptureDiff
}

// CaptureDiff compares the capture with the ledger; Skipped names why no comparison was made.
type CaptureDiff struct {
	CaptureOnly, LedgerOnly []string
	Skipped                 string
}

// OptionStructures groups the positions of one alias into structures, purely: no I/O, no clock.
type OptionStructures interface {
	Evaluate(StructureInput, StructureParams) StructureResult
}

// NewOptionStructuresFunc builds an OptionStructures.
type NewOptionStructuresFunc func() OptionStructures

// ResultInput is the input of Summarise.
type ResultInput struct {
	Alias              string
	Positions          []LedgerPosition
	Unresolved, Review []string
	AsOf               contracts.Day
}

// ResultSummary is the result of an alias by side and currency.
type ResultSummary struct{ Sections []ResultSection }

// ResultSection is one side and currency; money is decimal text, KeptShare a 6-place ratio or empty.
type ResultSection struct {
	Side, Currency                                                string
	Months                                                        []ResultMonth
	Endings                                                       []ResultEnding
	Realized, Premium, KeptShare, AssignedPremium, AssignedShares string
	Reliability                                                   ResultReliability
}

// ResultMonth is the realized result of a month and its running total.
type ResultMonth struct {
	Month, Realized, Running string
	Over                     bool
}

// ResultEnding counts the positions that ended with Status and their realized result.
type ResultEnding struct {
	Status   string
	Count    int
	Realized string
}

// ResultReliability counts positions by agreement with the broker figure.
type ResultReliability struct {
	Equal, Differ, NoBroker int
	Unresolved, Review      []string
}

// OptionResults summarises the realized result of one alias, purely.
type OptionResults interface {
	Summarise(ResultInput) ResultSummary
}

// NewOptionResultsFunc builds an OptionResults.
type NewOptionResultsFunc func() OptionResults

// NoticePosition is one held position as plain text, for the option notices.
type NoticePosition struct {
	Symbol, SecType, Expiry, Strike, Right, Multiplier, Quantity, Currency string
}

// NoticeGuard is a stored held-call guard verdict; Reasons are the codes of its blocked gates.
type NoticeGuard struct {
	Symbol, Expiry, Strike, Status string
	Reasons                        []string
}

// NoticeEarnings is the next earnings day of an issuer; Next is nil when none is known.
type NoticeEarnings struct {
	Symbol string
	Next   *contracts.Day
}

// NoticeValue is one stored account value as decimal text, such as the cushion tag.
type NoticeValue struct {
	Tag, Value, Currency string
}

// NoticeInput is the stored rows of one alias, converted to plain values.
type NoticeInput struct {
	Day           contracts.Day
	Alias         string
	CaptureDay    contracts.Day
	Positions     []NoticePosition
	PrevDay       contracts.Day
	PrevPositions []NoticePosition
	Guards        []NoticeGuard
	Earnings      []NoticeEarnings
	Values        []NoticeValue
}

// NoticeParams holds the thresholds; the cushion limits are decimal text.
type NoticeParams struct {
	ExpiryDays              int
	CushionWarn, CushionAct string
}

// Notice is one open notice, with its severity (info, warn, act).
type Notice struct {
	Code, Severity, Symbol, Contract string
	Expiry                           contracts.Day
	Facts                            []NoticeFact
}

// NoticeFact is one labelled figure of a notice.
type NoticeFact struct {
	Label, Value string
}

// CashDay is the assignment cash due on a day, in one currency, as decimal text.
type CashDay struct {
	Day              contracts.Day
	Currency, Amount string
}

// NoticeResult is the notices of one alias in the rule's order.
type NoticeResult struct {
	RuleVersion int
	Notices     []Notice
	CashByDate  []CashDay
	Cushion     string
}

// OptionNotices evaluates the option notices of one alias, purely: no I/O, no clock.
type OptionNotices interface {
	Evaluate(NoticeInput, NoticeParams) NoticeResult
}

// NewOptionNoticesFunc builds an OptionNotices.
type NewOptionNoticesFunc func() OptionNotices

// DefaultNoticeParamsFunc gives the confirmed thresholds.
type DefaultNoticeParamsFunc func() NoticeParams
