package company_quarters_page

// Chinese texts of the page (owner rule 2026-11-17): constants only. Labels, column names and terms (SEC, cik, EPS, FY) stay English.

const (
	missingGeneric = "这个季度没有存下这项数据。"

	noteNoLabel = "财年标签（例如 FY2025 Q3）要等下一次每日基本面任务重新抓取 SEC 财报后才会出现；在此之前，季度以其截止日期命名。"
)

const (
	textUnknownCompany = "还没有选公司，或者这个名字不在列表里；请从下面选一个。"
	textNoCIK          = "这家公司没有 SEC 编号（cik）；请先在标的页填写，才能显示它的季度。"
	textJP             = "日本公司的财报被有意略去，没有接入对应的数据来源。"
	textNoStatements   = "已存的 SEC 财报里还推算不出任何一个季度。"
	textReadFailed     = "读取已存的数据失败，所以不显示季度。"
)
