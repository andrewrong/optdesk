package report_page

const (
	// jpSuffix follows the block title in the text of a jp_slice_missing Gap.
	jpSuffix = "的日本时段数据尚未接入"
	// reasonJPMissing is the Gap reason of a block of the Japan report.
	reasonJPMissing = "jp_slice_missing"
)

type gap struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

// gapBlock is the common part of a block that has no producer: state gap, one not_built Gap.
type gapBlock struct {
	Key       string `json:"key"`
	Title     string `json:"title"`
	Say       string `json:"say"`
	Sub       string `json:"sub"`
	DataTimes string `json:"data_times"`
	State     string `json:"state"`
	Gaps      []gap  `json:"gaps"`
}

func notBuilt(t blockText) gap { return gap{reasonNotBuilt, t.title + gapSuffix} }

// jpMissing is the Gap of a block of the Japan report, which has no slice yet.
func jpMissing(t blockText) gap { return gap{reasonJPMissing, t.title + jpSuffix} }

func newGapBlock(t blockText, g gap) gapBlock {
	return gapBlock{
		Key: t.key, Title: t.title, Say: t.say, Sub: t.sub,
		DataTimes: noDataTimes, State: "gap", Gaps: []gap{g},
	}
}

type actionsBlock struct {
	gapBlock
	Trades gap `json:"trades"`
}

type accountMovesBlock struct {
	gapBlock
	Accounts gap `json:"accounts"`
	Movers   gap `json:"movers"`
}

type marketWatchBlock struct {
	gapBlock
	Indices   gap `json:"indices"`
	Companies gap `json:"companies"`
}

type globalAssetsBlock struct {
	gapBlock
	Proxies   gap    `json:"proxies"`
	Sector    gap    `json:"sector"`
	ChartAria string `json:"chart_aria"`
}
