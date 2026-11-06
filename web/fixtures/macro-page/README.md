# macro_page fixtures

Invented data only (see the docstring of `viewmodels/macro_page.py`). The files are written by
`scripts/gen_viewmodel_schema.py` (`pnpm gen:viewmodels`); CI fails on any difference. This README is
hand-written.

`ok.json`: the ease gauge on the stored 0 to 100 scale (reading `中性，较上周 −0.2。`), the events of this
week and next, and three indicators: 美国 CPI 同比 (及时), 中国 PMI (缺, reason `数据源未接入`), 美国失业率 (迟到).

`stale.json`: the stale label. `gap.json`: the ease is a gap and shows no gauge.
