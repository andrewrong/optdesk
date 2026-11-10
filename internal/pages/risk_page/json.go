package risk_page

import "encoding/json"

type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

type staleLabel struct {
	DataTime   string `json:"data_time"`
	StaleAfter string `json:"stale_after"`
	Text       string `json:"text"`
}

type gapJSON struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type moneyCell struct {
	Value    json.Number `json:"value"`
	Text     string      `json:"text"`
	Unit     string      `json:"unit"`
	Currency string      `json:"currency"`
}

type vegaFigure struct {
	Value json.Number `json:"value"`
	Text  string      `json:"text"`
	Unit  string      `json:"unit"`
}

type deltaRow struct {
	Name  string    `json:"name"`
	Delta moneyCell `json:"delta"`
}

type deltaJSON struct {
	Title   string     `json:"title"`
	Meaning string     `json:"meaning"`
	Heads   []string   `json:"heads"`
	Rows    []deltaRow `json:"rows"`
	Total   *moneyCell `json:"total"`
	Gap     *gapJSON   `json:"gap"`
}

type vegaRow struct {
	ExpiryText string     `json:"expiry_text"`
	Vega       vegaFigure `json:"vega"`
}

type vegaJSON struct {
	Meaning string      `json:"meaning"`
	Heads   []string    `json:"heads"`
	Rows    []vegaRow   `json:"rows"`
	Total   *vegaFigure `json:"total"`
	Gap     *gapJSON    `json:"gap"`
}

type betaJSON struct {
	Title         string     `json:"title"`
	Meaning       string     `json:"meaning"`
	BenchmarkText string     `json:"benchmark_text"`
	Heads         []string   `json:"heads"`
	Rows          []struct{} `json:"rows"`
	Total         *moneyCell `json:"total"`
	Gap           *gapJSON   `json:"gap"`
}

type sigmaJSON struct {
	Title    string     `json:"title"`
	Meaning  string     `json:"meaning"`
	Columns  []string   `json:"columns"`
	Rows     []struct{} `json:"rows"`
	AriaText string     `json:"aria_text"`
	Gap      *gapJSON   `json:"gap"`
}

type metric struct {
	Label       string  `json:"label"`
	ValueText   string  `json:"value_text"`
	Explanation string  `json:"explanation"`
	Table       *string `json:"table"`
}

type metricsJSON struct {
	Title   string   `json:"title"`
	Metrics []metric `json:"metrics"`
}

type pageJSON struct {
	Schema          string       `json:"schema"`
	SchemaVersion   int          `json:"schema_version"`
	RunID           string       `json:"run_id"`
	BuildID         string       `json:"build_id"`
	Scope           string       `json:"scope"`
	GeneratedAt     string       `json:"generated_at"`
	DataTimes       []sourceTime `json:"data_times"`
	RunStatus       string       `json:"run_status"`
	Title           string       `json:"title"`
	Sentence        string       `json:"sentence"`
	VegaTitle       string       `json:"vega_title"`
	Delta           deltaJSON    `json:"delta"`
	Vega            vegaJSON     `json:"vega"`
	BetaDelta       betaJSON     `json:"beta_delta"`
	Sigma           sigmaJSON    `json:"sigma"`
	StandardMetrics metricsJSON  `json:"standard_metrics"`
	Notes           []string     `json:"notes"`
	Gaps            []gapJSON    `json:"gaps"`
	Stale           staleLabel   `json:"stale"`
}

func betaGap(g gapJSON) betaJSON {
	return betaJSON{Title: titleBeta, Meaning: meaningBeta, BenchmarkText: benchmarkText,
		Heads: []string{headName, headBeta, headWeighted}, Rows: []struct{}{}, Gap: &g}
}

func sigmaGap(g gapJSON) sigmaJSON {
	return sigmaJSON{Title: titleSigma, Meaning: meaningSigma, Columns: []string{"-2σ", "-1σ", "+1σ", "+2σ"},
		Rows: []struct{}{}, AriaText: sigmaAria, Gap: &g}
}

// metricsGap: the schema's block has no gap field and needs one metric, so the one metric says the figure is
// missing; the page-level gaps carry the reason.
func metricsGap(g gapJSON) metricsJSON {
	return metricsJSON{Title: titleMetrics, Metrics: []metric{{Label: labelMetricsMissing, ValueText: textMetricsMissing, Explanation: g.Text}}}
}
