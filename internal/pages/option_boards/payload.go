//nolint:unused // used by the page builders of entries 3 and 4 of docs/design/option-page-builders.md
package option_boards

import "encoding/json"

// The typed payloads of page.option_page.v2 and page.stock_option_page.v3. Money and percentages are
// json.Number of a stored decimal text, never a float. Builders fill these structs and edit none of them.

// optionPayload is page.option_page.v2.
type optionPayload struct {
	Schema        string        `json:"schema"`
	SchemaVersion int           `json:"schema_version"`
	RunID         string        `json:"run_id"`
	BuildID       string        `json:"build_id"`
	Scope         string        `json:"scope"`
	GeneratedAt   string        `json:"generated_at"`
	DataTimes     []sourceTime  `json:"data_times"`
	RunStatus     string        `json:"run_status"`
	Title         string        `json:"title"`
	Stale         staleLabel    `json:"stale"`
	Markets       []marketPane  `json:"markets"`
	Notes         []string      `json:"notes"`
	Gaps          []gap         `json:"gaps"`
	Refresh       refreshObject `json:"refresh"`
}

type marketPane struct {
	Market          string        `json:"market"`
	Label           string        `json:"label"`
	Currency        string        `json:"currency"`
	Headline        string        `json:"headline"`
	CardsTitle      string        `json:"cards_title"`
	Cards           []opportunity `json:"cards"`
	CandidatesTitle string        `json:"candidates_title"`
	Candidates      table         `json:"candidates"`
	AllLink         link          `json:"all_link"`
	Chain           *chain        `json:"chain"`
}

type link struct {
	Text   string `json:"text"`
	Target string `json:"target"`
}

type verdict struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Tone  string `json:"tone"`
}

// opportunity is one card; Annualised is a percentAmount or a gap.
type opportunity struct {
	ID             string     `json:"id"`
	Symbol         string     `json:"symbol"`
	Verdict        verdict    `json:"verdict"`
	Reason         string     `json:"reason"`
	Annualised     any        `json:"annualised"`
	AnnualisedText string     `json:"annualised_text"`
	ContractLine   string     `json:"contract_line"`
	LiquidityLine  string     `json:"liquidity_line"`
	Tags           []string   `json:"tags"`
	SizeLine       string     `json:"size_line"`
	SizeUSDApprox  *usdAmount `json:"size_usd_approx"`
	IvLine         string     `json:"iv_line"`
	PremiumLine    string     `json:"premium_line"`
}

type usdAmount struct {
	Value  json.Number `json:"value"`
	Text   string      `json:"text"`
	Unit   string      `json:"unit"`
	Source *string     `json:"source"`
}

type percentAmount struct {
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

type countAmount struct {
	Value int64  `json:"value"`
	Text  string `json:"text"`
	Unit  string `json:"unit"`
}

// textCell is a text cell; Value is a string, an integer sort key or nil.
type textCell struct {
	Kind  string `json:"kind"`
	Text  string `json:"text"`
	Value any    `json:"value"`
	Badge *bool  `json:"badge,omitempty"`
}

type column struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Align string  `json:"align"`
	Hint  *string `json:"hint"`
}

// cellAt is the cell of one column; Cell is one of the amounts, a textCell or a gap.
type cellAt struct {
	Column string `json:"column"`
	Cell   any    `json:"cell"`
}

type row struct {
	ID    string   `json:"id"`
	Cells []cellAt `json:"cells"`
	Quote quote    `json:"quote"`
}

type table struct {
	Columns []column `json:"columns"`
	Rows    []row    `json:"rows"`
}

type chain struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Table   table  `json:"table"`
}

// stockPayload is page.stock_option_page.v3. Stale is null when the data is fresh.
type stockPayload struct {
	Schema        string         `json:"schema"`
	SchemaVersion int            `json:"schema_version"`
	RunID         string         `json:"run_id"`
	BuildID       string         `json:"build_id"`
	Scope         string         `json:"scope"`
	GeneratedAt   string         `json:"generated_at"`
	DataTimes     []sourceTime   `json:"data_times"`
	RunStatus     string         `json:"run_status"`
	Title         string         `json:"title"`
	Stale         *staleLabel    `json:"stale"`
	Top           topBar         `json:"top"`
	Notice        *string        `json:"notice"`
	SharedGap     *string        `json:"shared_gap"`
	Maps          []priceMap     `json:"maps"`
	Matrix        matrixBlock    `json:"matrix"`
	Cards         []chosenCard   `json:"cards"`
	NoCard        *string        `json:"no_card"`
	Held          []heldItem     `json:"held"`
	Contracts     contractsBlock `json:"contracts"`
	Footer        string         `json:"footer"`
	Refresh       refreshObject  `json:"refresh"`
	VolView       volView        `json:"vol_view"`
}

