# household risk: the family view of /household shows the cash the family needs if every short put is assigned, against the cash each account holds, and the company each dollar of exposure sits in

Author: the optdesk designer (draft 2026-11-21)
Owner ground: owner goal "value investor + option seller" (what the family is exposed to when it sells puts); owner UI doc, household pages, risk part
Date: 2026-11-21

Source: the main of 2026-11-21. `/household` of a book of two or more accounts is built by `household_page.BuildFamily` (`internal/pages/household_page/family.go`). Its ten detail blocks are the gap `not_built` (`detailGaps`), among them `obligations`, `cash_after`, `stress` and `concentration`. The page of one account already builds those four: `Config.obligations` replays the option ledger and returns the open short puts (`blockapi.ObligationIn`: strike, multiplier, contracts, premium), `pagefig` `household` gives `obligations` (gross put notional per put), `cashAfter` (`TotalCashValue` minus the gross notional of all short puts) and `stress` (cash after all assignments; the -10/-20/-30 % steps are the gap `missing_quote`), and `holdings.go` builds `concentration` from the stock marks. `readBook` already reads the capture of every account and the one USDJPY rate; `heldLines` (`held.go`) already gives each account's stock lines in USD from the stored marks (`mark.Reader`, at most 5 days old). Schema `page.household_page.v1` already has these four blocks (`MoneyBlock`, rows with `scope`, `share`, `note`; at most 60 rows, at most 2 totals), so the payload needs no change.

Rules carried: the front end renders data and computes nothing; Go computes; charts are recharts through the shadcn chart component (this design draws a table, no chart); hints and labels are Chinese and a term is explained next to it; no ticker and no account id in this file; a page states the result the owner sees.

## Today

Stored and enough for this design: per account the newest capture (`TotalCashValue`, `NetLiquidation`), the open short puts of the replayed ledger (strike x multiplier x contracts, currency), the stock positions and their stored closing marks, the USDJPY rate. The page of one account shows the four blocks; the family view shows "家庭合计页不拆分这一块，请在单个账户页查看" in each. The owner therefore cannot see, in one place, how much cash the whole family would have to pay if every short put were assigned, nor which accounts could not pay, nor which company the family would end up most exposed to.

Not stored (so not built; see Left out): a beta per company (no column in any migration), a sector the pages can read (`master.stock.sector_fund` exists but no reader or page uses it and its source is not documented), an underlying price for the -10/-20/-30 % steps (the page of one account has the same gap), margin requirements per account, and the delta of a family position (option greeks are stored per contract and used by `/risk` per account; a family sum is a separate row).

## What the owner sees

On `/household` (family view) the four blocks below stop saying "not built". Each title starts with "家庭风险". Example; all names and numbers are made up:

    家庭风险 · 期权义务 Option obligations
    每张未平仓卖出看跌（short put）一旦被指派，要付多少现金买入股票：行权价 x 每张股数 x 张数，按公司合并，美元。
    AAA 卖出看跌    18,000    账户（main）、账户（wife）；权利金 740
    BBB 卖出看跌    10,000    账户（main）
    合计            28,000

    家庭风险 · 指派后现金 Cash after assignment
    各账户现金减去本账户全部卖出看跌被指派所需现金，负数是缺口。现金分放在各账户，不能互相担保。
    账户（main）    现金 22,000 − 需要 18,000 = 4,000
    账户（wife）    现金 6,000 − 需要 10,000 = −4,000   缺口：现金不够
    合计            现金 28,000 − 需要 28,000 = 0       有 1 个账户现金不够

    家庭风险 · 压力测试 Stress test
    全部卖出看跌都被指派的情形（跌幅 10/20/30 % 的档位还没有，因为没有存下标的最新价）。
    全部指派所需现金   28,000   占全家净值 8.2 %
    全家现金           28,000   占全家净值 8.2 %
    指派后全家现金          0   占全家净值 0.0 %

    家庭风险 · 集中度 Concentration
    若全部被指派，每家公司占全家净值几成：股票市值 + 被指派要买入的金额（行权价计）。
    AAA    股票 30,900 + 指派 18,000 = 48,900   14.4 %
    BBB    股票      0 + 指派 10,000 = 10,000    2.9 %
    注：期权是现金结算的指数期权不会交割股票，这里仍按行权价全额计入，数字偏高。

