package underlyings

// zhNoName is the Name of a row with no stored name: a gap with its reason, never the code. The page schema
// requires a non-empty name.
const zhNoName = "未存储名称 (No name is stored)"

// The Chinese words of the explanations, warnings and empty states of the page. readOnlyText and heldReadText stay
// English for now: cmd/ot tests assert them; translating them needs a cmd/ot issue that unpins those tests first.
const (
	readOnlyText  = "Editing is off: start ot serve with --owner-edit to change names."
	refillText    = "分组关闭期间错过的日子无法补回：跳过那天的日线历史以后不会再取。"
	followedNone  = "没有分组在跟踪"
	followedPref  = "跟踪它的分组："
	pendingText   = "待定：收盘后解析"
	staleText     = "待定：解析任务还没有处理这个名称；请检查 ot scheduler"
	noNamesText   = "0 个名称。"
	heldReadText  = "Held names could not be read from the positions; held names may be missing here"
	noteHeldWaits = "已加入：持仓，等待解析"
)