// volView is the name-level volatility block of the stock page; it is never null.
type volView struct {
	Name     string   `json:"name"`
	Method   string   `json:"method"`
	Rows     []volRow `json:"rows"`
	Coaching string   `json:"coaching"`
}

// volRow is one line of the view: Text is the stored decimal of a ready row, Gap the reason code of a gap row.
type volRow struct {
	Label   string `json:"label"`
	Text    string `json:"text"`
	DataDay string `json:"data_day"`
	Gap     string `json:"gap"`
}

type topBar struct {
	Symbol        string  `json:"symbol"`
	Name          *string `json:"name"`
	Spot          *string `json:"spot"`
	DayChange     *string `json:"day_change"`
	DayTone       *string `json:"day_tone"`
	Quote         *string `json:"quote"`
	IV            *string `json:"iv"`
	Earnings      *string `json:"earnings"`
	Trend         string  `json:"trend"`
	Policy        string  `json:"policy"`
	RefreshExpiry *string `json:"refresh_expiry"`
	RefreshStrike *string `json:"refresh_strike"`
}

type matrixExpiry struct {
	Expiry string `json:"expiry"`
	Text   string `json:"text"`
}

type matrixCell struct {
	Key       string `json:"key"`
	Expiry    string `json:"expiry"`
	Strike    string `json:"strike"`
	YieldText string `json:"yield_text"`
	Premium   string `json:"premium"`
	Delta     string `json:"delta"`
	Volume    string `json:"volume"`
	Shade     string `json:"shade"`
	Star      bool   `json:"star"`
	Out       bool   `json:"out"`
	Thin      bool   `json:"thin"`
	Earnings  bool   `json:"earnings"`
	Chosen    bool   `json:"chosen"`
	Quote     quote  `json:"quote"`
}

type matrixBlock struct {
	Expiries []matrixExpiry `json:"expiries"`
	Strikes  []string       `json:"strikes"`
	Cells    []matrixCell   `json:"cells"`
	Facts    *string        `json:"facts"`
	Legend   string         `json:"legend"`
	Empty    *string        `json:"empty"`
}

type pair struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

type chosenCard struct {
	Key       string   `json:"key"`
	Expiry    string   `json:"expiry"`
	Strike    string   `json:"strike"`
	Chosen    bool     `json:"chosen"`
	InMatrix  bool     `json:"in_matrix"`
	Title     string   `json:"title"`
	PassTag   bool     `json:"pass_tag"`
	YieldLine string   `json:"yield_line"`
	Ladder    []pair   `json:"ladder"`
	Advice    string   `json:"advice"`
	Amounts   []string `json:"amounts"`
	Risk      string   `json:"risk"`
	Liquidity string   `json:"liquidity"`
}

type mapMark struct {
	Kind  string `json:"kind"`
	Price string `json:"price"`
	Text  string `json:"text"`
}

type mapBand struct {
	Kind string `json:"kind"`
	Low  string `json:"low"`
	High string `json:"high"`
	Text string `json:"text"`
}

type priceMap struct {
	Expiry      string    `json:"expiry"`
	Heading     string    `json:"heading"`
	Description string    `json:"description"`
	Marks       []mapMark `json:"marks"`
	Bands       []mapBand `json:"bands"`
}

type progress struct {
	Done string  `json:"done"`
	Mark *string `json:"mark"`
	Note string  `json:"note"`
}

type heldItem struct {
	Headline string    `json:"headline"`
	Detail   string    `json:"detail"`
	Account  string    `json:"account"`
	Earned   *string   `json:"earned"`
	Progress *progress `json:"progress"`
}

type contractLine struct {
	Expiry     string   `json:"expiry"`
	ExpiryText string   `json:"expiry_text"`
	Cells      []string `json:"cells"`
	Quote      quote    `json:"quote"`
}

type contractsBlock struct {
	Columns []string       `json:"columns"`
	Rows    []contractLine `json:"rows"`
	Basis   string         `json:"basis"`
	Notes   []string       `json:"notes"`
	Empty   *string        `json:"empty"`
}
