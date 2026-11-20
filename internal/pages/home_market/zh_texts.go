// zh_texts.go holds the Chinese hint of the Index IV column (docs/design/market-iv-rv20.md, Decision 15):
// string constants only, the one door of the English guard of cmd/ot.
package home_market

const indexIvHint = "指数隐含波动率：标普500用VIX，纳斯达克100用VXN，罗素2000用RVX，取FRED已存收盘值（不是期权实时报价）；标签里是数据日。"
