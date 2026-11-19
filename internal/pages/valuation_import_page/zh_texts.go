package valuation_import_page

// The texts of the page. Each technical word has its Chinese hint beside it (the owner's rule,
// docs/design/valuation-import.md Decision 13). String constants only.
const (
	// TextR is the word R with its hint.
	TextR = "R (折现倍数 R)"
	// TextBuyZoneTop is the buy-zone top with its hint.
	TextBuyZoneTop = "buy-zone top (买入区间上沿)"
	// TextBuyZoneTopSite is the buy-zone top the site computes, with its hint.
	TextBuyZoneTopSite = "buy-zone top, site (站点算出的买入区间上沿)"
	// TextBuyZoneTopFile is the buy-zone top the file carries, with its hint.
	TextBuyZoneTopFile = "buy-zone top, file (文件中的买入区间上沿)"
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

// The texts of the page that the web printed in English, each with its Chinese hint beside it.
const (
	TextTitle           = "Import valuations (导入估值)"
	TextIntro           = "Pick the export file, read the preview, then pick it again to confirm (选择导出文件,查看预览,再选一次同一文件以确认)"
	TextFile            = "File (文件)"
	TextPreview         = "Preview (预览)"
	TextNotInFile       = "Not in file (不在文件中)"
	TextUnvalued        = "No valuation in the file (文件中没有估值)"
	TextRemoved         = "Removed (已移除)"
	TextCountInsert     = "insert (新增)"
	TextCountSame       = "same (相同)"
	TextCountOlder      = "older (更旧)"
	TextCountRefused    = "refused (拒绝)"
	TextCountNeedsTick  = "needs tick (需勾选)"
	TextStored          = "stored (库内已有)"
	TextOlderThanStored = "older than stored (比库内已有的更旧)"
	TextMarginTable     = "Margin table (安全边际表)"
	TextResult          = "Result (结果)"
	TextSameFileAgain   = "Same file again (再选同一文件)"
	TextConfirm         = "Confirm (确认)"
)

// The plain sentences of the refusal reasons, as the importer words them, each with its hint.
const (
	ReasonOtherTop = "the site computes another buy-zone top (站点算出的买入区间上沿与文件不同)"
	ReasonHKCN     = "HK and CN differ: the site has one HKCN column (港股与 A 股的 R 不同,站点只有一列 HKCN)"
	ReasonRMarket  = "R of the row differs from the R of its market in the file (该行 R 与文件中所属市场的 R 不一致)"
	ReasonBadKey   = "the key picked is not valid or is already on the list (所选 key 不合规或已在清单中)"
)

// The other refusal reasons of the importer and the problems of calc.Check, each with its hint.
const (
	ReasonNoVersion     = "no R version or margin table to compute the lines (没有 R 版本或安全边际表,无法算出各线)"
	ReasonOutOfRange    = "the site cannot compute the lines: V or P is out of range (站点无法算出各线:V 或 P 超出范围)"
	ReasonUnknownMarket = "the list row has a market the import does not know (清单行的市场导入无法识别)"
	ProblemReviewType   = "review type is required (必须填写估值类型)"
	ProblemVersionDate  = "version date is required (必须填写版本日期)"
	ProblemVRequired    = "V is required (必须填写 V)"
	ProblemPRequired    = "P is required (必须填写 P)"
	ProblemVRange       = "V must be a number above 0 (V 必须是大于 0 的数)"
	ProblemPRange       = "P must be from 1 to 5 in steps of 0.5 (P 必须在 1 到 5 之间,步长 0.5)"
	ProblemMarginNumber = "margin table has a value that is not a number (安全边际表有不是数字的值)"
	// ProblemNoRowHint follows "margin table has no row for P <p>".
	ProblemNoRowHint = "(安全边际表缺少该 P 的行)"
	// ProblemRowHint follows "margin row from P <p>: ...".
	ProblemRowHint = "(安全边际表该行:买入区间上线不低于合理区间下线)"
)

// Prefixes of the check problems that carry numbers.
const (
	prefixNoRow = "margin table has no row for P "
	prefixRow   = "margin row from P "
)

// textFlagged follows the count of rows with a breakpoint flag other than ok.
const textFlagged = " rows carry a breakpoint flag other than ok (断点标记不是 ok 的行数)"
