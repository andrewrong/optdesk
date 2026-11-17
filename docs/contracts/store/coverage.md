# store/coverage

Package `internal/store/coverage` declares the read-only view of the runner's ledger that the data health page reads (`docs/design/data-health.md`, Interface, entry 1). It holds declarations only; `internal/store/ops` implements it, as the page role `ot_ro_pages`, through the view `rd.coverage_day_r1`. It also holds `WindowSessions` (30, the session days shown and repaired) and `LookbackDays` (60 calendar days, enough for 30 sessions).

## States

`ok`: the key is present and the source gave a value.

`no_value`: the key is marked `no_value` and the newest run of the key finished clean: the source answered "nothing for this key".

`expired`: the key is marked `no_value` and the newest run of the key is not a clean finish: the runner closed the key at its window after a failed, partial or crashed run. A real gap, not an answer.

`missing`: the plan of the day expected the key and it is not present.

## Cell fields

`Day` is the session day of the cell.

`Dataset` is the ledger dataset id (not a job id).

`Keys` is the number of expected keys of the (day, dataset); `Keys == OK + NoValue + Expired + Missing`.

`OK`, `NoValue`, `Expired` and `Missing` count the keys in each state.

`MissingKeys` holds at most 20 missing subject keys, ascending.

## Meaning of ok

ok means the runner recorded the key as stored. It is the runner's record that the data was written in that turn, not a count of data rows; a table restored or pruned outside the runner leaves the ledger green.