Rules the owner can read off the page:

1. A put is counted at its full strike amount (gross), not at the margin the broker holds. Premium received is shown in the note, never subtracted.
2. "指派后现金" is per account: an account's cash secures only its own puts (the page already says this in a note). The family sum is shown for orientation and the sentence "有 N 个账户现金不够" is the risk; a family sum above zero does not hide an account below zero.
3. A yen put is converted with the one USDJPY rate of the page (`FamilyResult.FxRate`); without a rate the figure of that put is the gap `unsupported_currency` and every sum it is part of is the same gap, never a partial number.
4. An account the family total does not count (no data, stale, read error) is not in any of the four blocks; the lead notes of the page already name it, and the blocks say "不含 N 个账户" with that count.
5. An account whose ledger or positions cannot be read is a named gap row in the blocks it affects; the other accounts still show. A sum that left an account out says so in its note; it is never shown as complete.
6. Concentration counts stocks at the stored closing mark (as the held companies page) and puts at the strike. A stock with no mark makes that company's stock part the gap `missing_quote`, and its exposure is the gap; the company row stays, no zero is written.
7. Share is the exposure over the family net total of the page (all counted accounts, USD), the same denominator as the held companies page.
8. No percent or amount is computed by the page builder or the web: they are copied from the figure of entry 1.

## Decisions

1. Fill the four existing blocks of the family view instead of adding a new block or page. Reason: schema `page.household_page.v1` already has them with the right shape, the web already renders them, and the one-account page already has the same four titles; the owner reads one set of words. The "家庭风险" heading is the prefix of the four block titles.
2. One new figure `FamilyRiskFigure` in `blockapi` (entry 1) takes every counted account's cash, open short puts and the stock lines and returns the four blocks' lines. Reason: Go computes, exact decimal arithmetic, and it is a figure of its own (like `FamilyFigure`, `HeldFigure`), so `PageFigures` and its callers do not change.
3. Same arithmetic as the page of one account: gross notional = strike x multiplier x contracts, premium in the note. Reason: the family row of an account then equals that account's own page (acceptance of entry 1 and 2 test it).
4. Cash is per account; the family sum is not a shared pool (Rule 2). Reason: `familyCashNote` already states that cash secures only its own account; showing only a family sum would hide the one real danger, one account that cannot pay.
5. Concentration key is the underlying symbol (as `HeldFigure`: one row per symbol and currency), not the issuer. Reason: the one rule the held companies page already uses; merging by issuer is a separate change to both pages.
6. Stock lines come from `Config.heldLines` (the same stored marks as the held companies page, newest mark up to 5 days back), so the two pages never disagree on a stock value.
7. `Config.FamilyRisk blockapi.NewFamilyRiskFigureFunc`, nil keeps today's gap `not_built` for all four blocks. With `Marks` or `Holdings` nil only `concentration` is the gap `no_reader`. Reason: the same switch as every other family block (`Config.Family`, `Config.FamilyTrend`); the field and its wiring in `cmd/ot/household_site.go` land in one entry.
8. No store, dataset, migration, schema or job change. The family view reads each counted account's positions and replays its ledger once more than today (the page of one account does the same per request); the cost is stated in the Adversarial section.
9. The -10/-20/-30 % steps stay the gap `missing_quote`, as on the page of one account. Reason: no stored underlying price source in this design; the stress block says so in words.
10. `book_risk` (the stored stress of `strategy.result`, rows `household:<currency>`) is not read here. Reason: it is written 45 minutes after the close by another job and counts cash-settled index options differently; two numbers for "if assigned" on one page is the confusion this design avoids. Left out.

## Modules

`docs/design/modules.md` is not changed. Paths match the rows compute, pages, ot and web; the first matching row owns each path. No store, dataset, migration or `contracts/` change; the auto-deploy is not held.

