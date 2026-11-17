# store/fundamental

Package `internal/store/fundamental` is the fact store of `obs.fundamental_fact` (`docs/design/data-collection.md`, entry 4, Decisions 1 and 15).

## Period columns

Migration `0053_fundamental_fact_period.sql` adds three columns to `obs.fundamental_fact`:

| Column | Type | Meaning |
| --- | --- | --- |
| `fy` | `integer`, null | the SEC fiscal year of the filing that reported the fact; null when SEC gave none |
| `fp` | `text NOT NULL DEFAULT ''` | the fiscal period of that filing: `Q1`, `Q2`, `Q3` or `FY`; empty when absent |
| `frame` | `text NOT NULL DEFAULT ''` | the calendar frame text, such as `CY2025Q2`; empty when SEC gave none |

`fy` and `fp` describe the filing, not the period of the fact. The migration touches no row.

## The view

`rd.fundamental_fact_r2` has the fourteen columns of `rd.fundamental_fact_r1` in the same order, then `fy`, `fp`, `frame`. Owner `ot_owner`; `SELECT` to `ot_ro_pages` and `ot_w_fundamental`. `rd.fundamental_fact_r1` is not changed and `FactsFiledBy` still reads it. `FactsWithPeriodFiledBy(ctx, source, subject, cutoff)` (the method of `companydataapi.PeriodFactReader`) reads r2 with the same filter, order and error classes as `FactsFiledBy` and fills `Fy`, `Fp` and `Frame`. The value `Open` returns has the method.

## Upsert

`UpsertFacts` writes the three columns in the insert and in `DO UPDATE SET`, and its `IS DISTINCT FROM` tuple lists them, so a stored row is replaced (counted as written) when `fy`, `fp` or `frame` differs. Rows stored before the migration have `fy` null and empty `fp` and `frame`: the next daily run of `fundamentals` fills the rows already stored. `InsertFacts` writes the columns too.

`ot_w_fundamental` has `SELECT` and `UPDATE` on the three columns (the upsert `WHERE` reads them); `INSERT` is table-level. Deploy: `setup pg`, `migrate`, `setup pg`, then the code that reads the columns.
