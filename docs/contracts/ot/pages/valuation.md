# ot pages `valuation_params`, `valuation_table`, `valuation_company`, `valuation_form`

Orders: 140, 150, 160, 170 (after `data_health_page`, 130).

Routes and names for `ot page`:

| Route | Name | Schema | View |
| --- | --- | --- | --- |
| `valuation-params` | `valuation_params` | `page.valuation_params_page.v1` | market parameters (R, AI switch, R and margin tables) |
| `valuation` | `valuation_table` | `page.valuation_page.v1` | `table`: every name on `settings.underlying` united with the profiles |
| `valuation-company` | `valuation_company` | `page.valuation_page.v1` | `company`: price band, lines, review history |
| `valuation-form` | `valuation_form` | `page.valuation_page.v1` | `form`: the entry form of a review or a profile |

Family file: `cmd/ot/valuation_site.go`; registered with `registerPage` in an `init()` (`cmd/ot/pagereg.go`). The common rules of every page are in `docs/contracts/ot.md`. Design: `docs/design/valuation-structure.md`, entry 7.

## Entering a valuation the owner already holds

A valuation the owner already holds in his own table is entered as review type `first`. The type `backfill` is only for an earlier version of a company that has a later one; a `backfill` is never current.

## Lines it reads

- `OT_PG_DSN_PAGES`: the views of `internal/store/valuation` (`valuation.Reader`), opened through `siteOpeners`. With no line (or the demo profile) there is no reader and each page is a gap, never a 500; a failed read is the same gap.
- `settings.underlying` through `rd.underlying`: the list of names and their `Key`.
- The newest stored `DGS10` point of the last 14 days: the yield of the params page.
- The close price of a company: the stock id reader of `market_state_page` and the bar reader (`rd.bars`); no close when either is missing.

## Query parameters (GET)

- `company=<key>` on `valuation-company` and `valuation-form`: the company; an unknown key gives an empty page with a gap.
- `saved=1`: shows "Saved." to the owner only.
- `warned=1` (params page): the saved R moves a market by more than `RStepWarn`.
- `review=<id>`: the review just written; the company page does not read it, it is for the form to keep.

## Post paths

Served only when owner edit is on (`rd.edit != nil`); with it off the path is not handled. Every post passes `guardOwnerPost` (cookie, content type, size) before the store is opened; a post without the token writes nothing. The writer is opened per post with the settings line (`valuation.OpenFunc`, `valuationpg.Open`).

### `POST /valuation-post` (form `valuation-form`)

Fields: `action`, `company` (a key chosen) or `key` (a free key typed), `review_id` (kept by the form across the drafts of one record; made when absent), `type` (`first`, `post_earnings`, `event`, `backfill`), `version_date`, `v`, `p`, `trigger` (`a` to `d`), `fiscal_period_end`, `report_date`, `earnings_base`, `base_basis`, `forward_factor`, `locked_multiple`, `method_1` to `method_3`, `reason_<dim>`, the text fields of the review, `void_reason`, and for a profile `name`, `symbol`, `currency`, `r_market`, `pool`, `pool_date`, `note`.

Actions:

- `preview`: writes nothing; answers the form document as JSON when `Accept` asks for `application/json`, else the page.
- `draft`: inserts a review with status `draft`.
- `effective`: runs `Calculator.Check`. With no problem inserts a review with status `effective` and `source` `owner`. Otherwise inserts a `draft` and answers 422 with the problems listed.
- `void`: inserts a void row of an effective review; `void_reason` and `review_id` are required.
- `profile`: inserts a profile row; refuses a typed `key` equal to a `Key` on `settings.underlying`.

The first save of a company with no profile inserts it first (defaults: from its row on `settings.underlying`, else the typed fields; the R market from the key prefix `US:`/`JP:`, else chosen; pool `in_pool`), then the review. A second save inserts no profile.

422 (the form is re-rendered, the text is in `error`): unknown action; `company: required`; key not on the list form or equal to a list key; `review_id: not an id`; a type, trigger or date with no valid form; `r_market` not chosen; a save of a review that is already effective (`ErrClosed`); a void of a review that is not effective (`ErrNotEffective`); a missing void reason; `effective` whose check has problems (saved as draft).

303: a review or void to `/valuation-company?company=<key>&saved=1&review=<id>`; a profile to `/valuation-form?company=<key>&saved=1`.

### `POST /valuation-params-post` (form `valuation-params`)

Fields: `action`, `effective_date` (required, `YYYY-MM-DD`), and per action:

- `r_table`: `table`, one bucket per line, `yield_up_to us jp hkcn` (commas, semicolons or blanks separate the fields); the top bucket has no `yield_up_to`, gives three fields and is last. One table version is inserted.
- `margin_table`: `table`, one bucket per line, `p_from m w h s`. One table version is inserted.
- `accept_r`: `yield`. The R values are recomputed on the server with `ProposeR` from the yield and the stored R table; posted R values are ignored. The AI switch and expiry of the newest version are carried over. A step above `RStepWarn` is saved and warned.
- `r_version`: `r_us`, `r_jp`, `r_hkcn`, `yield_used`, `ai_switch`, `ai_expiry`, `note`. The expiry text is required when the switch is on.

422 (the params page is re-rendered, the text is in `error`): unknown action; `effective_date` missing or not a date; a table with no line, a line with the wrong number of fields, a value that is not a number, an R bucket with no `yield_up_to` that is not last, a `yield_up_to` or `p_from` that does not ascend (the line is named: `line N: ...`); `accept_r` with no yield, no R table or no bucket for the yield; `r_version` with an R that is not a number, or the switch on with no expiry.

303: to `/valuation-params?saved=1`, and `&warned=1` when `accept_r` warned.

A write failure is 500 with the fixed text `internal error`; a method other than POST on a post path is 405.

## Version

v1: introduced with this document (orders 140 to 170).