- compute (entry 1): `internal/compute/blockapi/api.go` (521 lines), `internal/compute/blockapi/api_test.go`, new `internal/compute/pagefig/family_risk.go` and `family_risk_test.go`, `docs/contracts/compute.md`.
- pages and ot (entry 2, one judged two-module entry, because the site wiring is one line and must land with the field it sets): in `household_page`, new `family_risk.go` and `family_risk_test.go`; edited `family.go` (290 lines), `page.go` (Config), `zh_texts.go` (63 lines), `docs/contracts/pages.md`; in ot, edited `cmd/ot/household_site.go` (88 lines) and `cmd/ot/household_site_test.go`.
- web (entry 3): edited `web/fixtures/household-page/family.json`, `web/test/pages/household/householdFamily.test.tsx`. The blocks and their sections already render; no web source file changes unless the test shows a block that cannot show a row `note` (then one display field, still no computation). The two web files are shared with entry 3 of `docs/design/net-value-trend.md`; if that entry is still open, this entry lands after it.

No file listed is over 600 lines after the change; none is in `docs/design/ratchet.json`.

## Interface

In `internal/compute/blockapi/api.go`, declarations only (the implementation is `pagefig.NewFamilyRisk`):

    // FamilyRiskAccountIn is one counted account: its cash in USD (FamilyAccountOut.Cash), its open short puts and
    // whether its ledger was read. Alias is never the account id.
    type FamilyRiskAccountIn struct {
        Alias       string
        Cash        compute.FigureValue
        Obligations []ObligationIn
        Read        bool // false: the ledger or the positions of the account could not be read
    }
    // FamilyRiskIn: the counted accounts, the stock lines of all of them (USD, as HeldLineIn), the family net in USD
    // and the USDJPY rate (empty without one).
    type FamilyRiskIn struct {
        Accounts []FamilyRiskAccountIn
        Stocks   []HeldLineIn
        Net      compute.FigureValue
        UsdJpy   string
    }
    // FamilyRiskPut is one company's short puts of the family: the assignment cash in USD, the aliases that hold them.
    type FamilyRiskPut struct {
        Underlying string
        Cash       compute.FigureValue
        Aliases    []string
        Premium    string // USD text, "" when unknown
    }
    // FamilyRiskAccountOut is one account: cash, the cash its puts need, cash after (negative is a shortfall).
    type FamilyRiskAccountOut struct {
        Alias                string
        Cash, Need, CashAfter compute.FigureValue
        Short                bool // CashAfter is below zero
    }
    // FamilyRiskCompany is one company of the concentration: stock value, put cash, their sum and its share of Net.
    type FamilyRiskCompany struct {
        Underlying                            string
        Stock, Put, Exposure, Share           compute.FigureValue
    }
    // FamilyRiskResult. Puts and Companies are sorted by amount descending, a gap row last, ties by Underlying, at
    // most 60 rows each; Hidden counts the rows cut. NeedTotal, CashTotal and AfterTotal are the gap of the first
    // missing part, never a partial sum. ShortCount is the number of accounts with Short.
    type FamilyRiskResult struct {
        Puts                            []FamilyRiskPut
        Accounts                        []FamilyRiskAccountOut // input order
        Companies                       []FamilyRiskCompany
        NeedTotal, CashTotal, AfterTotal compute.FigureValue
        NeedShare, AfterShare           compute.FigureValue // of Net, percent
        ShortCount, Hidden              int
    }
    type FamilyRiskFigure interface {
        // FamilyRisk is pure: no I/O, no clock, no float.
        FamilyRisk(in FamilyRiskIn) FamilyRiskResult
    }
    type NewFamilyRiskFigureFunc func() FamilyRiskFigure

It is not a method of `FamilyFigure`, `HeldFigure` or `PageFigures`, so their callers are unchanged.

## Issues

Build order: entry 1 first (interface); entries 2 and 3 depend only on it (3 needs 2 only to see the blocks on a live page, not to compile). Every entry says `Contracts: unchanged`.

### 1 hrisk-1-figure (interface)

Module: compute. Order: first. Files: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `internal/compute/pagefig/family_risk.go`, `internal/compute/pagefig/family_risk_test.go`, `docs/contracts/compute.md`.

### 2 hrisk-2-page

Module: pages and ot. Order: after 1. Files: `internal/pages/household_page/family_risk.go`, `internal/pages/household_page/family_risk_test.go`, `internal/pages/household_page/family.go`, `internal/pages/household_page/page.go`, `internal/pages/household_page/zh_texts.go`, `docs/contracts/pages.md`, `cmd/ot/household_site.go`, `cmd/ot/household_site_test.go`.

