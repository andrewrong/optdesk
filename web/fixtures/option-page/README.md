# option_page fixtures

Invented data only (see the docstring of `viewmodels/option_page.py`). The files are written by
`scripts/gen_viewmodel_schema.py` (`pnpm gen:viewmodels`); CI fails on any difference. This README is
hand-written.

`ok.json`: US tab with six opportunity cards (4 值得看, 2 勉强), headline `4 个值得看，2 个勉强。`, eight
candidates (TGT is 跳过 with Bid/Ask/成交量/持仓量 as gap cells), and the AAA chain with the chosen contract
marked 已选; JP tab with two cards whose size line shows JPY and its ≈USD figure.

`stale.json`: the stale label. `gap.json`: one more gap, the Mid of AAA 11/28 at $46.
