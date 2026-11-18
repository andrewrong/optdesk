package structures_page

// The reasons a multi-leg unit of kind other was made, one line each, written in the note of the page.
const (
	// reasonOppositeSidesExpiries: legs of one right on opposite sides at different expiries.
	reasonOppositeSidesExpiries = "同一方向的期权腿多空相反且到期日不同"
	// reasonPutCallOneExpiry: a put and a call at one expiry.
	reasonPutCallOneExpiry = "同一到期日同时持有认沽和认购"
)

// The texts of the Ex-dividend cell of the list (Decision 7 of docs/design/option-pages-r8.md), set on the gap
// directly: gapTexts holds English.
const (
	exNoReaderText      = "没有除息数据来源"
	exPassedText        = "除息日已过，等待下次采集"
	exNoRowText         = "没有除息预期记录（该股可能不派息，或尚未采集）"
	exNotApplicableText = "不适用：没有卖出认购腿"
	exDividendLookback  = 10 // days before the page day of the dividend rows read
)

// The line of the page note that names the close behind each distance.
const (
	closeNotePrefix = "距现价所用价格："
	closeWord       = "收盘"
	printWord       = "最近成交"
	closeNoteSep    = "；"
)