### 3 hrisk-3-web

Module: web. Order: after 1. Files: `web/fixtures/household-page/family.json`, `web/test/pages/household/householdFamily.test.tsx`.

## Left out

- Beta-weighted exposure: no beta is stored for any company; a source and a table are a separate design.
- Delta of the family: option greeks are stored per contract (`option_eod`) and `/risk` shows delta per account; a family sum needs that page's loader in this package. Later row.
- Concentration by sector: `master.stock.sector_fund` has no reader and no documented source; one wrong sector would mislead more than none.
- The -10/-20/-30 % stress steps and the stored `book_risk` worst loss (Decision 10): need a stored underlying price source or a decision on which "if assigned" number the page keeps.
- Margin and buying power per account (what the broker holds back): not stored, and `TotalCashValue` is the only stored cash.
- Netting a put against stock the family already holds in the same company: the page shows both parts of the exposure, it does not net.
- A chart of the concentration: the table with share is enough for the owner row; a bar chart is a display change for later.
- Merging companies by issuer (two listings of one company): both pages use symbol and currency.

## Checked by CI

- `ci-go`: `go build ./...`, `go vet ./...`, `go test ./internal/compute/... ./internal/pages/... ./cmd/ot/...`.
- `ci-web`: `pnpm install --frozen-lockfile`, `tsc`, `eslint`, `vitest`, build.

## After merge (checked by the dispatcher, not by review)

On the live `/household` the four blocks show rows; the "合计" of `期权义务` equals the sum of the same figure on the pages of the single accounts (USD accounts), and the account row of `指派后现金` equals that account's own page.

## Adversarial

Run as a fresh reader on 2026-11-21.

1. Gap vs goal. The ask names assignment exposure, concentration by company or sector, and beta-weighted or delta exposure. Built: assignment cash against cash (per account and family), concentration by company with the assigned amount. Not built: sector, beta, delta, each in Left out with the missing stored fact. The part of the goal most likely wanted next is delta, because it is stored; it is a separate row only because the loader is in another package.
2. Biggest unnamed risk: the number overstates what the broker will ask. Gross strike amount ignores margin offsets, ignores that a put on an index is cash settled, and the page of one account does the same. Adopted: the index note on the concentration block, the "gross, not margin" rule on the page (Rule 1), and the family figure equals the account page by test, so the family page is never the odd one out. Not adopted: reading the class table of `book_risk` (`broad_high`, `broad_other`) to drop index puts, because that table lives in a job manifest and a second copy would drift.
3. Simpler way: read the stored `book_risk` rows (`household:USD`) instead of a new figure. Not adopted (Decision 10): it is written 45 minutes after the close, may be absent on a day, has per-class worst losses and not "cash needed", and counts index options differently. The new figure is about 150 lines on inputs the page of one account already reads.
4. Lock-in: the interface fixes four lines of output per account and company in USD text and a 60-row cap (the schema's cap). A later delta or sector column adds fields to `FamilyRiskCompany`, not a rewrite. The real lock-in is the schema cap of 60 rows and 2 totals: a family with more than 60 companies shows `Hidden` in a note, and raising the cap is a contracts change.
5. Claim most likely false: "the family row of an account equals that account's own page" (Decision 3). The family cash is `FamilyAccountOut.Cash`, converted to USD with the family's one rate, while the account page keeps the account currency and converts only a note; for a yen account the two differ by rounding. Adopted: the test for equality uses a USD account; for a yen account the test checks the converted figure against the exact decimal of the rate, and the acceptance does not claim equality there. A second candidate: "stock lines use the same marks as the held companies page": true only while both call `heldLines`; the test calls both.
6. Likeliest slip: the page builder adding the put cash and stock value itself because the sum is one line (the package purity test forbids `math/big`, so it would show as string arithmetic or a float). Adopted: acceptance of entry 2 requires the builder to call the figure and `purity_test.go` to pass unchanged. Second slip: reading the ledger and positions of every account on each request makes the page slower than the family page of today; the held companies page already reads positions for each account, so the added cost is the ledger replay per account, which the page of one account does per request. If the page becomes slow, caching is a separate issue, not a reason to drop the blocks.
