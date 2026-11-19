package strategy_replay_page

// The visible Chinese texts of the strategy replay page (docs/design/strategy-replay-page.md Decisions 8, 9
// and 14). String constants only; the builder copies them and composes nothing else.
const (
	textTitle        = "策略回放"
	textIntro        = "用已存的报价回放卖出看跌的持有到期与早平两种做法，对比他实际的结果。"
	textSubmit       = "开始回放"
	textRestartHint  = "中途被打断的回放不留任何东西，再按一次即可"
	textLotsShortPut = "卖出看跌的单子"
	textLotsShortLot = "卖出的手数"
)

// The labels and hints of the six fields. Each technical word has its Chinese gloss beside it.
const (
	labelLots      = "取单范围"
	labelAlias     = "账户别名"
	labelAsOf      = "截止日"
	labelEarlyKeep = "早平留存比例"
	labelEarlyDTE  = "早平剩余天数"
	labelMinLots   = "最少笔数"

	hintLots      = "卖出看跌（short puts）按单笔期权计，或按手数（short lots）计；持有到期（hold）：不提前平仓"
	hintAlias     = "留空表示所有已存账户"
	hintAsOf      = "格式 YYYY-MM-DD，留空表示今天"
	hintEarlyKeep = "早平（early）：权利金赚到一定比例或离到期不足若干天就买回；留存比例（kept share）：赚到的权利金占收到的权利金的比例；默认 0.50 [T]：试用值，不是定论"
	hintEarlyDTE  = "早平（early）：离到期不足若干天就买回；默认 21 [T]：试用值，不是定论"
	hintMinLots   = "少于这个笔数的行不下结论；默认 20 [T]：试用值，不是定论"
)

// The notice of each key.
const (
	noticeFormOff      = "回放未开启：缺少所有者令牌或设置库配置，设置后重启 ot serve"
	noticeTimedOut     = "回放超时，已中止；缩小范围后再按一次"
	noticeBusy         = "已有一次回放在运行，请等它结束后再按"
	noticeReadFailed   = "读取数据失败，请检查服务环境里的四行数据库设置"
	noticeBadLots      = "取单范围不对，请从列表里选一项"
	noticeBadAsOf      = "截止日不对，请按 YYYY-MM-DD 填写"
	noticeBadEarlyKeep = "早平留存比例不对，请填 0 到 1 之间的小数"
	noticeBadEarlyDTE  = "早平剩余天数不对，请填不小于 0 的整数"
	noticeBadMinLots   = "最少笔数不对，请填不小于 0 的整数"
)

// The four sentences of the notes of the report.
const (
	noteAssignment = "不模拟提前被指派。"
	noteGap        = "没有存报价的日子是缺口，不是猜测。"
	noteSnapshot   = "存的报价是开盘后 90 分钟的上午快照，结算用收盘价。"
	noteRolled     = "展期单的实际结果不含它展期进去的那一笔。"
)

// The pieces of minimum_text and empty_text.
const (
	minimumHead = "不足 "
	minimumTail = " 笔 [T]：无结论"
	emptyHead   = "数据不足，无结论："
	emptyLots   = " 笔，已存报价 "
	emptyDays   = " 天，首尾报价日："
	emptyTo     = " 至 "
	emptyNone   = "无"
)

// The hints of the count keys of the report, the Chinese text and the English word the web prints.
const (
	hintCountLots          = "取到的单子数（lots）"
	hintCountLotsWithQuote = "其中有已存报价的单子数（lots with quote）"
	hintCountStillOpen     = "还没到期的单子数（still open）"
	hintCountLotDays       = "所有单子持有的天数之和（lot days）"
	hintCountQuoteDays     = "有已存报价的天数（quote days）"
)

// The hints of the columns of the lines table.
const (
	hintColSet             = "分组（set）：全部账户或某个账户别名"
	hintColVariant         = "做法（variant）：持有到期（hold）或早平（early）"
	hintColCurrency        = "币种（currency）"
	hintColLots            = "单子数（lots）"
	hintColCycles          = "回放的周期数（cycles）"
	hintColClosedEarly     = "提前平仓的笔数（closed early）"
	hintColHeld            = "持有到期的笔数（held）"
	hintColAssigned        = "被指派的笔数（assigned）"
	hintColRolled          = "展期的笔数（rolled）"
	hintColInTheMoney      = "到期时在价内的笔数（in the money）"
	hintColKeptShare       = "留存比例（kept share）：赚到的权利金占收到的权利金的比例"
	hintColMeanGross       = "平均毛收益（mean gross）"
	hintColWorstGross      = "最差一笔的毛收益（worst gross）"
	hintColHoldKeptShare   = "持有到期（hold）的留存比例（hold kept share）：赚到的权利金占收到的权利金的比例"
	hintColActualKeptShare = "他实际的留存比例（actual kept share）：赚到的权利金占收到的权利金的比例"
)

// The hints of the two kinds of variant.
const (
	hintVariantHold  = "持有到期（hold）：不提前平仓"
	hintVariantEarly = "早平（early）：权利金赚到一定比例或离到期不足若干天就买回；留存比例（kept share）：赚到的权利金占收到的权利金的比例；[T]：试用值，不是定论"
)
