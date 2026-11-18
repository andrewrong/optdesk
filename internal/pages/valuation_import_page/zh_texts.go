package valuation_import_page

// The texts of the page. Each technical word has its Chinese hint beside it (the owner's rule,
// docs/design/valuation-import.md Decision 13). String constants only.
const (
	// TextR is the word R with its hint.
	TextR = "R (折现倍数 R)"
	// TextBuyZoneTop is the buy-zone top with its hint.
	TextBuyZoneTop = "buy-zone top (买入区间上沿)"
	// TextFirst is the review type first with its hint.
	TextFirst = "first (首次估值)"
	// TextEvent is the review type event with its hint.
	TextEvent = "event (事件重估)"
	// TextGeneralTick is the general line the owner ticks before the confirm.
	TextGeneralTick = "V is per listed share, in the listing currency (V 按上市股份计,币种为上市币种)"
	// TextOlder is the reason of a row skipped as older than the stored one.
	TextOlder = "older than stored (比库内已有的更旧)"
	// TextUnmatched is the reason shown beside a name that matches no stored underlying.
	TextUnmatched = "unmatched name (未匹配的名称,需自选 key 并勾选)"
)

// The plain sentences of the refusal reasons, as the importer words them, each with its hint.
const (
	ReasonOtherTop = "the site computes another buy-zone top (站点算出的买入区间上沿与文件不同)"
	ReasonHKCN     = "HK and CN differ: the site has one HKCN column (港股与 A 股的 R 不同,站点只有一列 HKCN)"
	ReasonRMarket  = "the row R differs from the R of its market (该行 R 与所属市场的 R 不一致)"
	ReasonBadKey   = "the key picked is not valid (所选 key 不合规)"
)

// textFlagged follows the count of rows with a breakpoint flag other than ok.
const textFlagged = " rows carry a breakpoint flag other than ok (断点标记不是 ok 的行数)"
